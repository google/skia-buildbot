// Package cq provides tools for interacting with the CQ tools.
package cq

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitiles"
)

var ()

const ()

func NewClient() *Client {
	return &Client{}
}

type Client struct {
}

func (c *Client) getCQBuilders() (string, error) {
	glog.Info("blah")
	var buf bytes.Buffer
	// TODO(rmistry):Move some of these into constants
	if err := gitiles.NewRepo("https://skia.googlesource.com/skia").ReadFile("infra/branch-config/cq.cfg", &buf); err != nil {
		return "", err
	}
	var cqCfg Config
	if err := proto.UnmarshalText(buf.String(), &cqCfg); err != nil {
		return "", err
	}
	for _, bucket := range cqCfg.Verifiers.GetTryJob().GetBuckets() {
		for _, builder := range bucket.GetBuilders() {
			if builder.GetExperimentPercentage() > 0 {
				// Exclude experimental builders.
				continue
			}
			fmt.Println(builder.GetName())
		}
	}
	return "", nil
}
