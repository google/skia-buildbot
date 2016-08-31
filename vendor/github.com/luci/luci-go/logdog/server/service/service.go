// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package service

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/common/auth"
	luciConfig "github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/common/gcloud/gs"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/storage/bigtable"
	"github.com/luci/luci-go/logdog/server/retryServicesClient"
	"github.com/luci/luci-go/logdog/server/service/config"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

var (
	// ErrInvalidConfig is an error that is returned when the supplied
	// configuration is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")

	// CoordinatorScopes is the set of OAuth2 scopes to use for the Coordinator
	// client.
	CoordinatorScopes = []string{
		auth.OAuthScopeEmail,
	}
)

// projectConfigCacheDuration is the amount of time to cache a project's
// configuration before reloading.
const projectConfigCacheDuration = 30 * time.Minute

// Service is a base class full of common LogDog service application parameters.
type Service struct {
	// Name is the name of this service. It is used for logging, metrics, and
	// user agent string generation.
	//
	// If empty, a service name will be inferred from the command-line arguments.
	Name string
	// Flags is the set of flags that will be used by the Service.
	Flags flag.FlagSet

	shutdownFunc atomic.Value

	loggingFlags log.Config
	authFlags    authcli.Flags
	configFlags  config.Flags
	tsMonFlags   tsmon.Flags

	coordinatorHost           string
	coordinatorInsecure       bool
	serviceID                 string
	storageCredentialJSONPath string
	cpuProfilePath            string
	heapProfilePath           string

	coord  logdog.ServicesClient
	config *config.Manager

	onGCE bool
}

// Run performs service-wide initialization and invokes the specified run
// function.
func (s *Service) Run(c context.Context, f func(context.Context) error) {
	c = gologger.StdConfig.Use(c)

	// If a service name isn't specified, default to the base of the current
	// executable.
	if s.Name == "" {
		s.Name = filepath.Base(os.Args[0])
	}

	rc := 0
	if err := s.runImpl(c, f); err != nil {
		log.WithError(err).Errorf(c, "Application exiting with error.")
		rc = 1
	}
	os.Exit(rc)
}

func (s *Service) runImpl(c context.Context, f func(context.Context) error) error {
	s.addFlags(c, &s.Flags)
	if err := s.Flags.Parse(os.Args[1:]); err != nil {
		log.WithError(err).Errorf(c, "Failed to parse command-line.")
		return err
	}

	// Install logging configuration.
	c = s.loggingFlags.Set(c)

	if p := s.cpuProfilePath; p != "" {
		fd, err := os.Create(p)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"path":       p,
			}.Errorf(c, "Failed to create CPU profile output file.")
			return err
		}
		defer fd.Close()

		pprof.StartCPUProfile(fd)
		defer pprof.StopCPUProfile()
	}

	if p := s.heapProfilePath; p != "" {
		defer func() {
			fd, err := os.Create(p)
			if err != nil {
				log.Fields{
					log.ErrorKey: err,
					"path":       p,
				}.Warningf(c, "Failed to create heap profile output file.")
				return
			}
			defer fd.Close()

			if err := pprof.WriteHeapProfile(fd); err != nil {
				log.Fields{
					log.ErrorKey: err,
					"path":       p,
				}.Warningf(c, "Failed to write heap profile.")
			}
		}()
	}

	// Are we running on a GCE intance?
	if err := s.probeGCEEnvironment(c); err != nil {
		log.WithError(err).Errorf(c, "Failed to probe GCE environment.")
		return err
	}

	// Validate the runtime environment.
	if s.serviceID == "" {
		return errors.New("no service ID was configured")
	}

	// Configure our signal handler. It will listen for terminating signals and
	// issue a shutdown signal if one is received.
	signalC := make(chan os.Signal)
	go func(c context.Context) {
		hasShutdownAlready := false
		for sig := range signalC {
			if !hasShutdownAlready {
				hasShutdownAlready = true

				log.Warningf(log.SetField(c, "signal", sig), "Received close signal. Send again to terminate immediately.")
				s.shutdown()
				continue
			}

			// No shutdown function registered; just exit immediately.
			s.shutdownImmediately()
			panic("never reached")
		}
	}(c)
	signal.Notify(signalC, os.Interrupt)
	defer func() {
		signal.Stop(signalC)
		close(signalC)
	}()

	// Initialize our tsmon library.
	c = tsmon.WithState(c, tsmon.NewState())

	if err := tsmon.InitializeFromFlags(c, &s.tsMonFlags); err != nil {
		log.WithError(err).Warningf(c, "Failed to initialize monitoring; will continue without metrics.")
	}
	defer tsmon.Shutdown(c)

	// Initialize our Client instantiations.
	var err error
	s.coord, err = s.initCoordinatorClient(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to setup Coordinator client.")
		return err
	}

	s.config, err = s.initConfig(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to setup configuration.")
		return err
	}
	defer s.config.Close()

	defer s.SetShutdownFunc(nil)

	// Install a process-wide cache.
	c = proccache.Use(c, &proccache.Cache{})

	// Run main service function.
	return f(c)
}

