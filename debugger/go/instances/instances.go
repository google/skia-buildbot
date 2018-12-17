// instances provides for running a bunch of skiaserve instances.
package instances

import (
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

	"github.com/google/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// MAX_INSTANCES is the max number of concurrent skiaserve instances we
	// support in the hosted environment.
	MAX_INSTANCES = 200

	// MIN_POOL_SIZE is the number of available spots in the pool that should be
	// maintained at all times. Instances will be culled by oldest lastUsed time
	// until this size is reached.
	MIN_POOL_SIZE = 10

	// START_PORT Is the beginning of the range of ports the skiaserve instances
	// will communicate on.
	START_PORT = 30000

	// START_WAIT_NUM poll the newly started skiaserve this many times before giving up.
	START_WAIT_NUM = 50

	// START_WAIT_PERIOD poll the newly started skiaserve this often.
	START_WAIT_PERIOD = 100 * time.Millisecond

	// EXIT_WAIT_PERIOD is the time to wait for the instance to exit.
	EXIT_WAIT_PERIOD = 2 * time.Second

	// SKIASERVE is the full path to the skiaserve executable.
	SKIASERVE = "/usr/local/bin/skiaserve"
)

var (
	// instancePrefixRe is used to strip out the instance uuid.
	instancePrefixRe = regexp.MustCompile("^/([A-Fa-f0-9-]+)(/.*)")

	runningInstances = metrics2.GetCounter("running_instances", nil)
)

// NewInstanceID creates a new id for an instance of skiaserve.
func NewInstanceID() string {
	return uuid.New().String()
}

// instance represents a single skiaserve instance, which may or may not
// be running. It is used in Instances.
type instance struct {
	// proxy is the proxy connection to talk to the running skiaserve.
	proxy *httputil.ReverseProxy

	// source is the schema and domain from where to load assets, e.g. "https://debugger.skia.org".
	source string

	// port is the port that skiaserve is listening on.
	port int

	display int

	// uuid is the login id of the uuid this skiaserve is running for.
	uuid string // "" means this isn't running.

	// lastUsed is the time the skiaserve instance last processed a request.
	lastUsed time.Time

	// started is the time that the skiaserve instance was started. Will be used
	// later when we give hosted users the ability to see if their skiaserve is
	// out of date.
	started time.Time

	process exec.Process
}

// Start a single instance of skiaserve running at the given port.
//
func (c *instance) Start(uuid string) (<-chan error, error) {
	runCmd := &exec.Command{
		Name:      "xvfb-run",
		Args:      []string{"--server-args", "-screen 0 1280x1024x24", "--server-num", fmt.Sprintf("%d", c.port), SKIASERVE, "--port", fmt.Sprintf("%d", c.port), "--source", c.source, "--hosted"},
		Env:       []string{fmt.Sprintf("DISPLAY=:%d", c.display)},
		LogStdout: true,
	}
	process, exitChan, err := exec.RunIndefinitely(runCmd)
	if err == nil {
		c.process = process
		c.uuid = uuid
	}
	return exitChan, err
}

// Stop the instance from running.
func (c *instance) Stop() {
	if err := c.process.Kill(); err != nil {
		sklog.Errorf("Error trying to kill instance: %s", err)
	}
}

// Sort so oldest with a non-empty uuid are first.
type instanceSlice []*instance

func (p instanceSlice) Len() int { return len(p) }
func (p instanceSlice) Less(i, j int) bool {
	// if p[i].uuid == "" XOR p[j].uuid == ""
	if (p[i].uuid == "") != (p[j].uuid == "") {
		return p[i].uuid > p[j].uuid
	} else {
		return p[i].lastUsed.Before(p[j].lastUsed)
	}
}
func (p instanceSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Instances is used to control a number of skiaserve instances all running
// at the same time.
//
// Instances implements http.handler, which reverse proxies incoming requests
// to the right backend.
//
// TODO(jcgregorio) Need to add memory limits to instance.
type Instances struct {
	// pool is the list of potential running skiaserve instances. We only start
	// them on demand.
	pool []*instance

	// instances is a map from uuid to a running skiaserve.
	instances map[string]*instance

	// mutex protects access to pool and instances.
	mutex sync.Mutex
}

func New(source string) *Instances {
	s := &Instances{
		pool:      []*instance{},
		instances: map[string]*instance{},
	}
	for i := 0; i < MAX_INSTANCES; i++ {
		port := START_PORT + i
		proxyurl := fmt.Sprintf("http://localhost:%d", port)
		u, err := url.Parse(proxyurl)
		if err != nil {
			sklog.Errorf("failed to parse url %q: %s", proxyurl, err)
		}
		c := &instance{
			port:    port,
			display: i + 100,
			proxy:   httputil.NewSingleHostReverseProxy(u),
			source:  source,
		}
		s.pool = append(s.pool, c)
	}

	// Start a culling process that runs every minute that keeps
	// some open spots in the pool.
	go func() {
		for range time.Tick(time.Minute) {
			s.cull()
		}
	}()

	return s
}

// cull terminates the oldest running instances to make room for more.
func (s *Instances) cull() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	sort.Sort(instanceSlice(s.pool))
	cull := s.pool[len(s.pool)-MIN_POOL_SIZE:]
	for _, co := range cull {
		if co.uuid != "" {
			co.Stop()
		}
	}
}

// getInstanceFromPool finds the first open instance in the pool.
func (s *Instances) getInstanceFromPool(uuid string) *instance {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// Find first open instance in the pool.
	var co *instance = nil
	for _, c := range s.pool {
		if c.uuid == "" {
			c.uuid = uuid
			co = c
			break
		}
	}
	if co != nil {
		s.instances[uuid] = co
	}
	return co
}

