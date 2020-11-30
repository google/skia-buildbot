package main

import (
	"io/ioutil"
	"log"
	"os"

	"go.skia.org/infra/task_scheduler/go/specs"
)

func main() {
	tasksJSONs := os.Args[1:]
	if len(tasksJSONs) == 0 {
		log.Fatal("Specify at least one tasks.json to validate.")
	}
	for _, tasksJSON := range tasksJSONs {
		contents, err := ioutil.ReadFile(tasksJSON)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := specs.ParseTasksCfg(string(contents)); err != nil {
			log.Fatalf("%s: %s", tasksJSON, err)
		}
	}
}
