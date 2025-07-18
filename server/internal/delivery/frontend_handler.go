package delivery

import (
	"io/fs"
	"net/http"
)

// FrontendHandler serves the embedded frontend files.
type FrontendHandler struct {
	fs http.Handler
}

// NewFrontendHandler creates a new FrontendHandler.
func NewFrontendHandler(files fs.FS) *FrontendHandler {
	return &FrontendHandler{
		fs: http.FileServer(http.FS(files)),
	}
}

// ServeHTTP serves the static files.
func (h *FrontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.fs.ServeHTTP(w, r)
}
