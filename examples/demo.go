
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