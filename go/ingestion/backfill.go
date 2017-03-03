package ingestion

import (
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func startBackFiller(source Source, outCh chan []ResultFileLocation, doneCh <-chan bool, initialTime, delta int64, repeatEvery time.Duration) {
	go func() {
		endTime := initialTime
		// util.Repeat(repeatEvery, doneCh, func() {
		for {
			startTime := endTime - delta
			sklog.Infof("Backfill polling range: %s - %s", time.Unix(startTime, 0), time.Unix(endTime, 0))
			resultFiles, err := source.Poll(startTime, endTime)
			if err != nil {
				sklog.Errorf("Backfill: error polling data source '%s': %s", source.ID(), err)
				return
			}
			sklog.Infof("Backfiller: Sending %d result files", len(resultFiles))
			for len(resultFiles) > 0 {
				chunkSize := util.MinInt(POLL_CHUNK_SIZE, len(resultFiles))
				outCh <- resultFiles[:chunkSize]
				resultFiles = resultFiles[chunkSize:]
			}
			sklog.Infof("Backfiller: Sent %d result files", len(resultFiles))
			endTime = startTime
		}
	}()
}
