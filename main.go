package main

// main.go is the entry point. It:
// 1. Creates the shared in-memory store
// 2. Wires up routes
// 3. Starts the HTTP server
//
// Equivalent to: app = FastAPI() + uvicorn.run(app, port=8080)

import (
	"log"
	"net/http"
	"os"

	"github.com/jonwhittlestone/tools-onoffapi/handlers"
	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func main() {
	store := models.NewStore()
	mux := http.NewServeMux()

	// Health endpoint — no auth required
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Machine CRUD routes — protected by API key middleware
	machineHandler := handlers.NewMachineHandler(store)
	machineHandler.RegisterRoutes(mux)

	// Wrap the entire mux with the API key middleware
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	protected := handlers.RequireAPIKey(apiKey, mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("onoffapi listening on :%s", port)
	if err := http.ListenAndServe(":"+port, protected); err != nil {
		log.Fatal(err)
	}
}
