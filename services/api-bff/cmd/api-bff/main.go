// Package main runs the api-bff service.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/aminio9/gereh/platform/go/service"
	"github.com/go-chi/chi/v5"
)

var version = "dev"

func main() {
	service.Run(service.Config{
		Name:           "api-bff",
		Version:        version,
		DefaultAddress: ":8080",
	}, registerRoutes)
}

func registerRoutes(router chi.Router) {
	router.Get("/v1/status", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"status":  "ok",
			"service": "api-bff",
			"version": version,
		})
	})
}
