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

func runNanobench(binary, skp string, samplesPerRun int, samples []float64) string {
	var commonFlags = []string{"--config", "gl", "-v", "--samples", strconv.Itoa(samplesPerRun), "--match"}

	cmd := &exec.Cmd {
		Path: binary,
		Args: append([]string{binary}, append(commonFlags, skp)...),
	}
	output, err := cmd.CombinedOutput()
	must(err)

	lines := strings.Split(string(output), "\n")

	var outputCheck = regexp.MustCompile(
		"^curr/maxrss\\s+loops\\s+min\\s+median\\s+mean\\s+max\\s+stddev\\s+samples\\s+config\\s+bench.*$")
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

// Resample using bootstrapping to calculate a new sample of the diff of the means (how much faster or slower).
// We can use this new sample to construct the confidence intervals for the means of the difference.
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
	samplesPerRun := 5
	trials := 100

	oldPath := flag.String("old", "", "path to old binary")
	newPath := flag.String("new", "", "path to new binary")
	skpsPath := flag.String("skps", "./skps", "directory that holds skps")
	textSkps := flag.Bool("text", false, "measure text performance")

	flag.Parse()

	oldBinary, err := exec.LookPath(*oldPath)
	must(err)

	newBinary, err := exec.LookPath(*newPath)
	must(err)

	var skpNames []string
	if *textSkps {
		skpNames = []string{"desk_nytimes", "desk_cnn", "desk_espn", "desk_facebook", "desk_googlecalendar",
			"desk_googlesearch", "tabl_mozilla", "mobi_techcrunch"}
	} else {
		skpsGlob := filepath.Join(*skpsPath, "*.skp")
		skpNames, err = filepath.Glob(skpsGlob)
		must(err)
	}

	for i := 0; i < len(skpNames); i++ {
		skpNames[i] = skpFromPath(skpNames[i])
	}

	totalSamples := trials * samplesPerRun
	oldSamples := make([]float64, totalSamples)
	newSamples := make([]float64, totalSamples)

	// warm up - this ends up being important for reducing noise.
	runNanobench(oldBinary, skpNames[0], samplesPerRun, oldSamples[0:samplesPerRun])
	runNanobench(newBinary, skpNames[0], samplesPerRun, newSamples[0:samplesPerRun])

	for _, skpName := range skpNames {

		// Skip for now because it takes soooo long to run.
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

		alpha := (1 - (confidence / 100)) / 2
		if u.P < alpha && (high < 0 || 0 < low) {
			fmt.Print("There is a difference ")
		} else if u.P >= alpha && low < 0 && 0 < high {
			fmt.Print("There is no difference ")
		}  else {
			fmt.Print("There may be a difference ")
		}

		oldMean := stats.Mean(lowestQuartileOld)
		newMean := stats.Mean(lowestQuartileNew)
		fmt.Printf("%s new (%.0f) is %0.3f times old (%.0f) %.2f%% confidence interval (%.1f, %.1f) diff mean %.0f U=%0.3f\n",
			skpName, newMean, newMean / oldMean, oldMean, confidence, low, high, newMean - oldMean, u.P)
	}

	os.Exit(0)
}

/*
There may be a difference desk_nytimes new (239388) is 1.010 times old (236991) 99.50% confidence interval (-884.9, 5553.6) diff mean 2398 U=0.000
There is a difference desk_cnn new (177269) is 1.021 times old (173620) 99.50% confidence interval (2975.9, 4310.7) diff mean 3649 U=0.000
There may be a difference desk_espn new (164704) is 1.007 times old (163495) 99.50% confidence interval (646.2, 1772.7) diff mean 1209 U=0.046
There may be a difference desk_facebook new (432430) is 1.004 times old (430908) 99.50% confidence interval (186.7, 2867.4) diff mean 1523 U=0.373
There is a difference desk_googlecalendar new (512823) is 1.013 times old (506346) 99.50% confidence interval (5068.6, 7870.0) diff mean 6477 U=0.000
There is a difference desk_googlesearch new (286958) is 1.020 times old (281405) 99.50% confidence interval (4783.4, 6306.9) diff mean 5554 U=0.000
There is a difference tabl_mozilla new (153252) is 1.034 times old (148275) 99.50% confidence interval (4621.5, 5315.2) diff mean 4977 U=0.000
There is a difference mobi_techcrunch new (188642) is 1.026 times old (183793) 99.50% confidence interval (4308.9, 5381.4) diff mean 4849 U=0.000


 */