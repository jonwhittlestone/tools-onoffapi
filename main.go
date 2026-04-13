package main

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

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	machineHandler := handlers.NewMachineHandler(store)
	machineHandler.RegisterRoutes(mux)

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}
	protected := handlers.RequireAPIKey(apiKey, mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("onoffapi listening on :8080")
	log.Fatal(http.ListenAndServe(":"+port, protected))
}
