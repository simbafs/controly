package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a single load test with a specified number of clients.",
	Long:  `Executes a load test with a fixed number of controller/display pairs and reports the results.`,
	Run: func(cmd *cobra.Command, args []string) {
		n, _ := cmd.Flags().GetInt("n")
		errorThreshold, _ := cmd.Flags().GetFloat64("error-threshold")
		server, _ := cmd.Flags().GetString("server")
		duration, _ := cmd.Flags().GetDuration("duration")
		tts, _ := cmd.Flags().GetDuration("tts")
		ttc, _ := cmd.Flags().GetDuration("ttc")
		commandFile, _ := cmd.Flags().GetString("command-file")
		httpPort, _ := cmd.Flags().GetInt("http-port")

		fmt.Printf("Starting single run load test with %d pairs...\n", n)

		// Setup context and signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nSignal received, shutting down...")
			cancel()
		}()

		testCtx, testCancel := context.WithTimeout(ctx, duration)
		defer testCancel()

		result := ExecuteTest(testCtx, n, server, commandFile, httpPort, tts, ttc)

		// Don't print report if the context was cancelled by a signal
		if ctx.Err() == nil {
			GenerateAndPrintReport(n, errorThreshold, result)
		}
	},
}

func init() {
	runCmd.Flags().IntP("n", "n", 1, "Number of controller/display pairs")
	runCmd.Flags().Float64("error-threshold", 1.0, "Error rate threshold for the test to be considered successful (e.g., 1.0 for 1%)")
	rootCmd.AddCommand(runCmd)
}
