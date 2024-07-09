package api

import (
	"github.com/go-chi/chi/v5"
)

// FrontendApi provides an interface for frontend apis to implement.
type FrontendApi interface {
	// RegisterHandlers registers the api handlers for their respective routes.
	RegisterHandlers(*chi.Mux)
}
