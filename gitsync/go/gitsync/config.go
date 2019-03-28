package main

import (
	"fmt"
	"strconv"

	"go.skia.org/infra/go/human"
)

type gitSyncConfig struct {
	BTInstanceID    string             `json:"bt_instance"`
	BTTableID       string             `json:"bt_table"`
	HttpPort        string             `json:"http_port"`
	Local           bool               `json:"local"`
	ProjectID       string             `json:"project"`
	PromPort        string             `json:"prom_port"`
	RepoURLs        []string           `json:"repo_url"`
	RefreshInterval human.JSONDuration `json:"refresh"`
	WorkDir         string             `json:"workdir"`
}

func (g *gitSyncConfig) String() string {
	ret := ""
	prefix := "      "
	ret += fmt.Sprintf("%s bt_instance  : %s\n", prefix, g.BTInstanceID)
	ret += fmt.Sprintf("%s bt_table     : %s\n", prefix, g.BTTableID)
	ret += fmt.Sprintf("%s http_port    : %s\n", prefix, g.HttpPort)
	ret += fmt.Sprintf("%s local        : %s\n", prefix, strconv.FormatBool(g.Local))
	ret += fmt.Sprintf("%s project      : %s\n", prefix, g.ProjectID)
	ret += fmt.Sprintf("%s prom_port    : %s\n", prefix, g.PromPort)
	for _, url := range g.RepoURLs {
		ret += fmt.Sprintf("%s repo_url     : %s\n", prefix, url)
	}
	ret += fmt.Sprintf("%s refresh      : %s\n", prefix, g.RefreshInterval.String())
	ret += fmt.Sprintf("%s workdir      : %s\n", prefix, g.WorkDir)
	return ret
}
