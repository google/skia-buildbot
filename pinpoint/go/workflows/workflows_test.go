package workflows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/pinpoint/go/compare"
	pinpointpb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestBisectParams_GetMagnitude(t *testing.T) {
	makeParam := func(magnitude string) *BisectParams {
		return &BisectParams{
			Request: &pinpointpb.ScheduleBisectRequest{
				ComparisonMagnitude: magnitude,
			},
		}
	}

	assert.InDelta(t, 1.0, makeParam("").GetMagnitude(), 1e-9)
	assert.InDelta(t, 1.0, makeParam("string").GetMagnitude(), 1e-9)
	assert.Zero(t, makeParam("0").GetMagnitude())
	assert.InDelta(t, 1.3, makeParam("1.3").GetMagnitude(), 1e-9)
	assert.InDelta(t, 12.1, makeParam("12.1").GetMagnitude(), 1e-9)
}

func TestBisectParams_GetInitialAttempt(t *testing.T) {
	makeParam := func(attempt string) *BisectParams {
		return &BisectParams{
			Request: &pinpointpb.ScheduleBisectRequest{
				InitialAttemptCount: attempt,
			},
		}
	}

	assert.Zero(t, makeParam("").GetInitialAttempt())
	assert.Zero(t, makeParam("string").GetInitialAttempt())
	assert.Zero(t, makeParam("0").GetInitialAttempt())
	assert.EqualValues(t, 50, makeParam("50").GetInitialAttempt())
}

func TestGetImprovementDirection_GivenDirection_ReturnsCorrectDirection(t *testing.T) {
	test := func(direction string, expected compare.ImprovementDir) {
		params := &BisectParams{
			Request: &pinpointpb.ScheduleBisectRequest{
				ImprovementDirection: direction,
			},
		}
		assert.Equal(t, params.GetImprovementDirection(), expected)
	}

	test("UP", compare.Up)
	test("Down", compare.Down)
	test("UNKNOWN", compare.UnknownDir)
	test("fake-dir", compare.UnknownDir)
}
