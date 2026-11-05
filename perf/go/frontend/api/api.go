// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"github.com/go-chi/chi/v5"
)

// FrontendApi provides an interface for frontend apis to implement.
type FrontendApi interface {
	// RegisterHandlers registers the api handlers for their respective routes.
	RegisterHandlers(*chi.Mux)
}
