package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/nhalstead/exitplan"
	"github.com/spf13/cobra"
)

var (
	healthChecks     bool
	httpChecksServer int64

	testCmd = &cobra.Command{
		Use:   "test",
		Short: "Test the usage of the signals.",
		Long:  "Runs the code as normal in conjunction with cobra Command with the usage of SIGINT and SIGTERM.",
		Run: func(cmd *cobra.Command, args []string) {

			m := mux.NewRouter()

			srv := &http.Server{
				Handler:      m,
				Addr:         "127.0.0.1:" + strconv.FormatInt(httpChecksServer, 10),
				WriteTimeout: 5 * time.Second,
				ReadTimeout:  5 * time.Second,
			}

			plan := exitplan.NewPlan()
			plan.GradePeriod = 5 * time.Second
			plan.Add("http", srv.Shutdown)
			//plan.AddMany(map[string]exitplan.ExitOperation{
			//	"http": func(ctx context.Context) error {
			//		// Delayed Shutdown
			//		time.Sleep(5 * time.Second)
			//		return srv.Shutdown(ctx)
			//	},
			//})

			// Register a Request Handler on "/readyz" for the status.
			// See https://kubernetes.io/docs/reference/using-api/health-checks/ for more information
			m.HandleFunc("/readyz", plan.HandlerFunc).Methods(http.MethodGet)

			go srv.ListenAndServe()

			fmt.Println("Server Running")
			fmt.Println("Goto http://localhost:" + strconv.FormatInt(httpChecksServer, 10) + "/readyz")

			plan.Finally(func(ctx context.Context) error {
				// Do some final cool stuff before death
				fmt.Println("final callback made")
				return nil
			})
			plan.Wait(context.TODO())

		},
	}
)

func init() {
	testCmd.Flags().BoolVarP(&healthChecks, "heath-checks", "e", true, "enable health checks server.")
	testCmd.Flags().Int64VarP(&httpChecksServer, "port", "p", 8855, "http port for the test server to run on.")
}
