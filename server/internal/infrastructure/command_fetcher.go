package infrastructure

import (
	"fmt"
	"io"
	"net/http"
)

// HTTPCommandFetcher implements application.CommandFetcher using net/http.
type HTTPCommandFetcher struct{}

// NewHTTPCommandFetcher creates a new HTTPCommandFetcher.
func NewHTTPCommandFetcher() *HTTPCommandFetcher {
	return &HTTPCommandFetcher{}
}

// FetchCommands fetches command.json content from the given URL.
func (f *HTTPCommandFetcher) FetchCommands(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch command URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("command URL returned status code: %d", resp.StatusCode)
	}

	commandData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read command JSON: %w", err)
	}

	return commandData, nil
}
