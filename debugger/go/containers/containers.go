// package containers provides for running a bunch of skiaserve instances in containers.
package containers

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/debugger/go/runner"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
)

const (
	// MAX_CONTAINERS is the max number of concurrent skiaserve instances we
	// support in the hosted environment.
	MAX_CONTAINERS = 200

	// START_PORT Is the beginning of the range of ports the skiaserve instances
	// will communicate on.
	START_PORT = 20000
)

// container represents a single skiaserve instance, which may or may not
// be running. It is used in containers.
type container struct {
	// proxy is the proxy connection to talk to the running skiaserve.
	proxy *httputil.ReverseProxy

	// port is the port that skiaserve is listening on.
	port int

	// user is the login id of the user this skiaserve is running for.
	user string // "" means this isn't running.

	// lastUsed is the time the skiaserve instance last processed a request.
	lastUsed time.Time

	// started is the time that the skiaserve instance was started. Will be used
	// later when we give hosted users the ability to see if their skiaserve is
	// out of date.
	started time.Time
}

// Containers is used to control a number of skiaserve instances all running
// at the same time.
//
// Containers implements http.handler, which reverse proxies incoming requests
// to the right backend.
//
// TODO(jcgregorio) Need to add memory limits to container.
type Containers struct {
	// pool is the list of potential running skiaserve instances. We only start
	// them on demand.
	pool []*container

	// containers is a map from userid to a container running skiaserve.
	containers map[string]*container

	// runner is used to start skiaserve instances running.
	runner *runner.Runner

	// mutex protects access to pool and containers.
	mutex sync.Mutex
}

// New creates a new containers from the given runner.
func New(runner *runner.Runner) *Containers {
	s := &Containers{
		pool:       []*container{},
		containers: map[string]*container{},
		runner:     runner,
	}
	for i := 0; i < MAX_CONTAINERS; i++ {
		port := START_PORT + i
		proxyurl := fmt.Sprintf("http://localhost:%d", port)
		u, err := url.Parse(proxyurl)
		if err != nil {
			glog.Errorf("failed to parse url %q: %s", proxyurl, err)
		}
		c := &container{
			port:  port,
			proxy: httputil.NewSingleHostReverseProxy(u),
		}
		s.pool = append(s.pool, c)
	}
	return s
}

// startContainer starts skiaserve running in a container for the given user.
func (s *Containers) startContainer(user string) error {
	s.mutex.Lock()
	// Find first open container in the pool.
	var co *container = nil
	for _, c := range s.pool {
		if c.user == "" {
			c.user = user
			co = c
			break
		}
	}
	if co != nil {
		s.containers[user] = co
	}
	s.mutex.Unlock()
	if co == nil {
		// TODO(jcgregorio) Implement killing old containers to make room
		// for the new container.
		return fmt.Errorf("Could not find an open container.")
	}
	// Kick off a Go routine that calls runner.Start and then removes the
	// container from s.containers once skiaserve exits.
	go func() {
		co.started = time.Now()
		// This call to s.runner.Start() doesn't return until the container exits.
		if err := s.runner.Start(co.port); err != nil {
			glog.Errorf("Failed to start container at port %d: %s", co.port, err)
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		// Remove the entry for this container now that it has exited.
		delete(s.containers, user)
		co.user = ""
	}()

	return nil
}

// getContainer returns the Container for the given user, or nil if there isn't
// one for that user.
func (s *Containers) getContainer(user string) *container {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.containers[user]
}

// setLastUsed set the lastUsed timestamp for a Container.
func (s *Containers) setLastUsed(user string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.containers[user].lastUsed = time.Now()
}

// ServeHTTP implements the http.Handler interface by proxying the requests to
// the correct Container based on the user id.
func (s *Containers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Look up user.
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "Unauthorized", 503)
		return
	}

	// From user look up container.
	co := s.getContainer(user)
	if co == nil {
		// If no container then start one up.
		if err := s.startContainer(user); err != nil {
			httputils.ReportError(w, r, err, "Failed to start new container.")
			return
		}
		// Wait for skiaserve to start.
		// TODO(jcgregorio) We should actually poll the port and confirm the instance is running.
		time.Sleep(time.Second)
		co = s.getContainer(user)
		if co == nil {
			httputils.ReportError(w, r, fmt.Errorf("For user: %s", user), "Started container, but then couldn't find it.")
			return
		}
	}
	// Proxy.
	glog.Infof("Proxying request: %s %s", r.URL, user)
	co.proxy.ServeHTTP(w, r)
	// Update lastUsed.
	co.lastUsed = time.Now()
}

// StopAll stops all running containers.
func (s *Containers) StopAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, co := range s.containers {
		glog.Infof("Stopping container for user %q on port %d", co.user, co.port)
		runner.Stop(co.port)
	}
}
