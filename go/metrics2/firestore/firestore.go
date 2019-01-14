package firestore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	firestoreCollectionLiveness = "metrics-liveness"
	firestoreCollectionInt64    = "metrics-int64"
	firestoreCollectionFloat64  = "metrics-float64"
	firestoreNumAttempts        = 3
	firestoreTimeout            = time.Minute
)

// setFirestore sets the value for a metric in Firestore.
func setFirestore(doc *fs.DocumentRef, val interface{}) {
	_, err := firestore.Set(doc, val, firestoreNumAttempts, firestoreTimeout)
	if err != nil {
		sklog.Errorf("Failed to update metric in Firestore: %s", err)
	}
}

// deleteFirestore deletes the metric in Firestore.
func deleteFirestore(doc *fs.DocumentRef) error {
	_, err := firestore.Delete(doc, firestoreNumAttempts, firestoreTimeout)
	return err
}

// firestoreLiveness is a Liveness which is backed by Firestore.
type firestoreLiveness struct {
	metrics2.Liveness
	doc *fs.DocumentRef
}

// firestoreLivenessEntry represents the last-reset timestamp of a Liveness,
// stored in Firestore.
type firestoreLivenessEntry struct {
	LastReset time.Time `firestore:"lastReset"`
}

// See documentation for Liveness interface.
func (l *firestoreLiveness) Reset() {
	l.ManualReset(time.Now())
}

// See documentation for Liveness interface.
func (l *firestoreLiveness) ManualReset(lastSuccessfulUpdate time.Time) {
	l.Liveness.ManualReset(lastSuccessfulUpdate)
	setFirestore(l.doc, &firestoreLivenessEntry{lastSuccessfulUpdate})
}

// firestoreInt64Metric is an Int64Metric which is backed by Firestore.
type firestoreInt64Metric struct {
	metrics2.Int64Metric
	doc *fs.DocumentRef
}

// firestoreInt64MetricEntry represents the current value of an Int64Metric,
// stored in Firestore.
type firestoreInt64MetricEntry struct {
	Value int64 `firestore:"value"`
}

// See documentation for Int64Metric interface.
func (m *firestoreInt64Metric) Delete() error {
	if err := m.Int64Metric.Delete(); err != nil {
		return err
	}
	return deleteFirestore(m.doc)
}

// See documentation for Int64Metric interface.
func (m *firestoreInt64Metric) Update(v int64) {
	m.Int64Metric.Update(v)
	setFirestore(m.doc, &firestoreInt64MetricEntry{v})
}

// firestoreFloat64Metric is an Float64Metric which is backed by Firestore.
type firestoreFloat64Metric struct {
	metrics2.Float64Metric
	doc *fs.DocumentRef
}

// firestoreFloat64MetricEntry represents the current value of an Float64Metric,
// stored in Firestore.
type firestoreFloat64MetricEntry struct {
	Value float64 `firestore:"value"`
}

// See documentation for Float64Metric interface.
func (m *firestoreFloat64Metric) Delete() error {
	if err := m.Float64Metric.Delete(); err != nil {
		return err
	}
	return deleteFirestore(m.doc)
}

// See documentation for Float64Metric interface.
func (m *firestoreFloat64Metric) Update(v float64) {
	m.Float64Metric.Update(v)
	setFirestore(m.doc, &firestoreFloat64MetricEntry{v})
}

// firestoreCounter is a Counter which is backed by Firestore.
type firestoreCounter struct {
	metric metrics2.Int64Metric
	mutex  sync.Mutex
}

// See documentation for Counter interface.
func (c *firestoreCounter) Get() int64 {
	// Doesn't need to be locked: Get is atomic, and if Inc or Dec is called concurrently, either old
	// or new value is fine.
	return c.metric.Get()
}

// See documentation for Counter interface.
func (c *firestoreCounter) Inc(i int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metric.Update(c.metric.Get() + i)
}

// See documentation for Counter interface.
func (c *firestoreCounter) Dec(i int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metric.Update(c.metric.Get() - i)
}

// See documentation for Counter interface.
func (c *firestoreCounter) Reset() {
	// Needs a lock to avoid race with Inc/Dec.
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metric.Update(0)
}

// See documentation for Counter interface.
func (c *firestoreCounter) Delete() error {
	return c.metric.Delete()
}

// NewFirestoreClient creates a Client which is backed by Firestore, to provide
// persistence across restarts. It is important to differentiate multiple
// instances of the same app, to ensure that they don't clobber the metric value
// in Firestore. Do this either with the instance parameter to
// NewFirestoreClient, or with tags for individual metrics.
func NewFirestoreClient(ctx context.Context, parent metrics2.Client, project, app, instance string, ts oauth2.TokenSource) (metrics2.Client, error) {
	c, err := firestore.NewClient(ctx, project, app, instance, ts)
	if err != nil {
		return nil, err
	}
	return &firestoreClient{
		metricsClient: parent,
		fsClient:      c,
	}, nil
}

