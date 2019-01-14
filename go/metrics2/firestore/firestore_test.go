package firestore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
)

func TestFirestoreMetrics(t *testing.T) {
	testutils.LargeTest(t)

	ctx := context.Background()
	id1 := uuid.New().String()
	c1, err := NewFirestoreClient(ctx, metrics2.NewPromClient(), firestore.FIRESTORE_PROJECT, "firestore-metrics-test", id1, nil)
	assert.NoError(t, err)
	//c2, err := NewFirestoreClient(ctx, metrics2.NewPromClient(), firestore.FIRESTORE_PROJECT, "firestore-metrics-test", id1, nil)
	//assert.NoError(t, err)

	n1 := "my-metric"
	t1 := map[string]string{
		"my-tag-key": "my-tag-value",
	}
	m1 := c1.GetInt64Metric(n1, t1)
	m1.Update(42)
	// TODO(borenet): This gives "duplicate metrics collector registration attempted".
	// How can we test this?
	//assert.Equal(t, m1.Get(), c2.GetInt64Metric(n1, t1).Get())
}
