package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/simbafs/controly/server/internal"
	"github.com/simbafs/controly/server/internal/config"
)

//go:embed all:controller/*
var files embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		log.Println("Warning: could not get file descriptor limit:", err)
	} else {
		rLimit.Cur = rLimit.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			log.Println("Warning: could not set file descriptor limit:", err)
		} else {
			log.Printf("File descriptor limit set to: %d", rLimit.Cur)
		}
	}

	log.Printf("PID: %d", os.Getpid())

	cfg := config.NewConfig()
	hub := internal.NewHub(cfg.Token)
	go hub.Run()

	contentFs, err := fs.Sub(files, "controller/dist")
	if err != nil {
		panic(err)
	}

	router := mux.NewRouter()

	// WebSocket handlers
	router.HandleFunc("/ws", hub.ServeWs)
	router.HandleFunc("/ws/inspector", hub.InspectorWsHandler)

	// REST API handlers
	router.HandleFunc("/api/connections", hub.ConnectionsHandler).Methods("GET")
	router.HandleFunc("/api/displays/{id}", hub.DeleteDisplayHandler).Methods("DELETE")
	router.HandleFunc("/api/controllers/{id}", hub.DeleteControllerHandler).Methods("DELETE")

	// Serve embedded frontend files
	router.PathPrefix("/").Handler(hub.FrontendHandler(contentFs))

	log.Printf("Relay Server started on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, router))
}
