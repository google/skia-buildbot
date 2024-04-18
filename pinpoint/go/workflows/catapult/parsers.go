package catapult

import (
	"go.skia.org/infra/pinpoint/go/compare"
)

// convertImprovementDir converts the improvement direction from string to int.
// UP, DOWN, UNKNOWN = (0, 1, 4)
func parseImprovementDir(dir compare.ImprovementDir) int32 {
	switch dir {
	case compare.Up:
		return 0
	case compare.Down:
		return 1
	default:
		return 4
	}
}
