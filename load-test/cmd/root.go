package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "load-test",
	Short: "A load testing tool for the Controly WebSocket server.",
	Long:  `A versatile load testing tool to measure the performance and stability of the Controly WebSocket server.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("server", "localhost:8080", "WebSocket server address")
	rootCmd.PersistentFlags().Duration("duration", 10*time.Second, "Total duration of the test")
	rootCmd.PersistentFlags().Duration("tts", 100*time.Millisecond, "Time to send status interval for displays")
	rootCmd.PersistentFlags().Duration("ttc", 1*time.Second, "Time to send command interval for controllers")
	rootCmd.PersistentFlags().String("command-file", "command.json", "Path to the command.json file")
	rootCmd.PersistentFlags().Int("http-port", 8081, "Port for the local HTTP server to serve command.json")
}
