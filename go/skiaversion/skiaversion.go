/*
	Reports version information.

	Requires running "make skiaversion" to set the constants.
*/

package skiaversion

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/skia-dev/glog"
)

// Date format as reported by 'date --rfc-3339="second"'
const DATE_FORMAT = "2006-01-02 15:04:05-07:00"

var parsedDate time.Time

func init() {
	var err error
	parsedDate, err = time.Parse(DATE_FORMAT, DATE)
	if err != nil {
		glog.Fatalf("Failed to parse build date. Did you forget to run \"make skiaversion\"? %v", err)
	}
}

// Version holds information about the version of code this program is running.
type Version struct {
	Commit string    `json:"commit"`
	Date   time.Time `json:"date"`
}

// GetVersion returns a Version object for this program.
func GetVersion() *Version {
	return &Version{COMMIT, parsedDate}
}

// JsonHandler is a pre-built handler for HTTP requests which returns version
// information in JSON format.
func JsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	bytes, err := json.Marshal(GetVersion())
	if err != nil {
		glog.Error(err)
	}
	w.Write(bytes)
}
