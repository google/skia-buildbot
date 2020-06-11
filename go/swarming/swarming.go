/*
	Constants for swarming.
*/
package swarming

import (
	"time"
)

const (
	SWARMING_SERVER          = "chromium-swarm.appspot.com"
	SWARMING_SERVER_PRIVATE  = "chrome-swarming.appspot.com"
	SWARMING_SERVER_DEV      = "chromium-swarm-dev.appspot.com"
	RECOMMENDED_IO_TIMEOUT   = 20 * time.Minute
	RECOMMENDED_HARD_TIMEOUT = 1 * time.Hour
	RECOMMENDED_PRIORITY     = 90
	RECOMMENDED_EXPIRATION   = 4 * time.Hour
	// "priority 0 can only be used for terminate request"
	HIGHEST_PRIORITY = 1
	LOWEST_PRIORITY  = 255
)
