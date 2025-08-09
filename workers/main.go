package main

import (
	"encoding/json"
	"net/http"
)

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":    "ok",
		"component": "worker",
	})
}

func main() {
	http.HandleFunc("/health", health)
	// écoute sur 8090, comme dans le docker-compose
	_ = http.ListenAndServe(":8090", nil)
}
