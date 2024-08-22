package coverage

import (
	"context"
	"net"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/coverage/config"
	"go.skia.org/infra/go/coverage/coveragestore"
	coverage_store "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore"
	coverage_service "go.skia.org/infra/go/coverage/service"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/pool/wrapper/timeout"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const appName = "coverage"

// Coverage provides a struct for the application.
type Coverage struct {
	grpcServer     *grpc.Server
	lisGRPC        net.Listener
	coverageConfig *config.CoverageConfig
	promPort       string
}

// pgxLogAdaptor allows bubbling pgx logs up into our application.
type pgxLogAdaptor struct{}

// Log a message at the given level with data key/value pairs. data may be nil.
func (pgxLogAdaptor) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	switch level {
	case pgx.LogLevelTrace:
	case pgx.LogLevelDebug:
	case pgx.LogLevelInfo:
	case pgx.LogLevelWarn:
		sklog.Warningf("pgx - %s %v", msg, data)
	case pgx.LogLevelError:
		sklog.Warningf("pgx - %s %v", msg, data)
	case pgx.LogLevelNone:
	}
}

// maxPoolConnections is the MaxConns our pgxPool will maintain.
const maxPoolConnections = 300

// singletonPool is the one and only instance of pool.Pool that an
// application should have, used in NewCockroachDBFromConfig.
var singletonPool pool.Pool

// singletonPoolMutex is used to enforce the singleton nature of singletonPool,
// used in NewCockroachDBFromConfig
var singletonPoolMutex sync.Mutex

// CoverageService provides an interface for a service to be hosted on Coverage application.
type CoverageService interface {
	// RegisterGrpc registers the grpc service with the server instance.
	RegisterGrpc(server *grpc.Server)
	// GetServiceDescriptor returns the service descriptor for the service.
	GetServiceDescriptor() grpc.ServiceDesc
}

// initialize initializes the Coverage application.
func (c *Coverage) initialize(coverageStore coveragestore.Store) error {

	ctx := context.Background()

	// Use config file to load vales
	config, err := c.coverageConfig.LoadCoverageConfig(c.coverageConfig.ConfigFilename)
	if err != nil || config == nil {
		sklog.Fatal(err)
	}

	common.InitWithMust(
		appName,
		common.PrometheusOpt(&config.PromPort),
	)

	if coverageStore == nil {
		var err error
		coverageStore, err = NewCoverageStoreFromConfig(ctx, c.coverageConfig)
		if err != nil {
			sklog.Errorf("Error creating coverage store. %s", err)
			return err
		}
	}
	c.grpcServer = grpc.NewServer()
	reflection.Register(c.grpcServer)

	services := []CoverageService{
		coverage_service.New(coverageStore),
	}
	c.registerServices(services)

	c.lisGRPC, _ = net.Listen("tcp4", ":"+config.ServicePort)
	sklog.Info("Coverage server listening at ", c.lisGRPC.Addr())

	cleanup.AtExit(c.Cleanup)
	return nil
}

// NewCockroachDBFromConfig opens an existing CockroachDB database.
func NewCockroachDBFromConfig(ctx context.Context, coverageConfig *config.CoverageConfig) (pool.Pool, error) {
	singletonPoolMutex.Lock()
	defer singletonPoolMutex.Unlock()

	if singletonPool != nil {
		return singletonPool, nil
	}

	cfg, err := pgxpool.ParseConfig(coverageConfig.GetConnectionString())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse database config: %q", coverageConfig.GetConnectionString())
	}

	cfg.MaxConns = maxPoolConnections
	cfg.ConnConfig.Logger = pgxLogAdaptor{}
	rawPool, err := pgxpool.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Wrap the db pool in a ContentTimeout which checks that every context has
	// a timeout.
	singletonPool = timeout.New(rawPool)

	return singletonPool, err
}

// NewCoverageStoreFromConfig creates a new coverage.Store from the
// CoverageConfig which provides access to the coverage data.
func NewCoverageStoreFromConfig(ctx context.Context, coverageConfig *config.CoverageConfig) (coveragestore.Store, error) {
	db, err := NewCockroachDBFromConfig(ctx, coverageConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return coverage_store.New(db)
}

// registerServices registers all available services for Coverage.
func (c *Coverage) registerServices(services []CoverageService) {
	for _, service := range services {
		service.RegisterGrpc(c.grpcServer)
	}

}

// ServeGRPC does not return unless there is an error during the startup process, in which case it
// returns the error, or if a call to [Cleanup()] causes a graceful shutdown, in which
// case it returns either nil if the graceful shutdown succeeds, or an error if it does not.
func (c *Coverage) ServeGRPC() error {
	if err := c.grpcServer.Serve(c.lisGRPC); err != nil {
		sklog.Errorf("failed to serve grpc: %v", err)
		return err
	}
	return nil
}

// New creates a new instance of Coverage application.
func New(config *config.CoverageConfig,
	coverageStore coveragestore.Store) (*Coverage, error) {
	c := &Coverage{
		coverageConfig: config,
		promPort:       config.PromPort,
	}
	err := c.initialize(coverageStore)
	return c, err
}

// Cleanup performs a graceful shutdown of the grpc server.
func (c *Coverage) Cleanup() {
	sklog.Info("Shutdown server gracefully.")
	if c.grpcServer != nil {
		c.grpcServer.GracefulStop()
	}
}

// Serve intiates the listener to serve traffic.
func (c *Coverage) Serve() {
	sklog.Info("Starting Server...")
	if err := c.ServeGRPC(); err != nil {
		sklog.Fatal(err)
	}
}
