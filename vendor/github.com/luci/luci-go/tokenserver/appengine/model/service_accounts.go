// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"time"

	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/tokenserver/api"
)

// ServiceAccount is a single registered Cloud IAM service account.
//
// We store them in the datastore to avoid hitting Cloud IAM API all the time.
type ServiceAccount struct {
	ID string `gae:"$id"` // "<projectId>/<accountId>", root entity

	// Fields extracted from Cloud IAM response.

	ProjectID      string // Cloud Project ID that owns the account, for indexing
	UniqueID       string `gae:",noindex"` // globally unique account ID (usually int64 string)
	Email          string `gae:",noindex"` // service account email
	DisplayName    string `gae:",noindex"` // how account shows up in Cloud Console
	OAuth2ClientID string `gae:",noindex"` // OAuth2 client id for the service account

	// Fields managed by the token server.

	FQDN       string    `gae:",noindex"` // FQDN of an associated host that uses this service account
	Registered time.Time // when this entity was created, indexed (just in case)
}

// GetProto converts this entity to corresponding protobuf message for API.
func (s *ServiceAccount) GetProto() *tokenserver.ServiceAccount {
	return &tokenserver.ServiceAccount{
		ProjectId:      s.ProjectID,
		UniqueId:       s.UniqueID,
		Email:          s.Email,
		DisplayName:    s.DisplayName,
		Oauth2ClientId: s.OAuth2ClientID,
		Fqdn:           s.FQDN,
		Registered:     google.NewTimestamp(s.Registered.UTC()),
	}
}
