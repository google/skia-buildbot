package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

const (
	pinpointSwarmingTagName = "pinpoint_job_id"
)

// flag names
const (
	pinpointJobIDFlagName = "pinpoint-job"
	replayFromZipFlagName = "replay-from-zip"
	recordToZipFlagName   = "record-to-zip"
)

type commonCmd struct {
	pinpointJobID string
	recordToZip   string
	replayFromZip string
}

func (cmd *commonCmd) flags() []cli.Flag {
	pinpointJobIDFlag := &cli.StringFlag{
		Name:        pinpointJobIDFlagName,
		Value:       "",
		Usage:       "ID of the pinpoint job to check",
		Destination: &cmd.pinpointJobID,
	}
	replayFromZipFlag := &cli.StringFlag{
		Name:        replayFromZipFlagName,
		Value:       "",
		Usage:       "Zip file to replay data from",
		Destination: &cmd.replayFromZip,
		Action: func(ctx *cli.Context, v string) error {
			if cmd.recordToZip != "" {
				return fmt.Errorf("only one of -%s or -%s may be specified", replayFromZipFlagName, recordToZipFlagName)
			}
			return nil
		},
	}
	recordToZipFlag := &cli.StringFlag{
		Name:        recordToZipFlagName,
		Value:       "",
		Usage:       "Zip file to save replay data to",
		Destination: &cmd.recordToZip,
		Action: func(ctx *cli.Context, v string) error {
			if cmd.replayFromZip != "" {
				return fmt.Errorf("only one of -%s or -%s may be specified", replayFromZipFlagName, recordToZipFlagName)
			}
			return nil
		},
	}
	return []cli.Flag{pinpointJobIDFlag, replayFromZipFlag, recordToZipFlag}
}