// firestoreClient is a Client which is backed by Firestore.
type firestoreClient struct {
	metricsClient metrics2.Client
	fsClient      *firestore.Client
}

// makeFirestoreDocumentID creates a Firestore document ID using the given
// metric name and tags.
func makeFirestoreDocumentID(name string, tags []map[string]string) string {
	mergedTags := map[string]string{}
	for _, m := range tags {
		for k, v := range m {
			mergedTags[k] = v
		}
	}
	tagStrs := make([]string, 0, len(mergedTags))
	for k, v := range mergedTags {
		tagStrs = append(tagStrs, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("%s_%s", name, strings.Join(tagStrs, ","))
}

// See documentation for Client interface.
func (c *firestoreClient) Flush() error {
	// TODO(borenet): We could make all Firestore operations asynchronous
	// and have Flush() collect any errors.
	return c.metricsClient.Flush()
}

// errMsg wraps the given error, if any, with a message indicating that we're
// falling back to local, non-Firestore metrics.
func errMsg(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("Failed to create metric in Firestore. Falling back to local metric. Error: %s", err)
}

// newMetric loads the initial value for the given metric into the given object,
// and returns the Firestore DocumentRef.
func (c *firestoreClient) newMetric(coll string, obj interface{}, name string, tagsList []map[string]string) (*fs.DocumentRef, error) {
	id := makeFirestoreDocumentID(name, tagsList)
	doc := c.fsClient.Collection(coll).Doc(id)

	// TODO(borenet): Should the below be done in a transaction?
	snap, err := firestore.Get(doc, firestoreNumAttempts, firestoreTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		_, err := firestore.Set(doc, obj, firestoreNumAttempts, firestoreTimeout)
		return doc, errMsg(err)
	} else if err != nil {
		return nil, errMsg(err)
	}
	return doc, errMsg(snap.DataTo(&obj))
}

// See documentation for Client interface.
func (c *firestoreClient) NewLiveness(name string, tagsList ...map[string]string) metrics2.Liveness {
	e := firestoreLivenessEntry{
		LastReset: time.Now(),
	}
	m := c.metricsClient.NewLiveness(name, tagsList...)
	doc, err := c.newMetric(firestoreCollectionLiveness, &e, name, tagsList)
	if err != nil {
		sklog.Error(err)
		return m
	}
	m.ManualReset(e.LastReset)
	return &firestoreLiveness{
		Liveness: m,
		doc:      doc,
	}
}

// See documentation for Client interface.
func (c *firestoreClient) GetInt64Metric(name string, tagsList ...map[string]string) metrics2.Int64Metric {
	e := firestoreInt64MetricEntry{
		Value: 0,
	}
	m := c.metricsClient.GetInt64Metric(name, tagsList...)
	doc, err := c.newMetric(firestoreCollectionInt64, &e, name, tagsList)
	if err != nil {
		sklog.Error(err)
		return m
	}
	m.Update(e.Value)
	return &firestoreInt64Metric{
		Int64Metric: m,
		doc:         doc,
	}
}

// See documentation for Client interface.
func (c *firestoreClient) GetFloat64Metric(name string, tagsList ...map[string]string) metrics2.Float64Metric {
	e := firestoreFloat64MetricEntry{
		Value: 0,
	}
	m := c.metricsClient.GetFloat64Metric(name, tagsList...)
	doc, err := c.newMetric(firestoreCollectionFloat64, &e, name, tagsList)
	if err != nil {
		sklog.Error(err)
		return m
	}
	m.Update(e.Value)
	return &firestoreFloat64Metric{
		Float64Metric: m,
		doc:           doc,
	}
}

// See documentation for Client interface.
func (c *firestoreClient) GetCounter(name string, tagsList ...map[string]string) metrics2.Counter {
	m := c.GetInt64Metric(name, tagsList...)
	return &firestoreCounter{
		metric: m,
	}
}

// See documentation for Client interface.
func (c *firestoreClient) GetFloat64SummaryMetric(name string, tagsList ...map[string]string) metrics2.Float64SummaryMetric {
	// GetFloat64SummaryMetric is not implemented for firestoreClient
	// because there isn't a good way to restore the values from Firestore
	// with the correct timestamps.
	sklog.Error("GetFloat64SummaryMetric is unimplemented for firestoreClient. Falling back to local metrics.")
	return c.metricsClient.GetFloat64SummaryMetric(name, tagsList...)
}

// See documentation for Client interface.
func (c *firestoreClient) NewTimer(name string, tagsList ...map[string]string) metrics2.Timer {
	return metrics2.NewTimerHelper(c, name, true, tagsList...)
}

var _ metrics2.Client = &firestoreClient{}
var _ metrics2.Liveness = &firestoreLiveness{}
var _ metrics2.Int64Metric = &firestoreInt64Metric{}
var _ metrics2.Float64Metric = &firestoreFloat64Metric{}
var _ metrics2.Counter = &firestoreCounter{}
