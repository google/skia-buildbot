package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.opencensus.io/trace"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/logs"
	"go.skia.org/infra/task_driver/go/td"
)

// logsHandler reads log entries from BigTable and writes them to the ResponseWriter.
func logsHandler(w http.ResponseWriter, r *http.Request, lm *logs.LogsManager, taskId, stepId, logId string) {
	// TODO(borenet): If we had access to the Task Driver DB, we could first
	// retrieve the run and then limit our search to its duration. That
	// might speed up the search quite a bit.
	w.Header().Set("Content-Type", "text/plain")
	entries, err := lm.Search(r.Context(), taskId, stepId, logId)
	if err != nil {
		httputils.ReportError(w, err, "Failed to search log entries.", http.StatusInternalServerError)
		return
	}
	if len(entries) == 0 {
		// TODO(borenet): Maybe an empty log is not the same as a
		// missing log?
		http.Error(w, fmt.Sprintf("No matching log entries were found."), http.StatusNotFound)
		return
	}
	for _, e := range entries {
		line := e.TextPayload
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		if _, err := w.Write([]byte(line)); err != nil {
			httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	}
}

// getVar returns the variable which should be present in the request path.
// It returns "" if it is not found, in which case it also writes an error to
// the ResponseWriter.
func getVar(w http.ResponseWriter, r *http.Request, key string) string {
	val, ok := mux.Vars(r)[key]
	if !ok {
		http.Error(w, fmt.Sprintf("No %s in request path.", key), http.StatusBadRequest)
		return ""
	}
	return val
}

// taskLogsHandler returns a handler which serves logs for a given task.
func taskLogsHandler(lm *logs.LogsManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskId := getVar(w, r, "taskId")
		if taskId == "" {
			return
		}
		logsHandler(w, r, lm, taskId, "", "")
	}
}

// stepLogsHandler returns a handler which serves logs for a given step.
func stepLogsHandler(lm *logs.LogsManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskId := getVar(w, r, "taskId")
		stepId := getVar(w, r, "stepId")
		if taskId == "" || stepId == "" {
			return
		}
		logsHandler(w, r, lm, taskId, stepId, "")
	}
}

// singleLogHandler returns a handler which serves logs for a single log ID.
func singleLogHandler(lm *logs.LogsManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskId := getVar(w, r, "taskId")
		stepId := getVar(w, r, "stepId")
		logId := getVar(w, r, "logId")
		if taskId == "" || stepId == "" || logId == "" {
			return
		}
		logsHandler(w, r, lm, taskId, stepId, logId)
	}
}

// getTaskDriver returns a db.TaskDriverRun instance for the given request. If
// anything went wrong, returns nil and writes an error to the ResponseWriter.
func getTaskDriver(ctx context.Context, w http.ResponseWriter, r *http.Request, d db.DB) *db.TaskDriverRun {
	ctx, span := trace.StartSpan(ctx, "getTaskDriverDisplay")
	defer span.End()
	id := getVar(w, r, "taskId")
	if id == "" {
		return nil
	}
	td, err := d.GetTaskDriver(ctx, id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve task driver.", http.StatusInternalServerError)
		return nil
	}
	if td == nil {
		http.Error(w, "No task driver exists with the given ID.", http.StatusNotFound)
		return nil
	}
	return td
}

// getTaskDriverDisplay returns a display.TaskDriverRunDisplay instance for the
// given request. If anything went wrong, returns nil and writes an error to the
// ResponseWriter.
func getTaskDriverDisplay(ctx context.Context, w http.ResponseWriter, r *http.Request, d db.DB) *display.TaskDriverRunDisplay {
	ctx, span := trace.StartSpan(ctx, "getTaskDriverDisplay")
	defer span.End()
	td := getTaskDriver(ctx, w, r, d)
	if td == nil {
		// Any error was handled by getTaskDriver.
		return nil
	}
	disp, err := display.TaskDriverForDisplay(td)
	if err != nil {
		httputils.ReportError(w, err, "Failed to format task driver for response.", http.StatusInternalServerError)
		return nil
	}
	return disp
}

// jsonTaskDriverHandler returns the JSON representation of the requested Task Driver.
func jsonTaskDriverHandler(d db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.StartSpan(r.Context(), "jsonTaskDriverHandler")
		defer span.End()
		disp := getTaskDriverDisplay(ctx, w, r, d)
		if disp == nil {
			// Any error was handled by getTaskDriverDisplay.
			return
		}

		if err := json.NewEncoder(w).Encode(disp); err != nil {
			httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
			return
		}
	}
}

// fullErrorHandler returns the text of a given error.
func fullErrorHandler(d db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.StartSpan(r.Context(), "fullErrorHandler")
		defer span.End()
		taskId := getVar(w, r, "taskId")
		errId := getVar(w, r, "errId")
		if taskId == "" || errId == "" {
			http.NotFound(w, r)
			return
		}
		stepId, ok := mux.Vars(r)["stepId"]
		if !ok {
			stepId = td.StepIDRoot
		}
		errIdx, err := strconv.Atoi(errId)
		if err != nil || errIdx < 0 {
			httputils.ReportError(w, err, "Invalid error ID", http.StatusInternalServerError)
			return
		}
		td := getTaskDriver(ctx, w, r, d)
		if td == nil {
			// Any error was handled by getTaskDriver.
			return
		}
		step, ok := td.Steps[stepId]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if errIdx >= len(step.Errors) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(step.Errors[errIdx])); err != nil {
			httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	}
}

// AddTaskDriverHandlers adds handlers for Task Drivers to the given Router.
func AddTaskDriverHandlers(r *mux.Router, d db.DB, lm *logs.LogsManager) {
	r.HandleFunc("/json/td/{taskId}", httputils.CorsHandler(jsonTaskDriverHandler(d)))
	r.HandleFunc("/errors/{taskId}/{errId}", fullErrorHandler(d))
	r.HandleFunc("/errors/{taskId}/{stepId}/{errId}", fullErrorHandler(d))
	r.HandleFunc("/logs/{taskId}", taskLogsHandler(lm))
	r.HandleFunc("/logs/{taskId}/{stepId}", stepLogsHandler(lm))
	r.HandleFunc("/logs/{taskId}/{stepId}/{logId}", singleLogHandler(lm))
}
