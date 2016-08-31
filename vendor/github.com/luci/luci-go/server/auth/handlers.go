// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/router"
)

// InstallHandlers installs authentication related HTTP handlers.
//
// All new HTTP routes live under '/auth/api/' prefix.
//
// If you are using appengine/gaeauth/server, these handlers are already
// installed.
func InstallHandlers(r *router.Router, base router.MiddlewareChain) {
	r.GET("/auth/api/v1/accounts/self", base.Extend(Authenticate), accountsSelfHandler)
	r.GET("/auth/api/v1/server/certificates", base, certsHandler)
	r.GET("/auth/api/v1/server/info", base, infoHandler)
}

// accountsSelfHandler returns JSON with information about the caller.
//
// It can be used to verify callers' IP, access tokens, etc.
func accountsSelfHandler(c *router.Context) {
	state := GetState(c.Context)
	if state == nil {
		httpReplyError(c, http.StatusNotImplemented, "Auth state is not available, application is probably using auth library wrong.")
		return
	}
	var reply struct {
		Identity string `json:"identity,omitempty"`
		IP       string `json:"ip,omitempty"`
	}
	reply.Identity = string(state.User().Identity)
	reply.IP = state.PeerIP().String()
	httpReply(c, http.StatusOK, &reply)
}

// certsHandler servers public certificates of the signer in the context.
func certsHandler(c *router.Context) {
	s := GetSigner(c.Context)
	if s == nil {
		httpReplyError(c, http.StatusNotFound, "No Signer instance available")
		return
	}
	certs, err := s.Certificates(c.Context)
	if err != nil {
		httpReplyError(c, http.StatusInternalServerError, fmt.Sprintf("Can't fetch certificates - %s", err))
	} else {
		httpReply(c, http.StatusOK, certs)
	}
}

// infoHandler returns information about the current service identity.
func infoHandler(c *router.Context) {
	s := GetSigner(c.Context)
	if s == nil {
		httpReplyError(c, http.StatusNotFound, "No Signer instance available")
		return
	}
	info, err := s.ServiceInfo(c.Context)
	if err != nil {
		httpReplyError(c, http.StatusInternalServerError, fmt.Sprintf("Can't grab service info - %s", err))
	} else {
		httpReply(c, http.StatusOK, info)
	}
}

////

func httpReply(c *router.Context, code int, out interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	if err := json.NewEncoder(c.Writer).Encode(out); err != nil {
		logging.Errorf(c.Context, "Failed to JSON encode output - %s", err)
	}
}

func httpReplyError(c *router.Context, code int, msg string) {
	errorReply := struct {
		Error string `json:"error"`
	}{msg}
	httpReply(c, code, &errorReply)
}
