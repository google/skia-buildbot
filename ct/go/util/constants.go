package util

const (
	// TODO(rmistry): Switch this to use chrome-bot when ready to run in prod
	CT_USER         = "rmistry"
	NUM_WORKERS int = 100
)

var (
	// Slaves  = GetCTWorkers()
	// TODO(rmistry): Switch this to use GetCTWorkers() when ready to run in prod
	Slaves = []string{
		"epoger-linux.cnc.corp.google.com",
		"piraeus.cnc.corp.google.com",
		"172.23.212.25",
	}
)
