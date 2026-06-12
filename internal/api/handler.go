package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/alexasmi/agentic-task-executor/internal/config"
	"github.com/alexasmi/agentic-task-executor/internal/workflows"
)

type Handler struct {
	temporalClient client.Client
	cfg            *config.Config
}

func NewHandler(temporalClient client.Client, cfg *config.Config) *Handler {
	return &Handler{
		temporalClient: temporalClient,
		cfg:            cfg,
	}
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "Agentic Task Executor",
		"version": "0.1.0",
		"docs":    "/docs",
	})
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *Handler) ExecuteTask(w http.ResponseWriter, r *http.Request) {
	var params TaskParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "Invalid request body: " + err.Error()})
		return
	}

	if params.RepoURL == "" || params.TaskDescription == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "repo_url and task_description are required"})
		return
	}

	workflowID := "task-" + uuid.New().String()

	workflowInput := workflows.WorkflowInput{
		RepoURL:         params.RepoURL,
		TaskDescription: params.TaskDescription,
		Checklist:       params.Checklist,
		Context:         params.Context,
		WaitForCI:       params.WaitForCI,
		BranchName:      params.BranchName,
	}

	slog.Info("Starting workflow", "workflow_id", workflowID, "repo", params.RepoURL)

	run, err := h.temporalClient.ExecuteWorkflow(r.Context(),
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: h.cfg.TemporalTaskQueue,
		},
		workflows.AgenticTaskWorkflow,
		workflowInput,
	)
	if err != nil {
		slog.Error("Failed to start workflow", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Detail: "Failed to start task execution: " + err.Error()})
		return
	}

	slog.Info("Workflow started", "workflow_id", workflowID, "run_id", run.GetRunID())

	writeJSON(w, http.StatusAccepted, TaskResponse{
		WorkflowID: workflowID,
		RunID:      run.GetRunID(),
		Status:     "running",
	})
}

func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	desc, err := h.temporalClient.DescribeWorkflowExecution(r.Context(), workflowID, "")
	if err != nil {
		slog.Error("Failed to describe workflow", "workflow_id", workflowID, "error", err)
		writeJSON(w, http.StatusNotFound, ErrorResponse{Detail: "Workflow not found or query failed: " + err.Error()})
		return
	}

	statusMap := map[enums.WorkflowExecutionStatus]string{
		enums.WORKFLOW_EXECUTION_STATUS_RUNNING:          "running",
		enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:        "completed",
		enums.WORKFLOW_EXECUTION_STATUS_FAILED:           "failed",
		enums.WORKFLOW_EXECUTION_STATUS_CANCELED:         "canceled",
		enums.WORKFLOW_EXECUTION_STATUS_TERMINATED:       "terminated",
		enums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW: "running",
		enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:        "timed_out",
	}

	execInfo := desc.WorkflowExecutionInfo
	wfStatus := statusMap[execInfo.GetStatus()]
	if wfStatus == "" {
		wfStatus = "unknown"
	}

	resp := TaskStatus{
		WorkflowID: workflowID,
		RunID:      execInfo.Execution.GetRunId(),
		Status:     wfStatus,
	}

	if execInfo.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
		handle := h.temporalClient.GetWorkflow(r.Context(), workflowID, "")
		var result workflows.WorkflowResult
		if err := handle.Get(r.Context(), &result); err == nil {
			resp.Result = map[string]any{
				"success": result.Success,
				"summary": result.Summary,
				"details": result.Details,
				"pr_url":  result.PRURL,
			}
		}
	} else if execInfo.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_FAILED {
		resp.Error = "Workflow execution failed"
	} else if execInfo.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		queryResult, err := h.temporalClient.QueryWorkflow(r.Context(), workflowID, "", "get_status")
		if err == nil {
			var queryData map[string]any
			if err := queryResult.Get(&queryData); err == nil {
				resp.Result = queryData
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SignalWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	var req SignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "Invalid request body: " + err.Error()})
		return
	}

	if req.SignalName == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "signal_name is required"})
		return
	}

	err := h.temporalClient.SignalWorkflow(r.Context(), workflowID, "", req.SignalName, req.SignalArgs)
	if err != nil {
		slog.Error("Failed to signal workflow", "workflow_id", workflowID, "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "Failed to send signal: " + err.Error()})
		return
	}

	slog.Info("Signal sent", "workflow_id", workflowID, "signal", req.SignalName)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"message":     "Signal '" + req.SignalName + "' sent successfully to workflow " + workflowID,
		"workflow_id": workflowID,
		"signal_name": req.SignalName,
	})
}

func (h *Handler) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")

	err := h.temporalClient.CancelWorkflow(r.Context(), workflowID, "")
	if err != nil {
		slog.Error("Failed to cancel workflow", "workflow_id", workflowID, "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Detail: "Failed to cancel workflow: " + err.Error()})
		return
	}

	slog.Info("Workflow cancelled", "workflow_id", workflowID)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"message":     "Workflow " + workflowID + " cancellation requested",
		"workflow_id": workflowID,
	})
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 100 {
		limit = 100
	}

	_ = limit
	// Placeholder — same as the Python version
	writeJSON(w, http.StatusOK, []TaskStatus{})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