// startInstance starts skiaserve running in a instance for the given uuid.
//
// It waits until skiaserve responds to an HTTP request before returning.
//
// The actual instance for the uuid is determined by looking at the prefix of
// the URL.Path, i.e. /2..F/foo will be directed to instance 2..F for the given uuid
// and skiaserve will be sent the URL.Path "/foo", i.e. with the instance
// number prefix stripped.
func (s *Instances) startInstance(uuid string) error {
	co := s.getInstanceFromPool(uuid)
	if co == nil {
		return fmt.Errorf("Could not start an instance.")
	}
	runningInstances := metrics2.GetCounter("running_instances", nil)
	runningInstances.Inc(1)
	co.started = time.Now()
	exitChan, err := co.Start(uuid)
	if err != nil {
		return fmt.Errorf("Failed to start instance at port %d: %s", co.port, err)
	}

	go func() {
		<-exitChan
		// Remove the entry for this instance now that it has exited.
		s.mutex.Lock()
		defer s.mutex.Unlock()
		delete(s.instances, uuid)
		runningInstances.Dec(1)
		co.uuid = ""
	}()

	// Poll the port until we get a response.
	url := fmt.Sprintf("http://localhost:%d", co.port)
	var resp *http.Response
	client := httputils.NewTimeoutClient()
	for i := 0; i < START_WAIT_NUM; i++ {
		resp, err = client.Get(url)
		if resp != nil && resp.Body != nil {
			util.Close(resp.Body)
		}
		if err == nil {
			break
		}
		time.Sleep(START_WAIT_PERIOD)
	}
	if err != nil {
		return fmt.Errorf("Started instance but skiaserve never responded: %s", err)
	}

	return nil
}

// getInstance returns the instance for the given uuid, or nil if there isn't
// one for that uuid.
func (s *Instances) getInstance(uuid string) *instance {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.instances[uuid]
}

// setLastUsed set the lastUsed timestamp for an instance.
func (s *Instances) setLastUsed(uuid string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.instances[uuid].lastUsed = time.Now()
}

// ServeHTTP implements the http.Handler interface by proxying the requests to
// the correct instance based on the uuid.
//
// All requests are routed to the instance, with the exception of
// /instanceStatus and /instanceNew which will be handled by 'co' itself.
func (s *Instances) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/instanceNew" && r.Method == "POST" {
		// TODO(jcgregorio) Add gorilla.csrf protection.
		http.Redirect(w, r, fmt.Sprintf("/%s/", NewInstanceID()), 303)
		return
	}
	instanceID := ""
	// Strip off the uuid from the URL.
	parts := instancePrefixRe.FindStringSubmatch(r.URL.Path)
	sklog.Infof("Parts: %v", parts)
	if len(parts) == 3 {
		instanceID = parts[1]
		r.URL.Path = parts[2]
	} else {
		httputils.ReportError(w, r, fmt.Errorf("Invalid URL %q", r.URL.Path), "Not a valid URL.")
		return
	}

	co := s.getInstance(instanceID)
	if co == nil {
		// If no instance then start one up.
		if err := s.startInstance(instanceID); err != nil {
			httputils.ReportError(w, r, err, "Failed to start new instance.")
			return
		}
		co = s.getInstance(instanceID)
		if co == nil {
			httputils.ReportError(w, r, fmt.Errorf("Failed to start instance %q", instanceID), "Started instance, but then couldn't find it.")
			return
		}
	}

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
			return
		} else if r.Method == "POST" {
			co.Stop()
			time.Sleep(EXIT_WAIT_PERIOD)
			s.mutex.Lock()
			defer s.mutex.Unlock()
			// Remove the entry for this instance now that it has exited.
			delete(s.instances, instanceID)
			http.Redirect(w, r, fmt.Sprintf("/%s/", instanceID), 303)
		}
		return
	}

	// Proxy.
	sklog.Infof("Proxying request: %s %s", r.URL, instanceID)
	// If the request is a POST and we are at a non-zero instanceNum then pass in
	// a recording response.  If the response is a 303 then we return a 303 with
	// the correct location URL, otherwise we return the response verbatim.
	if r.Method == "POST" {
		// TODO(jcgregorio) If this is an uploaded SKP then store it in GCS
		// to make restarts less painful.
		rw := httptest.NewRecorder()
		co.proxy.ServeHTTP(rw, r)
		if rw.Code == 303 {
			http.Redirect(w, r, fmt.Sprintf("/%s/", instanceID), 303)
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
	s.setLastUsed(instanceID)
}

// StopAll stops all running instances.
func (s *Instances) StopAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, co := range s.instances {
		sklog.Infof("Stopping instance for uuid %q on port %d", co.uuid, co.port)
		co.Stop()
	}
}

type InstanceInfo struct {
	ID     string        `json:"id"`
	UUID   string        `json:"uuid"`
	Uptime time.Duration `json:"uptime"`
	Port   int           `json:"port"`
}

type InstanceInfoSlice []*InstanceInfo

func (p InstanceInfoSlice) Len() int           { return len(p) }
func (p InstanceInfoSlice) Less(i, j int) bool { return p[i].ID < p[j].ID }
func (p InstanceInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (s *Instances) DescribeAll() []*InstanceInfo {
	info := []*InstanceInfo{}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for id, co := range s.instances {
		info = append(info, &InstanceInfo{
			ID:     id,
			UUID:   co.uuid,
			Uptime: time.Now().Sub(co.started),
			Port:   co.port,
		})
	}
	sort.Sort(InstanceInfoSlice(info))
	return info
}
