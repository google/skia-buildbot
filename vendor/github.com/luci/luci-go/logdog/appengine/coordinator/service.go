// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/luci/luci-go/appengine/gaemiddleware"
	luciConfig "github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/gcloud/gs"
	"github.com/luci/luci-go/common/gcloud/pubsub"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/appengine/coordinator/config"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/storage/bigtable"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"

	gcps "cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/grpc/metadata"
)

// Services is a set of support services used by Coordinator.
//
// Each Services instance is valid for a singel request, but can be re-used
// throughout that request. This is advised, as the Services instance may
// optionally cache values.
//
// Services methods are goroutine-safe.
//
// By default, a production set of services will be used. However, this can be
// overridden for testing to mock the service layer.
type Services interface {
	// Config returns the current instance and application configuration
	// instances.
	//
	// The production instance will cache the results for the duration of the
	// request.
	Config(context.Context) (*config.Config, error)

	// ProjectConfig returns the project configuration for the named project.
	//
	// The production instance will cache the results for the duration of the
	// request.
	ProjectConfig(context.Context, luciConfig.ProjectName) (*svcconfig.ProjectConfig, error)

	// Storage returns an intermediate storage instance for use by this service.
	//
	// The caller must close the returned instance if successful.
	IntermediateStorage(context.Context) (storage.Storage, error)

	// GSClient instantiates a Google Storage client.
	GSClient(context.Context) (gs.Client, error)

	// ArchivalPublisher returns an ArchivalPublisher instance.
	ArchivalPublisher(context.Context) (ArchivalPublisher, error)
}

// ProdServices is middleware chain used by Coordinator services.
//
// It sets up basic GAE functionality as well as installs a production Services
// instance.
func ProdServices() router.MiddlewareChain {
	return gaemiddleware.BaseProd().Extend(func(c *router.Context, next router.Handler) {
		c.Context = WithServices(c.Context, &prodServicesInst{})
		next(c)
	})
}

// prodServicesInst is a Service exposing production faciliites. A unique
// instance is bound to each each request.
type prodServicesInst struct {
	sync.Mutex

	// gcfg is the cached global configuration.
	gcfg           *config.Config
	projectConfigs map[luciConfig.ProjectName]*cachedProjectConfig

	// archivalIndex is the atomically-manipulated archival index for the
	// ArchivalPublisher. This is shared between all ArchivalPublisher instances
	// from this service.
	archivalIndex int32
}

func (s *prodServicesInst) Config(c context.Context) (*config.Config, error) {
	s.Lock()
	defer s.Unlock()

	// Load/cache the global config.
	if s.gcfg == nil {
		var err error
		s.gcfg, err = config.Load(c)
		if err != nil {
			return nil, err
		}
	}

	return s.gcfg, nil
}

// cachedProjectConfig is a singleton instance that holds a project config
// state. It is populated when resolve is called, and is goroutine-safe for
// read-only operations.
type cachedProjectConfig struct {
	sync.Once

	project luciConfig.ProjectName
	pcfg    *svcconfig.ProjectConfig
	err     error
}

func (cp *cachedProjectConfig) resolve(c context.Context) (*svcconfig.ProjectConfig, error) {
	// Load the project config exactly once. This will be cached for the remainder
	// of this request.
	//
	// If multiple goroutines attempt to load it, exactly one will, and the rest
	// will block. All operations after this Once must be read-only.
	cp.Do(func() {
		cp.pcfg, cp.err = config.ProjectConfig(c, cp.project)
	})
	return cp.pcfg, cp.err
}

func (s *prodServicesInst) getOrCreateCachedProjectConfig(project luciConfig.ProjectName) *cachedProjectConfig {
	s.Lock()
	defer s.Unlock()

	if s.projectConfigs == nil {
		s.projectConfigs = make(map[luciConfig.ProjectName]*cachedProjectConfig)
	}
	cp := s.projectConfigs[project]
	if cp == nil {
		cp = &cachedProjectConfig{
			project: project,
		}
		s.projectConfigs[project] = cp
	}
	return cp
}

