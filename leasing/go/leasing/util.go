package main

import (
	"fmt"
)

func GetTrooperEmail() (string, error) {
	// TODO(rmistry): Read from http://skia-tree-status.appspot.com/current-trooper
	// {"username": "kjlubick@google.com", "schedule_start": "10/30", "schedule_end": "11/05"}
	return "rmistry@google.com", nil
}

func GetSwarmingTaskLink(server, taskId string) string {
	return fmt.Sprintf("https://%s/task?id=%s", server, taskId)
}
