package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/jonwhittlestone/tools-onoffapi/handlers"
	"github.com/jonwhittlestone/tools-onoffapi/models"
)

//go:embed static
var staticFiles embed.FS

func main() {

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	store := models.NewStore()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// GET /auth?key=<value> — validates a key server-side; used by the deep-link
	// login flow so the API key never has to be stored in the URL permanently.
	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("key") != apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"valid":false}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"valid":true}`)) //nolint:errcheck
	})

	machineHandler := handlers.NewMachineHandler(store)
	machineHandler.RegisterRoutes(mux)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	protected := handlers.RequireAPIKey(apiKey, mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("onoffapi listening on :8080")
	log.Fatal(http.ListenAndServe(":"+port, protected))
}
