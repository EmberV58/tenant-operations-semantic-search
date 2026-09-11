package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	search "example.com/tenant-ops-search/search"
)

type server struct{ search *search.TenantSearch }

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	model := os.Getenv("EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	client := search.NewClient(apiKey, model)
	handler := server{search: search.NewTenantSearch(client, client, "saas-operations")}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /search", handler.handleSearch)
	httpServer := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("tenant search listening on %s", httpServer.Addr)
	log.Fatal(httpServer.ListenAndServe())
}

func (s server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	results, err := s.search.Search(r.Context(), input.Query, input.Limit)
	if err != nil {
		status := http.StatusBadGateway
		var apiErr *search.InfraiError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			status = apiErr.HTTPStatus
		}
		if strings.TrimSpace(input.Query) == "" || input.Limit < 1 || input.Limit > 20 {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response for status %s: %v", strconv.Itoa(status), err)
	}
}
