package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "job-scheduler",
	Short: "A CLI for managing the Golang Job Scheduler services",
	Long:  `Golang Job Scheduler is a scalable and high-performance job scheduler...`,
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
