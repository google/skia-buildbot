package service_accounts

/*
	Configuration for service accounts in the Skolo.

	TODO(borenet): This should maybe be in a config file, but it's
	convenient to have it in code.
*/

var (
	// Default service account used for bots which connect to
	// chrome-swarming.appspot.com.
	chromeSwarming = &ServiceAccount{
		Project:  "google.com:skia-buildbots",
		Email:    "chrome-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com",
		Nickname: "swarming",
	}

	// Default service account used for bots which connect to
	// chromium-swarm.appspot.com.
	chromiumSwarm = &ServiceAccount{
		Project:  "skia-swarming-bots",
		Email:    "chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com",
		Nickname: "swarming",
	}

	// Service account used by the jumphost itself.
	jumphost = &ServiceAccount{
		Project:  "google.com:skia-buildbots",
		Email:    "jumphost@skia-buildbots.google.com.iam.gserviceaccount.com",
		Nickname: "jumphost",
	}

	// Service account used by the RPi masters.
	rpiMaster = &ServiceAccount{
		Project:  "google.com:skia-buildbots",
		Email:    "rpi-master@skia-buildbots.google.com.iam.gserviceaccount.com",
		Nickname: "rpi-master",
	}

	// Determines which keys go on which machines:
	// map[hostname][]*ServiceAccount
	JumphostServiceAccountMapping = map[string][]*ServiceAccount{
		"jumphost-internal-01": []*ServiceAccount{
			chromeSwarming,
			jumphost,
			rpiMaster,
		},
		"jumphost-linux-01": []*ServiceAccount{
			chromiumSwarm,
			jumphost,
		},
		"jumphost-rpi-01": []*ServiceAccount{
			chromiumSwarm,
			jumphost,
			rpiMaster,
		},
		"jumphost-win-02": []*ServiceAccount{
			chromiumSwarm,
			jumphost,
		},
		"jumphost-win-03": []*ServiceAccount{
			chromiumSwarm,
			jumphost,
		},
	}

	// Maps hostnames to the address used to SSH into each jumphost.
	JumphostSSHMapping = map[string]string{
		"jumphost-internal-01": "internal-01.skolo",
		"jumphost-linux-01":    "linux-01.skolo",
		"jumphost-rpi-01":      "rpi-01.skolo",
		"jumphost-win-02":      "win-02.skolo",
		"jumphost-win-03":      "win-03.skolo",
	}
)

// ServiceAccount is a struct representing a service account.
type ServiceAccount struct {
	Project  string
	Email    string
	Nickname string
}
