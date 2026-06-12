package main

import (
	"log"
	"log/slog"

	"github.com/google/go-github/v68/github"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/alexasmi/agentic-task-executor/internal/activities"
	"github.com/alexasmi/agentic-task-executor/internal/config"
	"github.com/alexasmi/agentic-task-executor/internal/workflows"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	slog.Info("Starting Temporal worker",
		"host", cfg.TemporalHost,
		"namespace", cfg.TemporalNamespace,
		"task_queue", cfg.TemporalTaskQueue,
	)

	c, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalHost,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	w := worker.New(c, cfg.TemporalTaskQueue, worker.Options{})

	w.RegisterWorkflow(workflows.AgenticTaskWorkflow)

	gitActs := &activities.GitActivities{GitHubToken: cfg.GitHubToken}
	w.RegisterActivity(gitActs)

	ghClient := github.NewClient(nil).WithAuthToken(cfg.GitHubToken)
	ghActs := &activities.GitHubActivities{Client: ghClient}
	w.RegisterActivity(ghActs)

	agentActs := &activities.AgentActivities{Config: cfg}
	w.RegisterActivity(agentActs)

	slog.Info("Worker configured",
		"workflows", []string{"AgenticTaskWorkflow"},
		"task_queue", cfg.TemporalTaskQueue,
	)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	slog.Info("Worker stopped")
}
