package exitplan

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

// ExitOperation is a cleanup function on shutting down
type ExitOperation func(ctx context.Context) error

type ExecutionPlan struct {
	Signals            []os.Signal
	Timeout            time.Duration
	GradePeriod        time.Duration

	callbacks          map[string]ExitOperation
	callbacksMutex     sync.RWMutex
	finalCallback      ExitOperation

	isTerminating      bool
	isTerminatingMutex sync.RWMutex

	termListeners      []chan struct{}
	termLock           sync.Mutex
	interruptListen    sync.Mutex
}

// NewPlan will create a new ExecutionPlan with a default
//  GradePeriod of 5 seconds and Timeout of 25 seconds
func NewPlan() *ExecutionPlan {
	return NewPlanWithTimer(5 * time.Second, 20 * time.Second)
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
		termListeners: make([]chan struct{}, 0),
		isTerminating: false,
	}

	return &plan
}

func (p *ExecutionPlan) IsTerminating() bool {
	return p.isTerminating
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

// NewExitChan will return a new chan listener to allow for
//  use within a select statement.
func (p *ExecutionPlan) NewExitChan() chan struct{} {
	c := make(chan struct{})

	p.termLock.Lock()
	p.termListeners = append(p.termListeners, c)
	p.termLock.Unlock()

	return c
}

func (p *ExecutionPlan) Add(name string, handler ExitOperation) {
	p.callbacksMutex.Lock()
	defer p.callbacksMutex.Unlock()
	p.callbacks[name] = handler
}

func (p *ExecutionPlan) Finally(handler ExitOperation) {
	p.finalCallback = handler
}

// Wait will wait until the program gets an exit signal and all handlers have succeeded.
// If used on the main thread, this will allow it to die
func (p *ExecutionPlan) Wait(ctx context.Context) {
	<-p.Start(ctx)
}

// Start will begin watching the os.Signal for the set interrupts.
// If a signal is set then everything kicks into action.
func (p *ExecutionPlan) Start(ctx context.Context) chan struct{} {

	// Used to prevent two calls to wait, having two listeners
	p.interruptListen.Lock()
	defer p.interruptListen.Unlock()

	// Chan to be used to allow execution to continue
	sigChannel := make(chan struct{})

	// Create a new goroutines to kick off the exit method calls once the os.Signal hits.
	go func() {
		s := make(chan os.Signal, 1)

		// Set syscalls to listen for using the chan
		signal.Notify(s, p.Signals...)

		// Wait for an interrupt to be triggered.
		<-s

		// Indicate internally the app is going to shutdown and to not accept
		//  any new connections.
		log.Println("interrupt received...")
		p.isTerminatingMutex.Lock()
		p.isTerminating = true
		p.isTerminatingMutex.Unlock()

		// Close the termListener chan(s) to send a signal that it's received a terminating signal
		go func(termListeners []chan struct{}) {
			for _, c := range termListeners {
				close(c)
			}
		}(p.termListeners)

		// Wait to allow for connections to drain.
		time.Sleep(p.GradePeriod)

		// Set timeout for the operations to complete and prevent system hang and prevent SIGKILL
		log.Println("shutting down")
		timeoutFunc := time.AfterFunc(p.Timeout, func() {
			log.Printf("timeout %d ms has elapsed, force exit", p.Timeout.Milliseconds())
			os.Exit(0)
		})

		var wg sync.WaitGroup

		// Execute exit operations async to allow for a faster shutdown process.
		p.callbacksMutex.RLock()
		for key, op := range p.callbacks {
			wg.Add(1)
			go func(innerKey string, dispose ExitOperation) {
				defer wg.Done()

				log.Printf("disposing: %s", innerKey)
				if err := dispose(ctx); err != nil {
					log.Printf("%s: dispose failed: %s", innerKey, err.Error())
					return
				}
				log.Printf("%s was disposed gracefully", innerKey)
			}(key, op)
		}
		p.callbacksMutex.RUnlock()

		// Wait for all Exit Operations to complete their exit operation.
		// If the timeoutFunc expires, kill the entire process.
		wg.Wait()

		// Stop the timeout function for os.Exit to allow the final callbacks to run.
		timeoutFunc.Stop()

		// Final cleanup callback
		// Successfully cleaned up connections and exit operations
		if p.finalCallback != nil {
			if err := p.finalCallback(ctx); err != nil {
				log.Printf("final: dispose failed: %s", err.Error())
				return
			}
			log.Println("final was disposed gracefully")
		}

		// Close the signal channel for the holding callback.
		close(sigChannel)
	}()

	return sigChannel
}
