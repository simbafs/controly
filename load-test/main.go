package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
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

type SetIDPayload struct {
	ID string `json:"id"`
}

type Command struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// Atomic counters for statistics
var (
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
)

func runProgressIndicator(ctx context.Context) {
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	spinnerChars := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprint(os.Stderr, "\r") // Clear the spinner
			return
		case <-ticker.C:
			fmt.Fprintf(os.Stderr, "\r%s Running load test...", spinnerStyle.Render(spinnerChars[i]))
			i = (i + 1) % len(spinnerChars)
		}
	}
}

func main() {
	// Command-line flags
	n := flag.Int("n", 1, "Number of controller/display pairs")
	tts := flag.Duration("tts", 100*time.Millisecond, "Time to send status interval")
	ttc := flag.Duration("ttc", 1*time.Second, "Time to send command interval")
	duration := flag.Duration("duration", 10*time.Second, "Total duration of the test")
	serverAddr := flag.String("server", "localhost:8080", "WebSocket server address")
	commandFile := flag.String("command-file", "command.json", "Path to the command.json file")
	httpPort := flag.Int("http-port", 8081, "Port for the local HTTP server to serve command.json")
	errorThreshold := flag.Float64("error-threshold", 1.0, "Error rate threshold for the test to be considered successful (e.g., 1.0 for 1%)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// Progress indicator
	go runProgressIndicator(ctx)

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Start local HTTP server for command.json
	commandURL := startCommandServer(*httpPort, *commandFile)

	var wg sync.WaitGroup

	for i := range *n {
		wg.Add(2)
		displayID := fmt.Sprintf("load-test-display-%d", i)
		controllerID := fmt.Sprintf("load-test-controller-%d", i)

		go runDisplay(ctx, &wg, *serverAddr, displayID, commandURL, *tts)
		time.Sleep(10 * time.Millisecond)
		go runController(ctx, &wg, *serverAddr, controllerID, displayID, *ttc)
	}

	go func() {
		wg.Wait()
		cancel()
	}()

	<-ctx.Done()

	// --- Final Report ---
	generateAndPrintReport(
		*n,
		*errorThreshold,
		atomic.LoadUint64(&successfulControllers),
		atomic.LoadUint64(&successfulDisplays),
		atomic.LoadUint64(&controllerCommandsSent),
		atomic.LoadUint64(&displayCommandsReceived),
		atomic.LoadUint64(&displayStatusSent),
		atomic.LoadUint64(&controllerStatusReceived),
		atomic.LoadUint64(&connectionErrors),
		atomic.LoadUint64(&subscribeWriteErrors),
		atomic.LoadUint64(&controllerWriteErrors),
		atomic.LoadUint64(&controllerReadErrors),
		atomic.LoadUint64(&displayWriteErrors),
		atomic.LoadUint64(&displayReadErrors),
	)
}

func startCommandServer(port int, filePath string) string {
	addr := fmt.Sprintf(":%d", port)
	http.HandleFunc("/command.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	})
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start command server: %v", err)
		}
	}()
	return fmt.Sprintf("http://localhost%s/command.json", addr)
}

func runDisplay(ctx context.Context, wg *sync.WaitGroup, serverAddr, displayID, commandURL string, tts time.Duration) {
	defer wg.Done()

	q := url.Values{"type": {"display"}, "id": {displayID}, "command_url": {commandURL}}
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws", RawQuery: q.Encode()}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		atomic.AddUint64(&connectionErrors, 1)
		return
	}
	atomic.AddUint64(&successfulDisplays, 1)
	defer c.Close()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
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
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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

func runController(ctx context.Context, wg *sync.WaitGroup, serverAddr, controllerID, targetDisplayID string, ttc time.Duration) {
	defer wg.Done()

	q := url.Values{"type": {"controller"}, "id": {controllerID}}
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws", RawQuery: q.Encode()}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		atomic.AddUint64(&connectionErrors, 1)
		return
	}
	atomic.AddUint64(&successfulControllers, 1)
	defer c.Close()

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
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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
