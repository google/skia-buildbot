// This executable was helpful when trying to run commands across all of our k8s nodes.
// When dealing with "failed to garbage collect required amount of images" logs and
// FreeDiskSpaceFailed, ImageGCFailed events, we found it helpful to run
// docker image prune --all --force across all the nodes after gathering information about
// affected nodes.
// Feel free to edit/adapt this script for future node inquiries.
package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const (
	// TODO(kjlubick) these could be flags if we really needed them to be
	projectID = "skia-public"
	zoneID    = "us-central1-a"
	sshKey    = "/home/kjlubick/.ssh/google_compute_engine"
	user      = "kjlubick"
)

func getIPAddress(ctx context.Context, node string) string {
	ipCmd := exec.CommandContext(ctx, "gcloud", "compute", "instances", "describe", node,
		`--format=get(networkInterfaces[0].accessConfigs[0].natIP)`, "--project", projectID, "--zone", zoneID)
	out, err := ipCmd.Output()
	if err != nil {
		ee := err.(*exec.ExitError)
		panic(err.Error() + ee.String() + string(ee.Stderr))
	}
	return strings.TrimSpace(string(out))
}

func runSSHCommand(ctx context.Context, node, sshCommand string) {
	ip := getIPAddress(ctx, node)
	sshCmd := exec.CommandContext(ctx, "ssh", user+"@"+ip,
		"-o", "ProxyCommand=corp-ssh-helper %h %p",
		"-o", "StrictHostKeyChecking=no",
		"-i", sshKey,
		sshCommand,
	)
	out, err := sshCmd.Output()
	if err != nil {
		ee := err.(*exec.ExitError)
		panic(err.Error() + ee.String() + string(ee.Stderr))
	}
	fmt.Printf("%s\t%s", node, string(out))
}

func getCreatedTimestamp(ctx context.Context, node string) string {
	cmd := exec.CommandContext(ctx, "kubectl", "describe", "node", node)
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	findCreation := regexp.MustCompile(`CreationTimestamp:\s*(.+?)\n`)
	if match := findCreation.FindStringSubmatch(string(out)); len(match) > 0 {
		return match[1]
	}
	return "<unknown>"
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "--output=name")
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	nodes := strings.Split(string(out), "\n")
	if len(nodes) == 0 {
		panic("No nodes found")
	}
	for _, node := range nodes {
		node = strings.TrimPrefix(node, "node/")
		if len(node) == 0 {
			continue
		}
		runSSHCommand(ctx, node, "docker image prune --all --force")
	}
}
