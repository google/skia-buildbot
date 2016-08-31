// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

import (
	"fmt"
	"runtime"
	"time"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	ps "github.com/luci/luci-go/common/gcloud/pubsub"
	"github.com/luci/luci-go/common/lhttp"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/grpc/prpc"
	api "github.com/luci/luci-go/logdog/api/endpoints/coordinator/registration/v1"
	"github.com/luci/luci-go/logdog/client/butler/output"
	out "github.com/luci/luci-go/logdog/client/butler/output/pubsub"
	"github.com/luci/luci-go/logdog/common/types"

	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

// Scopes returns the set of OAuth scopes required for this Output.
func Scopes() []string {
	// E-mail scope needed for Coordinator authentication.
	scopes := []string{auth.OAuthScopeEmail}
	// Publisher scope needed to publish to Pub/Sub transport.
	scopes = append(scopes, ps.PublisherScopes...)

	return scopes
}

// Config is the set of configuration parameters for this Output instance.
type Config struct {
	// Auth is the Authenticator to use for registration and publishing. It should
	// be configured to hold the scopes returned by Scopes.
	Auth *auth.Authenticator

	// Host is the name of the LogDog Host to connect to.
	Host string

	// Project is the project that this stream belongs to.
	Project config.ProjectName
	// Prefix is the stream prefix to register.
	Prefix types.StreamName
	// PrefixExpiration is the prefix expiration to use when registering.
	// If zero, no expiration will be expressed to the Coordinator, and it will
	// choose based on its configuration.
	PrefixExpiration time.Duration

	// SourceInfo, if not empty, is auxiliary source information to register
	// alongside the stream.
	SourceInfo []string

	// PublishContext is the special Context to use for publishing messages. If
	// nil, the Context supplied to Register will be used.
	//
	// This is useful when the Context supplied to Register responds to
	// cancellation (e.g., user sends SIGTERM), but we might not want to
	// immediately cancel pending publishes due to flushing.
	PublishContext context.Context

	// Track, if true, instructs this Output instance to track all log entries
	// that have been sent in-memory. This is useful for debugging.
	Track bool
}

// Register registers the supplied Prefix with the Coordinator. Upon success,
// an Output instance bound to that stream will be returned.
func (cfg *Config) Register(c context.Context) (output.Output, error) {
	// Validate our configuration parameters.
	switch {
	case cfg.Auth == nil:
		return nil, errors.New("no authenticator supplied")
	case cfg.Host == "":
		return nil, errors.New("no host supplied")
	}
	if err := cfg.Project.Validate(); err != nil {
		return nil, errors.Annotate(err).Reason("failed to validate project").
			D("project", cfg.Project).Err()
	}
	if err := cfg.Prefix.Validate(); err != nil {
		return nil, errors.Annotate(err).Reason("failed to validate prefix").
			D("prefix", cfg.Prefix).Err()
	}

	// Open a pRPC client to our Coordinator instance.
	httpClient, err := cfg.Auth.Client()
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get authenticated HTTP client.")
		return nil, err
	}

	// Configure our pRPC client.
	client := prpc.Client{
		C:       httpClient,
		Host:    cfg.Host,
		Options: prpc.DefaultOptions(),
	}

	// If our host begins with "localhost", set insecure option automatically.
	if lhttp.IsLocalHost(cfg.Host) {
		log.Infof(c, "Detected localhost; enabling insecure RPC connection.")
		client.Options.Insecure = true
	}

	// Register our Prefix with the Coordinator.
	log.Fields{
		"prefix": cfg.Prefix,
		"host":   cfg.Host,
	}.Debugf(c, "Registering prefix space with Coordinator service.")

	// Build our source info.
	sourceInfo := make([]string, 0, len(cfg.SourceInfo)+2)
	sourceInfo = append(sourceInfo, cfg.SourceInfo...)
	sourceInfo = append(sourceInfo,
		fmt.Sprintf("GOARCH=%s", runtime.GOARCH),
		fmt.Sprintf("GOOS=%s", runtime.GOOS),
	)

	svc := api.NewRegistrationPRPCClient(&client)
	resp, err := svc.RegisterPrefix(c, &api.RegisterPrefixRequest{
		Project:    string(cfg.Project),
		Prefix:     string(cfg.Prefix),
		SourceInfo: sourceInfo,
		Expiration: google.NewDuration(cfg.PrefixExpiration),
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to register prefix with Coordinator service.")
		return nil, err
	}
	log.Fields{
		"prefix":      cfg.Prefix,
		"bundleTopic": resp.LogBundleTopic,
	}.Debugf(c, "Successfully registered log stream prefix.")

	// Validate the response topic.
	fullTopic := ps.Topic(resp.LogBundleTopic)
	if err := fullTopic.Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"fullTopic":  fullTopic,
		}.Errorf(c, "Coordinator returned invalid Pub/Sub topic.")
		return nil, err
	}

	// Split our topic into project and topic name. This must succeed, since we
	// just finished validating the topic.
	proj, topic := fullTopic.Split()

	// Instantiate our Pub/Sub instance.
	//
	// We will use the non-cancelling context, for all Pub/Sub calls, as we want
	// the Pub/Sub system to drain without interruption if the application is
	// otherwise canceled.
	pctx := cfg.PublishContext
	if pctx == nil {
		pctx = c
	}

	psClient, err := pubsub.NewClient(pctx, proj, option.WithTokenSource(cfg.Auth.TokenSource()))
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"project":    proj,
		}.Errorf(c, "Failed to create Pub/Sub client.")
		return nil, errors.New("failed to get Pub/Sub client")
	}
	psTopic := psClient.Topic(topic)

	// Assert that our Topic exists.
	exists, err := retryTopicExists(c, psTopic)
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"project":    proj,
			"topic":      topic,
		}.Errorf(c, "Failed to check for Pub/Sub topic.")
		return nil, errors.New("failed to check for Pub/Sub topic")
	}
	if !exists {
		log.Fields{
			"fullTopic": fullTopic,
		}.Errorf(c, "Pub/Sub Topic does not exist.")
		return nil, errors.New("PubSub topic does not exist")
	}

	// We own the prefix and all verifiable parameters have been validated.
	// Successfully return our Output instance.
	//
	// Note that we use our publishing context here.
	return out.New(pctx, out.Config{
		Topic:    psTopic,
		Secret:   resp.Secret,
		Compress: true,
		Track:    cfg.Track,
	}), nil
}

func retryTopicExists(ctx context.Context, t *pubsub.Topic) (bool, error) {
	var exists bool
	err := retry.Retry(ctx, retry.Default, func() (err error) {
		exists, err = t.Exists(ctx)
		return
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"delay":      d,
		}.Errorf(ctx, "Failed to check if topic exists; retrying...")
	})
	return exists, err
}
