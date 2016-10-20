// Package cq provides tools for interacting with the CQ tools.
package cq

import (
	"bytes"
	//"fmt"
	"github.com/golang/protobuf/proto"
	//"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitiles"
)

var ()

const (
	CQ_CFG_FILE_PATH = "infra/branch-config/cq.cfg"
	SKIA_REPO        = "https://skia.googlesource.com/skia"
)

// TODO(rmistry): Remove Client if you need nothing else here!
func NewClient() *Client {
	return &Client{}
}

type Client struct {
}

func (c *Client) GetCQTryBots() ([]string, error) {
	var buf bytes.Buffer
	if err := gitiles.NewRepo(SKIA_REPO).ReadFile(CQ_CFG_FILE_PATH, &buf); err != nil {
		return nil, err
	}
	var cqCfg Config
	if err := proto.UnmarshalText(buf.String(), &cqCfg); err != nil {
		return nil, err
	}
	tryJobs := []string{}
	for _, bucket := range cqCfg.Verifiers.GetTryJob().GetBuckets() {
		for _, builder := range bucket.GetBuilders() {
			if builder.GetExperimentPercentage() > 0 {
				// Exclude experimental builders.
				continue
			}
			tryJobs = append(tryJobs, builder.GetName())
		}
	}
	return tryJobs, nil
}
