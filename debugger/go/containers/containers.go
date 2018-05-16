// package containers provides for running a bunch of skiaserve instances in containers.
package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"go.skia.org/infra/debugger/go/runner"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	// MAX_CONTAINERS is the max number of concurrent skiaserve instances we
	// support in the hosted environment.
	MAX_CONTAINERS = 200

	// START_PORT Is the beginning of the range of ports the skiaserve instances
	// will communicate on.
	START_PORT = 30000

	// START_WAIT_NUM poll the newly started skiaserve this many times before giving up.
	START_WAIT_NUM = 50

	// START_WAIT_PERIOD poll the newly started skiaserve this often.
	START_WAIT_PERIOD = 100 * time.Millisecond

	// EXIT_WAIT_PERIOD is the time to wait for the container to exit.
	EXIT_WAIT_PERIOD = 2 * time.Second
)

var (
	// Strip out the container uuid.
	containerPrefixRe = regexp.MustCompile("^/([A-Fa-f0-9])(/.*)")
)

// container represents a single skiaserve instance, which may or may not
// be running. It is used in containers.
type container struct {
	// proxy is the proxy connection to talk to the running skiaserve.
	proxy *httputil.ReverseProxy

	// port is the port that skiaserve is listening on.
	port int

	// uuid is the login id of the uuid this skiaserve is running for.
	uuid string // "" means this isn't running.

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

	// containers is a map from uuid to a container running skiaserve.
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
			sklog.Errorf("failed to parse url %q: %s", proxyurl, err)
		}
		c := &container{
			port:  port,
			proxy: httputil.NewSingleHostReverseProxy(u),
		}
		s.pool = append(s.pool, c)
	}
	return s
}

// startContainer starts skiaserve running in a container for the given uuid.
//
// It waits until skiaserve responds to an HTTP request before returning.
//
// The actual instance for the uuid is determined by looking at the prefix of
// the URL.Path, i.e. /2..F/foo will be directed to instance 2..F for the given uuid
// and skiaserve will be sent the URL.Path "/foo", i.e. with the instance
// number prefix stripped.
func (s *Containers) startContainer(ctx context.Context, uuid string) error {
	s.mutex.Lock()
	// Find first open container in the pool.
	var co *container = nil
	for _, c := range s.pool {
		if c.uuid == "" {
			c.uuid = uuid
			co = c
			break
		}
	}
	if co != nil {
		s.containers[uuid] = co
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
		counter := metrics2.GetCounter("running_instances", nil)
		counter.Inc(1)
		co.started = time.Now()
		// This call to s.runner.Start() doesn't return until the container exits.
		if err := s.runner.Start(ctx, co.port); err != nil {
			sklog.Errorf("Failed to start container at port %d: %s", co.port, err)
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		// Remove the entry for this container now that it has exited.
		delete(s.containers, uuid)
		counter.Dec(1)
		co.uuid = ""
	}()

	// Poll the port until we get a response.
	url := fmt.Sprintf("http://localhost:%d", co.port)
	var err error
	var resp *http.Response
	client := httputils.NewTimeoutClient()
	for i := 0; i < START_WAIT_NUM; i++ {
		resp, err = client.Get(url)
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				sklog.Errorf("Failed to close response while listing for skiaserve to start: %s", err)
			}
		}
		if err == nil {
			break
		}
		time.Sleep(START_WAIT_PERIOD)
	}
	if err != nil {
		return fmt.Errorf("Started container but skiaserve never responded: %s", err)
	}

	return nil
}

// getContainer returns the Container for the given uuid, or nil if there isn't
// one for that uuid.
func (s *Containers) getContainer(uuid string) *container {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.containers[uuid]
}

// setLastUsed set the lastUsed timestamp for a Container.
func (s *Containers) setLastUsed(uuid string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.containers[uuid].lastUsed = time.Now()
}

