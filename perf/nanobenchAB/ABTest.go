package main

import (
	"flag"
	"fmt"
	"github.com/aclements/go-moremath/stats"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var outputCheck = regexp.MustCompile("^curr/maxrss\\s+loops\\s+min\\s+median\\s+mean\\s+max\\s+stddev\\s+samples\\s+config\\s+bench.*$")

func runNanobench(binary, skp string, samplesPerRun int, samples []float64) string {
	var commonFlags = []string{"--config", "gl", "-v", "--samples", strconv.Itoa(samplesPerRun), "--match"}

	cmd := &exec.Cmd {
		Path: binary,
		Args: append([]string{binary}, append(commonFlags, skp)...),
	}
	output, err := cmd.CombinedOutput()
	must(err)

	lines := strings.Split(string(output), "\n")

	if !outputCheck.MatchString(lines[1]) {
		panic("Unexpected format. Did nanobench's format change?")
	}

	awkify := strings.Split(lines[3], "  ")
	if len(awkify) - 2 != len(samples) {
		panic("Problem gathering samples")
	}
	for i, _ := range samples {
		sample, sampleErr := strconv.ParseFloat(awkify[i + 1], 64)
		must(sampleErr)
		samples[i] = sample
	}

	return awkify[len(awkify) - 1]
}

func bootstrapMeanDiff(oldSample, newSample []float64) []float64 {
	resampleCount := 20000
	diffMeans := make([]float64, resampleCount)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sampleCount := len(oldSample)
	for s := 0; s < resampleCount; s++ {
		var oldSum, newSum float64
		for i := 0; i < sampleCount; i++ {
			oldSum += oldSample[r.Intn(sampleCount)]
			newSum += newSample[r.Intn(sampleCount)]
		}
		diffMeans[s] = (newSum - oldSum) / float64(sampleCount)
	}
	return diffMeans
}

func diffConfidenceInterval(confidence float64, oldSamples []float64, newSamples []float64) (float64, float64) {
	alpha := (1 - (confidence / 100)) / 2
	diffMeans := bootstrapMeanDiff(oldSamples, newSamples)
	sort.Float64s(diffMeans)
	alphaIndex := int(math.Floor(alpha * float64(len(diffMeans))))
	return diffMeans[alphaIndex], diffMeans[len(diffMeans) - alphaIndex]
}

func skpFromPath(path string) string {
	_, file := filepath.Split(path)
	return strings.TrimSuffix(file, filepath.Ext(file))
}

func main() {
	samplesPerRun := 10
	trials := 10

	oldPath := flag.String("old", "", "path to old binary")
	newPath := flag.String("new", "", "path to new binary")
	skpsPath := flag.String("skps", "./skps", "directory that holds skps")

	flag.Parse()

	oldBinary, err := exec.LookPath(*oldPath)
	must(err)

	newBinary, err := exec.LookPath(*newPath)
	must(err)

	skpsGlob := filepath.Join(*skpsPath, "*.skp")
	skpNames, err := filepath.Glob(skpsGlob)
	must(err)

	for i := 0; i < len(skpNames); i++ {
		skpNames[i] = skpFromPath(skpNames[i])
	}

	//skpNames = []string{"desk_nytimes", "desk_cnn", "tabl_mozilla", "mobi_techcrunch"}

	totalSamples := trials * samplesPerRun
	oldSamples := make([]float64, totalSamples)
	newSamples := make([]float64, totalSamples)

	for _, skpName := range skpNames {

		if skpName == "desk_carsvg" {
			continue
		}

		for trial := 0; trial < trials; trial++ {
			startSample := trial * samplesPerRun
			endSample := startSample + samplesPerRun
			if trial % 1 == 0 {
				runNanobench(oldBinary, skpName, samplesPerRun, oldSamples[startSample:endSample])
				runNanobench(newBinary, skpName, samplesPerRun, newSamples[startSample:endSample])
			} else {
				runNanobench(newBinary, skpName, samplesPerRun, newSamples[startSample:endSample])
				runNanobench(oldBinary, skpName, samplesPerRun, oldSamples[startSample:endSample])
			}
		}

		u, err := stats.MannWhitneyUTest(oldSamples, newSamples, stats.LocationDiffers)
		must(err)

		sort.Float64s(oldSamples)
		sort.Float64s(newSamples)

		// Assume that higher 3/4 of the data are noisy because of system interference.
		lowestQuartile := (trials / 4) * samplesPerRun
		lowestQuartileOld := oldSamples[:lowestQuartile]
		lowestQuartileNew := newSamples[:lowestQuartile]

		confidence := 99.5
		low, high := diffConfidenceInterval(confidence, lowestQuartileOld, lowestQuartileNew)

		if (u.P > 0.001) {
			fmt.Print("There is no difference by U-Test ")
		} else if low < 0 && 0 < high {
			fmt.Print("There is no difference by confidence interval ")
		} else {
			fmt.Print("There is a difference ")
		}

		oldMean := stats.Mean(lowestQuartileOld)
		newMean := stats.Mean(lowestQuartileNew)
		fmt.Printf("%s new (%.0f) is %0.3f times old (%.0f) %.2f%% confidence interval (%.1f, %.1f) U=%0.3f\n",
			skpName, newMean, newMean / oldMean, oldMean, confidence, low, high, u.P)
	}

	os.Exit(0)
}
