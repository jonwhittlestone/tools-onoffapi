package main

import (
	"log"
	"net/http"

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

	log.Println("onoffapi listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