// ServeHTTP implements the http.Handler interface by proxying the requests to
// the correct Container based on the uuid.
func (s *Containers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	containerID := ""
	// Strip off the uuid from the URL.
	parts := containerPrefixRe.FindStringSubmatch(r.URL.Path)
	if len(parts) == 3 {
		containerID = parts[1]
		r.URL.Path = parts[2]
	} else {
		httputils.ReportError(w, r, fmt.Errorf("Invalid URL %q", r.URL.Path), "Not a valid URL.")
		return
	}

	co := s.getContainer(containerID)
	if co == nil {
		// If no container then start one up.
		if err := s.startContainer(ctx, containerID); err != nil {
			httputils.ReportError(w, r, err, "Failed to start new container.")
			return
		}
		co = s.getContainer(containerID)
		if co == nil {
			httputils.ReportError(w, r, fmt.Errorf("Failed to start container %q", containerID), "Started container, but then couldn't find it.")
			return
		}
	}

	// Mostly we proxy requests to the backend, but there is a URL we handle here: /instanceStatus
	//
	if r.URL.Path == "/instanceStatus" {
		if r.Method == "GET" {
			// A GET to /instanceStatus will return the instance info, i.e. how long it's been running.
			enc := json.NewEncoder(w)
			if err := enc.Encode(
				struct {
					Started int64 `json:"started"`
				}{
					Started: co.started.Unix(),
				},
			); err != nil {
				httputils.ReportError(w, r, err, "Failed to serialize response.")
			}
		} else if r.Method == "POST" {
			// A POST to /instanceStatus will restart the instance.
			runner.Stop(co.port)
			time.Sleep(EXIT_WAIT_PERIOD)
			s.mutex.Lock()
			defer s.mutex.Unlock()
			// Remove the entry for this container now that it has exited.
			delete(s.containers, containerID)
			http.Redirect(w, r, "/", 303)
		}
		return
	}

	// Mostly we proxy requests to the backend, but there is a URL we handle here: /instanceNew
	//
	if r.URL.Path == "/instanceNew" && r.Method == "GET" {
		http.Redirect(w, r, fmt.Sprintf("/%x/", uuid.NewRandom()), 303)
	}

	// Proxy.
	sklog.Infof("Proxying request: %s %s", r.URL, containerID)
	// If the request is a POST and we are at a non-zero instanceNum then pass in
	// a recording response.  If the response is a 303 then we return a 303 with
	// the correct location URL, otherwise we return the response verbatim.
	if r.Method == "POST" {
		rw := httptest.NewRecorder()
		co.proxy.ServeHTTP(rw, r)
		if rw.Code == 303 {
			http.Redirect(w, r, fmt.Sprintf("/%s/", containerID), 303)
		} else {
			for k, values := range rw.HeaderMap {
				for _, v := range values {
					w.Header().Set(k, v)
				}
			}
			if _, err := w.Write(rw.Body.Bytes()); err != nil {
				sklog.Errorf("Failed proxying a recorded response: %s", err)
			}
		}
	} else {
		co.proxy.ServeHTTP(w, r)
	}

	// Update lastUsed.
	s.setLastUsed(containerID)
}

// StopAll stops all running containers.
func (s *Containers) StopAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, co := range s.containers {
		sklog.Infof("Stopping container for uuid %q on port %d", co.uuid, co.port)
		runner.Stop(co.port)
	}
}

type ContainerInfo struct {
	ID     string        `json:"id"`
	UUID   string        `json:"uuid"`
	Uptime time.Duration `json:"uptime"`
	Port   int           `json:"port"`
}

type ContainerInfoSlice []*ContainerInfo

func (p ContainerInfoSlice) Len() int           { return len(p) }
func (p ContainerInfoSlice) Less(i, j int) bool { return p[i].ID < p[j].ID }
func (p ContainerInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (s *Containers) DescribeAll() []*ContainerInfo {
	info := []*ContainerInfo{}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for id, co := range s.containers {
		info = append(info, &ContainerInfo{
			ID:     id,
			UUID:   co.uuid,
			Uptime: time.Now().Sub(co.started),
			Port:   co.port,
		})
	}
	sort.Sort(ContainerInfoSlice(info))
	return info
}
