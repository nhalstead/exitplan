package cmd

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "exitplan",
		Short: "Example usage of exit plan with cobra",
		Long:  "Testing of exit plan being used with cobra responding to SIGINT, SIGTERM, SIGHUP.",
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(testCmd)
}