func (s *prodServicesInst) ProjectConfig(c context.Context, project luciConfig.ProjectName) (*svcconfig.ProjectConfig, error) {
	return s.getOrCreateCachedProjectConfig(project).resolve(c)
}

func (s *prodServicesInst) IntermediateStorage(c context.Context) (storage.Storage, error) {
	cfg, err := s.Config(c)
	if err != nil {
		return nil, err
	}

	// Is BigTable configured?
	if cfg.Storage == nil {
		return nil, errors.New("no storage configuration")
	}

	bt := cfg.Storage.GetBigtable()
	if bt == nil {
		return nil, errors.New("no BigTable configuration")
	}

	// Validate the BigTable configuration.
	log.Fields{
		"project":      bt.Project,
		"instance":     bt.Instance,
		"logTableName": bt.LogTableName,
	}.Debugf(c, "Connecting to BigTable.")
	var merr errors.MultiError
	if bt.Project == "" {
		merr = append(merr, errors.New("missing project"))
	}
	if bt.Instance == "" {
		merr = append(merr, errors.New("missing instance"))
	}
	if bt.LogTableName == "" {
		merr = append(merr, errors.New("missing log table name"))
	}
	if len(merr) > 0 {
		return nil, merr
	}

	// Get an Authenticator bound to the token scopes that we need for BigTable.
	transport, err := auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(bigtable.StorageScopes...))
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create BigTable authenticator.")
		return nil, errors.New("failed to create BigTable authenticator")
	}

	// Explicitly clear gRPC metadata from the Context. It is forwarded to
	// delegate calls by default, and standard request metadata can break BigTable
	// calls.
	c = metadata.NewContext(c, nil)

	st, err := bigtable.New(c, bigtable.Options{
		Project:  bt.Project,
		Instance: bt.Instance,
		LogTable: bt.LogTableName,
		ClientOptions: []option.ClientOption{
			option.WithHTTPClient(&http.Client{Transport: transport}),
		},
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create BigTable instance.")
		return nil, err
	}
	return st, nil
}

func (s *prodServicesInst) GSClient(c context.Context) (gs.Client, error) {
	// Get an Authenticator bound to the token scopes that we need for
	// authenticated Cloud Storage access.
	transport, err := auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(gs.ReadOnlyScopes...))
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Cloud Storage transport.")
		return nil, errors.New("failed to create Cloud Storage transport")
	}
	return gs.NewProdClient(c, transport)
}

func (s *prodServicesInst) ArchivalPublisher(c context.Context) (ArchivalPublisher, error) {
	cfg, err := s.Config(c)
	if err != nil {
		return nil, err
	}

	fullTopic := pubsub.Topic(cfg.Coordinator.ArchiveTopic)
	if err := fullTopic.Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"topic":      fullTopic,
		}.Errorf(c, "Failed to validate archival topic.")
		return nil, errors.New("invalid archival topic")
	}
	project, topic := fullTopic.Split()

	// Create an authenticated Pub/Sub client.
	transport, err := auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(pubsub.PublisherScopes...))
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Pub/Sub authenticator.")
		return nil, errors.New("failed to create Pub/Sub authenticator")
	}
	client := &http.Client{Transport: transport}
	psClient, err := gcps.NewClient(c, project, option.WithHTTPClient(client))
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Pub/Sub client.")
		return nil, errors.New("failed to create Pub/Sub client")
	}

	return &pubsubArchivalPublisher{
		topic:            psClient.Topic(topic),
		publishIndexFunc: s.nextArchiveIndex,
	}, nil
}

func (s *prodServicesInst) nextArchiveIndex() uint64 {
	// We use a 32-bit value for this because it avoids atomic memory bounary
	// issues. Furthermore, we constrain it to be positive, using a negative
	// value as a sentinel that the archival index has wrapped.
	//
	// This is reasonable, as it is very unlikely that a single request will issue
	// more than MaxInt32 archival tasks.
	v := atomic.AddInt32(&s.archivalIndex, 1) - 1
	if v < 0 {
		panic("archival index has wrapped")
	}
	return uint64(v)
}
