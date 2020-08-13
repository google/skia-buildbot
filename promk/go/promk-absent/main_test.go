package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestEquationFromExpr_Equality(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("up == 0")
	assert.False(t, ignore)
	assert.Equal(t, "up", got)
}

func TestEquationFromExpr_LessThan(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("liveness_ci_pubsub_receive_s > 60 * 60 * 24 * 2")
	assert.False(t, ignore)
	assert.Equal(t, "liveness_ci_pubsub_receive_s", got)
}

func TestEquationFromExpr_LessThanOrEqual(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"} >= 10")
	assert.False(t, ignore)
	assert.Equal(t, "cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"}", got)
}

func TestEquationFromExpr_NotEqual(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("healthy{app=\"ct-perf\"} != 1")
	assert.False(t, ignore)
	assert.Equal(t, "healthy{app=\"ct-perf\"}", got)
}

func TestEquationFromExpr_Empty(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("")
	assert.False(t, ignore)
	assert.Equal(t, "", got)
}

func TestEquationFromExpr_IgnoreComputedEquations(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("computed:value")
	assert.True(t, ignore)
	assert.Equal(t, "", got)
}

func TestEquationFromExpr_IgnoreMultipleComparisons(t *testing.T) {
	unittest.SmallTest(t)

	got, ignore := equationFromExpr("a < b and b > c")
	assert.True(t, ignore)
	assert.Equal(t, "", got)
}
