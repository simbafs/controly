package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// Message structures based on spec.md
type IncomingMessage struct {
	Type    string          `json:"type"`
	From    string          `json:"from,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

type OutgoingMessage struct {
	Type    string `json:"type"`
	To      string `json:"to,omitempty"`
	Payload any    `json:"payload"`
}

type Command struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type TestResult struct {
	SuccessfulControllers    uint64
	SuccessfulDisplays       uint64
	ControllerCommandsSent   uint64
	ControllerStatusReceived uint64
	DisplayStatusSent        uint64
	DisplayCommandsReceived  uint64
	ConnectionErrors         uint64
	SubscribeWriteErrors     uint64
	ControllerWriteErrors    uint64
	ControllerReadErrors     uint64
	DisplayWriteErrors       uint64
	DisplayReadErrors        uint64
}

var (
	// Atomic counters
	successfulControllers    uint64
	successfulDisplays       uint64
	controllerCommandsSent   uint64
	controllerStatusReceived uint64
	displayStatusSent        uint64
	displayCommandsReceived  uint64
	connectionErrors         uint64
	subscribeWriteErrors     uint64
	controllerWriteErrors    uint64
	controllerReadErrors     uint64
	displayWriteErrors       uint64
	displayReadErrors        uint64

	// Singleton for command server
	startServerOnce sync.Once
)

func resetCounters() {
	atomic.StoreUint64(&successfulControllers, 0)
	atomic.StoreUint64(&successfulDisplays, 0)
	atomic.StoreUint64(&controllerCommandsSent, 0)
	atomic.StoreUint64(&controllerStatusReceived, 0)
	atomic.StoreUint64(&displayStatusSent, 0)
	atomic.StoreUint64(&displayCommandsReceived, 0)
	atomic.StoreUint64(&connectionErrors, 0)
	atomic.StoreUint64(&subscribeWriteErrors, 0)
	atomic.StoreUint64(&controllerWriteErrors, 0)
	atomic.StoreUint64(&controllerReadErrors, 0)
	atomic.StoreUint64(&displayWriteErrors, 0)
	atomic.StoreUint64(&displayReadErrors, 0)
}

func ExecuteTest(ctx context.Context, n int, serverAddr, commandFile string, httpPort int, tts, ttc, duration time.Duration) TestResult {
	resetCounters()
	log.SetFlags(0) // Disable logging for cleaner output during tests

	// The main context for the entire test run, primarily for SIGINT.
	mainCtx, mainCancel := context.WithCancel(ctx)
	defer mainCancel()

	go runProgressIndicator(mainCtx)
	commandURL := startCommandServer(httpPort, commandFile)

	var connectionWg, workWg sync.WaitGroup
	connectionWg.Add(n * 2)
	workWg.Add(n * 2)

	for i := range n {
		displayID := fmt.Sprintf("load-test-display-%d", i)
		controllerID := fmt.Sprintf("load-test-controller-%d", i)
		go runDisplay(mainCtx, &connectionWg, &workWg, serverAddr, displayID, commandURL, tts)
		time.Sleep(10 * time.Millisecond)
		go runController(mainCtx, &connectionWg, &workWg, serverAddr, controllerID, displayID, ttc)
	}

	// Wait for all connection attempts to complete.
	connectionWg.Wait()

	// If the main context is already done (e.g. Ctrl+C during connection), stop now.
	if mainCtx.Err() != nil {
		workWg.Wait() // Wait for goroutines to exit.
		return TestResult{}
	}

	// Start a timer to end the test after the duration.
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Duration is over.
	case <-mainCtx.Done():
		// Test was cancelled (e.g. by SIGINT).
	}

	// Cancel the context to signal workers to stop.
	mainCancel()

	// Wait for all workers to finish gracefully.
	workWg.Wait()

	return TestResult{
		SuccessfulControllers:    atomic.LoadUint64(&successfulControllers),
		SuccessfulDisplays:       atomic.LoadUint64(&successfulDisplays),
		ControllerCommandsSent:   atomic.LoadUint64(&controllerCommandsSent),
		DisplayCommandsReceived:  atomic.LoadUint64(&displayCommandsReceived),
		DisplayStatusSent:        atomic.LoadUint64(&displayStatusSent),
		ControllerStatusReceived: atomic.LoadUint64(&controllerStatusReceived),
		ConnectionErrors:         atomic.LoadUint64(&connectionErrors),
		SubscribeWriteErrors:     atomic.LoadUint64(&subscribeWriteErrors),
		ControllerWriteErrors:    atomic.LoadUint64(&controllerWriteErrors),
		ControllerReadErrors:     atomic.LoadUint64(&controllerReadErrors),
		DisplayWriteErrors:       atomic.LoadUint64(&displayWriteErrors),
		DisplayReadErrors:        atomic.LoadUint64(&displayReadErrors),
	}
}

func runProgressIndicator(ctx context.Context) {
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	spinnerChars := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprint(os.Stderr, "\r")
			return
		case <-ticker.C:
			fmt.Fprintf(os.Stderr, "\r%s Running load test...", spinnerStyle.Render(spinnerChars[i]))
			i = (i + 1) % len(spinnerChars)
		}
	}
}

func startCommandServer(port int, filePath string) string {
	addr := fmt.Sprintf(":%d", port)
	startServerOnce.Do(func() {
		http.HandleFunc("/command.json", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filePath)
		})
		go func() {
			if err := http.ListenAndServe(addr, nil); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start command server: %v", err)
			}
		}()
	})
	return fmt.Sprintf("http://localhost%s/command.json", addr)
}

func runDisplay(ctx context.Context, connectionWg, workWg *sync.WaitGroup, serverAddr, displayID, commandURL string, tts time.Duration) {
	defer workWg.Done()

	q := url.Values{"type": {"display"}, "id": {displayID}, "command_url": {commandURL}}
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws", RawQuery: q.Encode()}

	c, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	connectionWg.Done() // Signal that connection attempt is finished

	if err != nil {
		atomic.AddUint64(&connectionErrors, 1)
		return
	}
	atomic.AddUint64(&successfulDisplays, 1)
	// defer c.Close() // Removed for abrupt shutdown

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					atomic.AddUint64(&displayReadErrors, 1)
				}
				return
			}
			var msg IncomingMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				atomic.AddUint64(&displayReadErrors, 1)
				continue
			}
			if msg.Type == "command" {
				atomic.AddUint64(&displayCommandsReceived, 1)
			}
		}
	}()

	ticker := time.NewTicker(tts)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Abruptly stop without sending a close message.
			return
		case t := <-ticker.C:
			statusPayload := map[string]any{"timestamp": t.Unix(), "status": "OK"}
			statusMsg := OutgoingMessage{Type: "status", Payload: statusPayload}
			if err := c.WriteJSON(statusMsg); err != nil {
				atomic.AddUint64(&displayWriteErrors, 1)
				return
			}
			atomic.AddUint64(&displayStatusSent, 1)
		}
	}
}

func runController(ctx context.Context, connectionWg, workWg *sync.WaitGroup, serverAddr, controllerID, targetDisplayID string, ttc time.Duration) {
	defer workWg.Done()

	q := url.Values{"type": {"controller"}, "id": {controllerID}}
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws", RawQuery: q.Encode()}

	c, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	connectionWg.Done() // Signal that connection attempt is finished

	if err != nil {
		atomic.AddUint64(&connectionErrors, 1)
		return
	}
	atomic.AddUint64(&successfulControllers, 1)
	// defer c.Close() // Removed for abrupt shutdown

	subscribeMsg := OutgoingMessage{
		Type:    "subscribe",
		Payload: map[string][]string{"display_ids": {targetDisplayID}},
	}
	if err := c.WriteJSON(subscribeMsg); err != nil {
		atomic.AddUint64(&subscribeWriteErrors, 1)
		return
	}

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					atomic.AddUint64(&controllerReadErrors, 1)
				}
				return
			}
			var msg IncomingMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				atomic.AddUint64(&controllerReadErrors, 1)
				continue
			}
			if msg.Type == "status" && msg.From == targetDisplayID {
				atomic.AddUint64(&controllerStatusReceived, 1)
			}
		}
	}()

	ticker := time.NewTicker(ttc)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Abruptly stop without sending a close message.
			return
		case <-ticker.C:
			commandPayload := Command{
				Name: "set_text",
				Args: map[string]any{"value": fmt.Sprintf("load test %s", time.Now().Format(time.RFC3339Nano))},
			}
			commandMsg := OutgoingMessage{Type: "command", To: targetDisplayID, Payload: commandPayload}
			if err := c.WriteJSON(commandMsg); err != nil {
				atomic.AddUint64(&controllerWriteErrors, 1)
				return
			}
			atomic.AddUint64(&controllerCommandsSent, 1)
		}
	}
}
