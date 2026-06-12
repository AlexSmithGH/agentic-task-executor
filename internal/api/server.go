package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/alexasmi/agentic-task-executor/internal/config"
)

func NewServer(h *Handler, cfg *config.Config) *http.Server {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	r.Get("/", h.Root)
	r.Get("/health", h.HealthCheck)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/execute-task", h.ExecuteTask)
		r.Get("/task/{workflowID}/status", h.GetTaskStatus)
		r.Post("/task/{workflowID}/signal", h.SignalWorkflow)
		r.Post("/task/{workflowID}/cancel", h.CancelWorkflow)
		r.Get("/tasks", h.ListTasks)
	})

	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.APIHost, cfg.APIPort),
		Handler: r,
	}
}