func (s *Service) addFlags(c context.Context, fs *flag.FlagSet) {
	// Initialize logging flags.
	s.loggingFlags.Level = log.Warning
	s.loggingFlags.AddFlags(fs)

	// Initialize tsmon flags.
	s.tsMonFlags = tsmon.NewFlags()
	s.tsMonFlags.Flush = tsmon.FlushAuto
	s.tsMonFlags.Target.TargetType = target.TaskType
	s.tsMonFlags.Target.TaskJobName = s.Name
	s.tsMonFlags.Register(fs)

	s.authFlags.Register(fs, auth.Options{})
	s.configFlags.AddToFlagSet(fs)

	fs.StringVar(&s.serviceID, "service-id", "",
		"Specify the service ID that this instance is supporting. If empty, the service ID "+
			"will attempt to be resolved by probing the local environment. This probably will match the "+
			"App ID of the Coordinator.")
	fs.StringVar(&s.coordinatorHost, "coordinator", "",
		"The Coordinator service's [host][:port].")
	fs.BoolVar(&s.coordinatorInsecure, "coordinator-insecure", false,
		"Connect to Coordinator over HTTP (instead of HTTPS).")
	fs.StringVar(&s.storageCredentialJSONPath, "storage-credential-json-path", "",
		"If supplied, the path of a JSON credential file to load and use for storage operations.")
	fs.StringVar(&s.cpuProfilePath, "cpu-profile-path", "",
		"If supplied, enable CPU profiling and write the profile here.")
	fs.StringVar(&s.heapProfilePath, "heap-profile-path", "",
		"If supplied, enable CPU profiling and write the profile here.")
}

// probeGCEEnvironment fills in any parameters that can be probed from Google
// Compute Engine metadata.
//
// If we're not running on GCE, this will return nil. An error will only be
// returned if an operation that is expected to work fails.
func (s *Service) probeGCEEnvironment(c context.Context) error {
	s.onGCE = metadata.OnGCE()
	if !s.onGCE {
		return nil
	}

	// Determine our service ID from metadata. The service ID will equal the cloud
	// project ID.
	if s.serviceID == "" {
		var err error
		if s.serviceID, err = metadata.ProjectID(); err != nil {
			log.WithError(err).Errorf(c, "Failed to probe GCE project ID.")
			return err
		}
	}
	return nil
}

func (s *Service) initCoordinatorClient(c context.Context) (logdog.ServicesClient, error) {
	if s.coordinatorHost == "" {
		log.Errorf(c, "Missing Coordinator URL (-coordinator).")
		return nil, ErrInvalidConfig
	}

	httpClient, err := s.AuthenticatedClient(c, func(o *auth.Options) {
		o.Scopes = CoordinatorScopes
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create authenticated client.")
		return nil, err
	}

	prpcClient := prpc.Client{
		C:       httpClient,
		Host:    s.coordinatorHost,
		Options: prpc.DefaultOptions(),
	}
	prpcClient.Options.UserAgent = fmt.Sprintf("%s/%s", s.Name, prpc.DefaultUserAgent)
	if s.coordinatorInsecure {
		prpcClient.Options.Insecure = true
	}
	sc := logdog.NewServicesPRPCClient(&prpcClient)

	// Wrap the resulting client in a retry harness.
	return retryServicesClient.New(sc, nil), nil
}

func (s *Service) initConfig(c context.Context) (*config.Manager, error) {
	rt, err := s.AuthenticatedTransport(c, nil)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create config client.")
		return nil, err
	}

	s.configFlags.RoundTripper = rt
	o, err := s.configFlags.CoordinatorOptions(c, s.coord)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to load configuration parameters.")
		return nil, err
	}
	o.ServiceID = s.serviceID
	o.ProjectConfigCacheDuration = projectConfigCacheDuration
	o.KillFunc = s.shutdown

	return config.NewManager(c, *o)
}

