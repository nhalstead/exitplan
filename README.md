# Exit Plan

Simple go package to help with the shutdown process within a go program by integrating with SIGINT and SIGTERM.
The purpose is to be used with a container orchestration software like kubernetes where heath checks are done on the pod.

This can be used with or without the use of an HTTP server serving the heath check data.

## Demo and Details

This repo has the example in it already using the [cobra package](https://github.com/spf13/cobra).

Clone down the repo and build it.

Upon running executing `expitplan test`, an HTTP server on port 8855 will be running.

Browse to [http://localhost:8855/readyz](http://localhost:8855/readyz) to check the status!

---

Now that the server is running, and the page is responding with "ok", you can now press COMMAND+C / CTRL+C in the terminal window to trigger a SIGTERM.

After you trigger the SIGTERM, two actions take place. First a flag is set to indicate
 it's going to terminate (which changes the response for `/readyz`), then a grace period is waiting to complete.

After the grade period has passed it kicks into action again with a timeout for the deadline of the program to exit.
Before reaching the deadline, goroutines are being made to call the shutdown methods that have been registered.

Lastly before the program exists it makes one last synchronous call to the FinalCallback.

## Usage

### Import
```text
exitplan "github.com/nhalstead/exitplan/pkg/v1"
```

### Usage Example
```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nhalstead/exitplan/pkg/v1"
)

func main() {

	m := mux.NewRouter()

	srv := &http.Server{
		Handler:      m,
		Addr:         "127.0.0.1:8855",
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	plan := exitplan.NewPlan()
	plan.Add("http", srv.Shutdown)

	// Register a Request Handler on "/readyz" for the status.
	m.HandleFunc("/readyz", plan.HandlerFunc).Methods(http.MethodGet)

	go srv.ListenAndServe()

	fmt.Println("Server Running")
	fmt.Println("Goto http://localhost:8855/readyz")

	plan.Finally(func (ctx context.Context) error {
		// Do some final cool stuff before death
		fmt.Println("final callback made")
		return nil
	})
	plan.Wait()

}
```

## Inspired By

[Gracefully Shutdown your Go Application](https://alfiandnm.medium.com/gracefully-shutdown-your-go-application-9e7d5c73b5ac) by Alfian Dhimas

[Graceful shutdown with Go http servers and Kubernetes rolling updates](https://medium.com/over-engineering/graceful-shutdown-with-go-http-servers-and-kubernetes-rolling-updates-6697e7db17cf) by Wayne Ashley Berry

[Stackoverflow: Testing graceful shutdown on an HTTP server during a Kubernetes rollout](https://stackoverflow.com/a/58752566/5779200)

[terminus *Graceful shutdown and Kubernetes readiness / liveness checks for any Node.js HTTP applications*](https://github.com/godaddy/terminus) by GoDaddy Dev Team
