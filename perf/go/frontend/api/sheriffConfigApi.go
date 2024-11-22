package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	sheriffconfig "go.skia.org/infra/perf/go/sheriffconfig/service"
)

// sheriffConfigApi provides a struct for handling Sheriff Config requests.
type sheriffConfigApi struct {
	loginProvider alogin.Login
}

// NewSheriffConfigApi returns a new instance of the sheriffConfigApi struct.
func NewSheriffConfigApi(loginProvider alogin.Login) sheriffConfigApi {
	return sheriffConfigApi{
		loginProvider: loginProvider,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api sheriffConfigApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/configs/metadata", api.getMetadataHandler)
	router.Post("/_/configs/validate", api.validateConfigHandler)
}

type Pattern struct {
	ConfigSet string `json:"config_set"`
	Path      string `json:"path"`
}

type Validation struct {
	Patterns []Pattern `json:"patterns"`
	Url      string    `json:"url"`
}

type GetMetadataResponse struct {
	Version    string     `json:"version"`
	Validation Validation `json:"validation"`
}

// getMetadataHandler is a GET Method that'll be used by LUCI Config to determine which
// configuration files in infra_internal are owned by us. It'll provide a Url for validation
// for any changes to these files.
func (api sheriffConfigApi) getMetadataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !api.loginProvider.HasRole(r, roles.LuciConfig) {
		sklog.Infof("User is not authorized: %s", api.loginProvider.LoggedInAs(r).String())
		httputils.ReportError(w, nil, "Permission denied", http.StatusForbidden)
		return
	}

	resp := GetMetadataResponse{
		Version: "1.0",
		Validation: Validation{
			Patterns: []Pattern{
				{
					ConfigSet: "regex:projects/.+",
					Path:      "regex:skia-sheriff-configs.cfg",
				},
			},
			Url: "https://perf.luci.app/_/configs/validate",
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

type ValidateConfigRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Message struct {
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Text     string `json:"text"`
}

type ValidateConfigResponse struct {
	Messages []Message `json:"messages"`
}

// validateConfigHandler will receive incoming validation requests from LUCI Config.
// It'll return an empty object and status 200 if the validation is succesful. If the validation
// fails, it'll return status 200 and a formatted error message to LUCI Config.
//
// Example ValidateConfigRequest:
//
//	{
//		"content": "c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKfQ=="
//	}
func (api sheriffConfigApi) validateConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !api.loginProvider.HasRole(r, roles.LuciConfig) {
		sklog.Infof("User is not authorized: %s", api.loginProvider.LoggedInAs(r).String())
		httputils.ReportError(w, nil, "Permission denied", http.StatusForbidden)
		return
	}

	vcr := ValidateConfigRequest{}
	if err := json.NewDecoder(r.Body).Decode(&vcr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	err := sheriffconfig.ValidateContent(vcr.Content)
	if err != nil {
		ret := ValidateConfigResponse{
			Messages: []Message{
				{
					Path:     vcr.Path,
					Severity: "ERROR",
					Text:     err.Error(),
				},
			},
		}
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			sklog.Errorf("Failed to write or encode output: %s", err)
		}
	}

}
