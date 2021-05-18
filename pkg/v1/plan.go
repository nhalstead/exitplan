package v1

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ExitOperation is a clean up function on shutting down
type ExitOperation func(ctx context.Context) error

type ExecutionPlan struct {
	Signals       []os.Signal
	Timeout       time.Duration
	GradePeriod   time.Duration
	callbacks     map[string]ExitOperation
	finalCallback ExitOperation
	isTerminating bool
}

// NewPlan will create a new ExecutionPlan with a default
//  GradePeriod of 5 seconds and Timeout of 25 seconds
func NewPlan() *ExecutionPlan {
	return NewPlanWithTimer(5*time.Second, 25*time.Second)
}

func NewPlanWithTimer(gradePeriod, timeout time.Duration) *ExecutionPlan {
	plan := ExecutionPlan{
		Signals: []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGHUP,
		},
		Timeout:       timeout,
		GradePeriod:   gradePeriod,
		callbacks:     make(map[string]ExitOperation, 5),
		isTerminating: false,
	}

	return &plan
}

func (p *ExecutionPlan) IsTerminating() bool {
	return p.isTerminating
}

func (p *ExecutionPlan) AddMany(many map[string]ExitOperation) *ExecutionPlan {

	for k, v := range many {
		p.callbacks[k] = v
	}

	return p
}

func (p *ExecutionPlan) Add(name string, handler ExitOperation) *ExecutionPlan {

	p.callbacks[name] = handler

	return p
}

func (p *ExecutionPlan) Finally(handler ExitOperation) *ExecutionPlan {

	p.finalCallback = handler

	return p
}

func (p *ExecutionPlan) Wait() {
	p.WaitContext(context.Background())
}

// HandlerFunc is used on the HTTP Server Side to support a RESTful way of ready state.
// See https://kubernetes.io/docs/reference/using-api/health-checks/ for more information
func (p *ExecutionPlan) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	if p.IsTerminating() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("terminating"))
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// WaitContext will wait until the program gets an exit signal and all handlers have succeeded.
// If used on the main thread, this will allow it to die
func (p *ExecutionPlan) WaitContext(ctx context.Context) {
	<-p.WaitWithChan(ctx)
}

func (p *ExecutionPlan) WaitWithChan(ctx context.Context) <-chan struct{} {

	// Chan to be used to allow execution to continue
	sigChannel := make(chan struct{})

	// Create a new goroutines to kick off the exit method calls.
	go func() {
		s := make(chan os.Signal, 1)

		// Set syscalls to listen for using the chan
		signal.Notify(s, p.Signals...)

		// Wait for an interrupt to be triggered.
		<-s

		// Indicate internally the app is going to shutdown and to not accept
		//  and new connections.
		log.Println("interrupt received...")
		p.isTerminating = true

		// Allow for connections to drain.
		time.Sleep(p.GradePeriod)

		// Set timeout for the operations to complete and prevent system hang or dropped connections
		log.Println("shutting down")
		timeoutFunc := time.AfterFunc(p.Timeout, func() {
			log.Printf("timeout %d ms has been elapsed, force exit", p.Timeout.Milliseconds())
			os.Exit(0)
		})

		defer timeoutFunc.Stop()

		var wg sync.WaitGroup

		// Execute exit operations async to allow for a faster shutdown process.
		for key, op := range p.callbacks {
			wg.Add(1)
			go func(innerKey string, innerOp ExitOperation) {
				defer wg.Done()

				log.Printf("disposing: %s", innerKey)
				if err := innerOp(ctx); err != nil {
					log.Printf("%s: dispose failed: %s", innerKey, err.Error())
					return
				}
				log.Printf("%s was disposed gracefully", innerKey)
			}(key, op)
		}

		// Wait for all of the Exit Operations to complete their exit operation.
		wg.Wait()

		// Final cleanup callback
		if p.finalCallback != nil {
			if err := p.finalCallback(ctx); err != nil {
				log.Printf("final: dispose failed: %s", err.Error())
				return
			}
			log.Println("final was disposed gracefully")
		}

		close(sigChannel)
	}()

	return sigChannel
}
