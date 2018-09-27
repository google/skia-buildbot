package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/sklog"
)

func main() {
	flag.Parse()
	defer sklog.Flush()
	resultsFile := "results.json"
	cmdStr := []string{"build\\nanobench.exe", "--skps", "skp", "--config", "angle_d3d11_es2", "--match", "desk_skbug6850fast.skp", "--outResultsFile", resultsFile}
	cmd := exec.Command(cmdStr[0], cmdStr[1:]...)
	sklog.Infof("Executing %s", strings.Join(cmdStr, " "))
	out, err := cmd.CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error running %s: %s\nOutput:\n%s", strings.Join(cmdStr, " "), err, out)
	}
	res, err := ioutil.ReadFile(resultsFile)
	if err != nil {
		sklog.Fatal(err)
	}
	var data interface{}
	err = json.Unmarshal(res, &data)
	if err != nil {
		sklog.Fatal(err)
	}
	for _, k := range []string{"results", "desk_skbug6850fast.skp_1_1000_1000", "angle_d3d11_es2", "min_ms"} {
		datam, ok := data.(map[string]interface{})
		if !ok {
			sklog.Fatalf("For key %q, can't cast %v to map[string]interface{}.", k, data)
		}
		data, ok = datam[k]
		if !ok {
			sklog.Fatalf("No key %q in %v.", k, datam)
		}
	}
	ms := data.(float64)
	sklog.Infof("--------------------------------------------------")
	if ms > 0.31 {
		sklog.Infof("slower")
	} else {
		sklog.Infof("faster")
	}
}