// SetShutdownFunc sets the service shutdown function.
func (s *Service) SetShutdownFunc(f func()) {
	s.shutdownFunc.Store(f)
}

func (s *Service) shutdown() {
	v := s.shutdownFunc.Load()
	if f, ok := v.(func()); ok {
		f()
	} else {
		s.shutdownImmediately()
	}
}

func (s *Service) shutdownImmediately() {
	os.Exit(1)
}

// Config returns the cached service configuration.
func (s *Service) Config() *svcconfig.Config {
	return s.config.Config()
}

// ProjectConfig returns the cached project configuration.
//
// If the project configuration is not available, nil will be returned.
func (s *Service) ProjectConfig(c context.Context, proj luciConfig.ProjectName) (*svcconfig.ProjectConfig, error) {
	return s.config.ProjectConfig(c, proj)
}

// Coordinator returns the cached Coordinator client.
func (s *Service) Coordinator() logdog.ServicesClient {
	return s.coord
}

// ServiceID returns the service ID.
//
// This is synonymous with the cloud "project ID" and the AppEngine "app ID".
func (s *Service) ServiceID() string {
	return s.serviceID
}

// IntermediateStorage instantiates the configured intermediate Storage
// instance.
func (s *Service) IntermediateStorage(c context.Context) (storage.Storage, error) {
	cfg := s.config.Config()
	if cfg.GetStorage() == nil {
		log.Errorf(c, "Missing storage configuration.")
		return nil, ErrInvalidConfig
	}

	btcfg := cfg.GetStorage().GetBigtable()
	if btcfg == nil {
		log.Errorf(c, "Missing BigTable storage configuration")
		return nil, ErrInvalidConfig
	}

	// Initialize Storage authentication.
	a, err := s.Authenticator(c, func(o *auth.Options) {
		o.Scopes = bigtable.StorageScopes
		if s.storageCredentialJSONPath != "" {
			o.ServiceAccountJSONPath = s.storageCredentialJSONPath
		}
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create BigTable Authenticator.")
		return nil, err
	}

	bt, err := bigtable.New(c, bigtable.Options{
		Project:  btcfg.Project,
		Instance: btcfg.Instance,
		LogTable: btcfg.LogTableName,
		ClientOptions: []option.ClientOption{
			option.WithTokenSource(a.TokenSource()),
		},
	})
	if err != nil {
		return nil, err
	}
	return bt, nil
}

// GSClient returns an authenticated Google Storage client instance.
func (s *Service) GSClient(c context.Context) (gs.Client, error) {
	rt, err := s.AuthenticatedTransport(c, func(o *auth.Options) {
		o.Scopes = gs.ReadWriteScopes
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create authenticated GS transport.")
		return nil, err
	}

	client, err := gs.NewProdClient(c, rt)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Google Storage client.")
		return nil, err
	}
	return client, nil
}

// Authenticator returns an Authenticator instance. The Authenticator is
// configured from a base set of Authenticator Options.
//
// An optional permutation function can be provided to modify those Options
// before the Authenticator is created.
func (s *Service) Authenticator(c context.Context, f func(o *auth.Options)) (*auth.Authenticator, error) {
	authOpts, err := s.authFlags.Options()
	if err != nil {
		return nil, ErrInvalidConfig
	}
	if f != nil {
		f(&authOpts)
	}
	return auth.NewAuthenticator(c, auth.SilentLogin, authOpts), nil
}

// AuthenticatedTransport returns an authenticated http.RoundTripper transport.
// The transport is configured from a base set of Authenticator Options.
//
// An optional permutation functon can be provided to modify those Options
// before the Authenticator is created.
func (s *Service) AuthenticatedTransport(c context.Context, f func(o *auth.Options)) (http.RoundTripper, error) {
	a, err := s.Authenticator(c, f)
	if err != nil {
		return nil, err
	}
	return a.Transport()
}

// AuthenticatedClient returns an authenticated http.Client. The Client is
// configured from a base set of Authenticator Options.
//
// An optional permutation functon can be provided to modify those Options
// before the Authenticator is created.
func (s *Service) AuthenticatedClient(c context.Context, f func(o *auth.Options)) (*http.Client, error) {
	a, err := s.Authenticator(c, f)
	if err != nil {
		return nil, err
	}
	return a.Client()
}
