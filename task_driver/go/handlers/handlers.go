package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/logs"
)

// logsHandler reads log entries from BigTable and writes them to the ResponseWriter.
func logsHandler(w http.ResponseWriter, r *http.Request, lm *logs.LogsManager, taskId, stepId, logId string) {
	// TODO(borenet): If we had access to the Task Driver DB, we could first
	// retrieve the run and then limit our search to its duration. That
	// might speed up the search quite a bit.
	w.Header().Set("Content-Type", "text/plain")
	entries, err := lm.Search(taskId, stepId, logId)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to search log entries.")
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
			httputils.ReportError(w, r, err, "Failed to write response.")
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
func getTaskDriver(w http.ResponseWriter, r *http.Request, d db.DB) *db.TaskDriverRun {
	id := getVar(w, r, "taskId")
	if id == "" {
		return nil
	}
	td, err := d.GetTaskDriver(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve task driver.")
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
func getTaskDriverDisplay(w http.ResponseWriter, r *http.Request, d db.DB) *display.TaskDriverRunDisplay {
	td := getTaskDriver(w, r, d)
	if td == nil {
		// Any error was handled by getTaskDriver.
		return nil
	}
	disp, err := display.TaskDriverForDisplay(td)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to format task driver for response.")
		return nil
	}
	return disp
}

// jsonTaskDriverHandler returns the JSON representation of the requested Task Driver.
func jsonTaskDriverHandler(d db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		disp := getTaskDriverDisplay(w, r, d)
		if disp == nil {
			// Any error was handled by getTaskDriverDisplay.
			return
		}

		if err := json.NewEncoder(w).Encode(disp); err != nil {
			httputils.ReportError(w, r, err, "Failed to encode response.")
			return
		}
	}
}

// AddTaskDriverHandlers adds handlers for Task Drivers to the given Router.
func AddTaskDriverHandlers(r *mux.Router, d db.DB, lm *logs.LogsManager) {
	r.HandleFunc("/json/td/{taskId}", jsonTaskDriverHandler(d))
	r.HandleFunc("/logs/{taskId}", taskLogsHandler(lm))
	r.HandleFunc("/logs/{taskId}/{stepId}", stepLogsHandler(lm))
	r.HandleFunc("/logs/{taskId}/{stepId}/{logId}", singleLogHandler(lm))
}
