package main

import (
	"flag"
	"fmt"
	"html/template"

	"go.skia.org/infra/go/allowed"
)

// flags
var (
	authGroup          = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	namespace          = flag.String("namespace", "", "The Cloud Firestore namespace.")
	project            = flag.String("project", "skia-public", "The Google Cloud project name.")
)

type MachineState struct {
	Dimensions map[string][]interface{} `json:"dimensions"`
	State      map[string]interface{}   `json:"state"`
}

type server struct {
	machineState *machine.State
	templates    *template.Template
	allow        allowed.Allow // Who is allowed to use the site.
}

func main() {
	fmt.Println("Hello, 世界")
}
