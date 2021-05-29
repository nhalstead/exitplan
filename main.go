package main

import (
	"os"

	"github.com/nhalstead/exitplan/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
