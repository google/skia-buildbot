package testdata

import "go.skia.org/infra/go/testutils"

// The JSON in these files was recieved from calls to
// https://chromium-swarm.appspot.com/_ah/api/swarming/v1/bot/[bot]/get
// when bots were displaying interesting behavior
var DEAD_BOT = "dead_bot.json"
var DEAD_AND_QUARANTINED = "dead_and_quarantined.json"
var DELETED_BOT = "deleted_bot.json"
var MISSING_DEVICE = "missing_device.json"
var TOO_HOT = "too_hot.json"
var USB_FAILURE = "usb_failure.json"

// ReadFile reads a file from the ./testdata directory.
func ReadFile(filename string) (string, error) {
	return testutils.ReadFile(filename)
}
