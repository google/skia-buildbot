package prom

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEquationFromExpr_Equality(t *testing.T) {

	got, ignore := EquationFromExpr("up == 0")
	assert.False(t, ignore)
	assert.Equal(t, "up", got)
}

func TestEquationFromExpr_NoOperations(t *testing.T) {

	got, ignore := EquationFromExpr("vector(1)")
	assert.True(t, ignore)
	assert.Equal(t, "", got)
}

func TestEquationFromExpr_LessThan(t *testing.T) {

	got, ignore := EquationFromExpr("liveness_ci_pubsub_receive_s > 60 * 60 * 24 * 2")
	assert.False(t, ignore)
	assert.Equal(t, "liveness_ci_pubsub_receive_s", got)
}

func TestEquationFromExpr_LessThanOrEqual(t *testing.T) {

	got, ignore := EquationFromExpr("cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"} >= 10")
	assert.False(t, ignore)
	assert.Equal(t, "cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"}", got)
}

func TestEquationFromExpr_NotEqual(t *testing.T) {

	got, ignore := EquationFromExpr("healthy{app=\"ct-perf\"} != 1")
	assert.False(t, ignore)
	assert.Equal(t, "healthy{app=\"ct-perf\"}", got)
}

func TestEquationFromExpr_Empty(t *testing.T) {

	got, ignore := EquationFromExpr("")
	assert.False(t, ignore)
	assert.Equal(t, "", got)
}

func TestEquationFromExpr_IgnoreComputedEquations(t *testing.T) {

	got, ignore := EquationFromExpr("computed:value")
	assert.True(t, ignore)
	assert.Equal(t, "", got)
}

func TestEquationFromExpr_IgnoreMultipleComparisons(t *testing.T) {

	got, ignore := EquationFromExpr("a < b and b > c")
	assert.True(t, ignore)
	assert.Equal(t, "", got)
}
