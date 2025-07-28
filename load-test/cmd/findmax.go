package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// testCapacity runs a load test for a given number of clients and returns true if it's successful.
func testCapacity(ctx context.Context, numClients int, duration time.Duration, server, commandFile string, httpPort int, tts, ttc time.Duration, successRateThreshold float64) bool {
	if numClients <= 0 {
		return true // A test with 0 or fewer clients is considered a success to not break search logic.
	}
	if ctx.Err() != nil {
		return false
	}

	fmt.Printf("--- Testing with %d client pairs ---\n", numClients)
	// The test execution itself is limited by the duration parameter.
	runCtx, runCancel := context.WithTimeout(ctx, duration)
	defer runCancel()

	result := ExecuteTest(runCtx, numClients, server, commandFile, httpPort, tts, ttc)

	if ctx.Err() != nil { // Check main context cancellation
		return false
	}

	totalRequested := uint64(numClients * 2)
	totalConnected := result.SuccessfulControllers + result.SuccessfulDisplays
	var successRate float64
	if totalRequested > 0 {
		successRate = (float64(totalConnected) / float64(totalRequested)) * 100
	}

	statusStyle := lipgloss.NewStyle().Bold(true)
	success := successRate >= successRateThreshold
	if success {
		fmt.Printf("Result: %s (%.2f%% connections succeeded)\n\n", statusStyle.Foreground(lipgloss.Color("#32CD32")).Render("SUCCESS"), successRate)
	} else {
		fmt.Printf("Result: %s (%.2f%% connections succeeded)\n\n", statusStyle.Foreground(lipgloss.Color("#FF5E5E")).Render("FAILURE"), successRate)
	}
	time.Sleep(1 * time.Second) // Pause between tests
	return success
}

var findMaxCmd = &cobra.Command{
	Use:   "find-max",
	Short: "Find the maximum number of clients using a dynamic search strategy.",
	Long: `Finds the maximum number of controller/display pairs that can run stably.
It first tests the lower bound. If that passes, it tests the upper bound and doubles it until a failure, establishing a search range.
Finally, it uses binary search within that range to pinpoint the maximum stable load.`,
	Run: func(cmd *cobra.Command, args []string) {
		low, _ := cmd.Flags().GetInt("low")
		high, _ := cmd.Flags().GetInt("high")
		successRateThreshold, _ := cmd.Flags().GetFloat64("success-rate")

		server, _ := cmd.Flags().GetString("server")
		duration, _ := cmd.Flags().GetDuration("duration")
		tts, _ := cmd.Flags().GetDuration("tts")
		ttc, _ := cmd.Flags().GetDuration("ttc")
		commandFile, _ := cmd.Flags().GetString("command-file")
		httpPort, _ := cmd.Flags().GetInt("http-port")

		fmt.Printf("Starting dynamic search for max clients (Initial Range: %d-%d, Success Rate: >=%.2f%%)\n\n", low, high, successRateThreshold)

		// Setup context and signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nSignal received, stopping search...")
			cancel()
		}()

		finalStyle := lipgloss.NewStyle().Bold(true).Padding(1).Border(lipgloss.DoubleBorder(), true).BorderForeground(lipgloss.Color("69"))

		// --- Step 1: Test lower bound ---
		fmt.Println("--- Step 1: Testing lower bound ---")
		if !testCapacity(ctx, low, duration, server, commandFile, httpPort, tts, ttc, successRateThreshold) {
			if ctx.Err() == nil {
				fmt.Println(finalStyle.Render(fmt.Sprintf("Lower bound of %d clients failed. Aborting.", low)))
			} else {
				fmt.Println(finalStyle.Render("Search stopped by user."))
			}
			return
		}
		if ctx.Err() != nil {
			fmt.Println(finalStyle.Render("Search stopped by user."))
			return
		}
		lastSuccess := low

		// --- Step 2: Find upper bound ---
		fmt.Println("\n--- Step 2: Finding upper failure bound ---")
		currentHigh := high
		for ctx.Err() == nil {
			if !testCapacity(ctx, currentHigh, duration, server, commandFile, httpPort, tts, ttc, successRateThreshold) {
				// We found the failure point. The search range is [lastSuccess, currentHigh].
				high = currentHigh
				low = lastSuccess
				break
			}

			// currentHigh passed. It's our new lastSuccess.
			lastSuccess = currentHigh

			// Let's check for overflow before doubling.
			if currentHigh > 1_000_000_000 {
				fmt.Println("Reached capacity limit, will not expand further.")
				low = currentHigh
				high = currentHigh // This effectively makes the binary search range empty.
				break
			}

			fmt.Printf("Boundary test at %d passed. Increasing to %d\n\n", currentHigh, currentHigh*2)
			currentHigh *= 2
		}

		if ctx.Err() != nil {
			fmt.Println(finalStyle.Render("Search stopped by user."))
			return
		}

		// --- Step 3: Binary Search ---
		fmt.Printf("\n--- Step 3: Binary searching in range [%d, %d] ---\n\n", low, high)

		binaryLow := low + 1
		binaryHigh := high - 1

		for binaryLow <= binaryHigh {
			if ctx.Err() != nil {
				break
			}

			mid := binaryLow + (binaryHigh-binaryLow)/2
			if mid <= lastSuccess { // Optimization: don't re-test known-good values
				binaryLow = mid + 1
				continue
			}

			if testCapacity(ctx, mid, duration, server, commandFile, httpPort, tts, ttc, successRateThreshold) {
				lastSuccess = mid
				binaryLow = mid + 1
			} else {
				binaryHigh = mid - 1
			}
		}

		if ctx.Err() != nil {
			fmt.Println(finalStyle.Render("Search stopped by user."))
		} else if lastSuccess > 0 {
			fmt.Println(finalStyle.Render(fmt.Sprintf("Maximum stable client pairs found: %d", lastSuccess)))
		} else {
			// This case should ideally not be reached if low > 0 and passes the initial test.
			fmt.Println(finalStyle.Render("Could not find a stable number of clients in the given range."))
		}
	},
}

func init() {
	findMaxCmd.Flags().Int("low", 1, "Lower bound for binary search")
	findMaxCmd.Flags().Int("high", 1000, "Upper bound for binary search")
	findMaxCmd.Flags().Float64("success-rate", 99.0, "Minimum connection success rate to be considered stable")
	rootCmd.AddCommand(findMaxCmd)
}
