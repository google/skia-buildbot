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

		if (u.P > 0.001) {
			fmt.Print("There is no difference by U-Test ")
		} else if low < 0 && 0 < high {
			fmt.Print("There is no difference by confidence interval ")
		} else {
			fmt.Print("There is a difference ")
		}

		oldMean := stats.Mean(lowestQuartileOld)
		newMean := stats.Mean(lowestQuartileNew)
		fmt.Printf("%s new (%.0f) is %0.3f times old (%.0f) %.2f%% confidence interval (%.1f, %.1f) diff mean %.0f U=%0.3f\n",
			skpName, newMean, newMean / oldMean, oldMean, confidence, low, high, newMean - oldMean, u.P)
	}

	os.Exit(0)
}

/*
Here is the output of two types of runs. 1) where the tested systems are the same 2) where the new system
is about 2% slower in text drawing than the old system.

//tested systems are the same 5-samples 30-trials
There is no difference by U-Test desk_chalkboard new (5204246) is 1.025 times old (5079341) 99.50% confidence interval (-82285.1, 339115.8) diff mean 124905 U=0.605
There is no difference by U-Test desk_cnn new (207033) is 1.008 times old (205462) 99.50% confidence interval (-3049.3, 6041.8) diff mean 1571 U=0.516
There is no difference by U-Test desk_css3gradients new (153185) is 0.996 times old (153738) 99.50% confidence interval (-3960.8, 3056.4) diff mean -553 U=0.549
There is no difference by U-Test desk_ebay new (81390) is 0.998 times old (81547) 99.50% confidence interval (-2290.0, 2069.7) diff mean -157 U=0.119
There is no difference by U-Test desk_espn new (168398) is 0.997 times old (168885) 99.50% confidence interval (-1557.4, 607.3) diff mean -487 U=0.803
There is no difference by U-Test desk_facebook new (438419) is 0.998 times old (439366) 99.50% confidence interval (-5297.1, 3242.3) diff mean -947 U=0.239
There is no difference by U-Test desk_gmail new (462582) is 0.992 times old (466247) 99.50% confidence interval (-6928.5, -383.1) diff mean -3664 U=0.004
There is no difference by U-Test desk_googlecalendar new (510978) is 0.993 times old (514665) 99.50% confidence interval (-6804.1, -434.1) diff mean -3687 U=0.439
There is no difference by U-Test desk_googledocs new (446650) is 1.000 times old (446456) 99.50% confidence interval (-2569.9, 3170.2) diff mean 194 U=0.681
There is no difference by U-Test desk_googleimagesearch new (455905) is 0.999 times old (456162) 99.50% confidence interval (-2463.9, 1902.4) diff mean -257 U=0.322
There is no difference by U-Test desk_googlesearch new (284248) is 1.004 times old (283117) 99.50% confidence interval (-680.4, 2874.1) diff mean 1131 U=0.969
There is no difference by U-Test desk_googlespreadsheet new (2895927) is 1.004 times old (2883761) 99.50% confidence interval (-5587.9, 30870.2) diff mean 12166 U=0.920
There is no difference by confidence interval desk_linkedin new (1069090) is 1.005 times old (1063692) 99.50% confidence interval (-1682.0, 12569.7) diff mean 5398 U=0.000
There is no difference by U-Test desk_mapsvg new (2858704) is 0.999 times old (2860699) 99.50% confidence interval (-13330.1, 10260.2) diff mean -1995 U=0.598
There is no difference by U-Test desk_nytimes new (227624) is 1.008 times old (225767) 99.50% confidence interval (-289.9, 4009.8) diff mean 1857 U=0.015
There is no difference by U-Test desk_pokemonwiki new (1284741) is 1.005 times old (1278175) 99.50% confidence interval (977.1, 12498.9) diff mean 6566 U=0.014
There is no difference by U-Test desk_samoasvg new (2069326) is 0.994 times old (2082254) 99.50% confidence interval (-31698.1, 5051.1) diff mean -12928 U=0.169
There is no difference by U-Test desk_theverge new (127681) is 1.003 times old (127358) 99.50% confidence interval (-781.3, 1404.3) diff mean 324 U=0.970
There is no difference by U-Test desk_tiger8svg new (6161311) is 0.996 times old (6186728) 99.50% confidence interval (-69921.8, 18174.5) diff mean -25417 U=0.553
There is no difference by U-Test desk_tigersvg new (2888217) is 1.003 times old (2879391) 99.50% confidence interval (-6620.7, 24886.7) diff mean 8826 U=0.123
There is no difference by U-Test desk_twitter new (585557) is 0.996 times old (587917) 99.50% confidence interval (-6635.0, 1696.1) diff mean -2361 U=0.649
There is no difference by U-Test desk_weather new (414795) is 0.985 times old (421160) 99.50% confidence interval (-15515.5, 2007.7) diff mean -6365 U=0.126
There is no difference by U-Test desk_wikipedia new (1032269) is 0.996 times old (1035960) 99.50% confidence interval (-10056.7, 2568.7) diff mean -3691 U=0.465
There is no difference by U-Test desk_wowwiki new (228429) is 1.000 times old (228495) 99.50% confidence interval (-1494.4, 1432.0) diff mean -66 U=0.103
There is no difference by U-Test desk_yahooanswers new (386413) is 1.003 times old (385355) 99.50% confidence interval (-866.7, 3055.1) diff mean 1058 U=0.033
There is no difference by U-Test desk_yahoosports new (627869) is 0.992 times old (632991) 99.50% confidence interval (-9200.4, -772.3) diff mean -5121 U=0.628
There is no difference by U-Test desk_ynevsvg new (15758532) is 1.001 times old (15745577) 99.50% confidence interval (-64484.8, 87937.0) diff mean 12955 U=0.356
There is no difference by U-Test desk_youtube new (79032) is 0.999 times old (79137) 99.50% confidence interval (-706.9, 477.3) diff mean -105 U=0.370
There is no difference by U-Test mobi_amazon new (274126) is 1.007 times old (272210) 99.50% confidence interval (421.7, 3467.1) diff mean 1916 U=0.070
There is no difference by U-Test mobi_baidu new (267443) is 1.004 times old (266487) 99.50% confidence interval (-579.3, 2481.6) diff mean 956 U=0.530
There is no difference by U-Test mobi_booking new (518319) is 1.002 times old (517425) 99.50% confidence interval (-2526.5, 4376.9) diff mean 894 U=0.174
There is no difference by U-Test mobi_capitalvolkswagen new (287200) is 0.993 times old (289345) 99.50% confidence interval (-4553.9, 222.1) diff mean -2145 U=0.012
There is no difference by U-Test mobi_cnn new (176676) is 1.002 times old (176391) 99.50% confidence interval (-1401.2, 1996.3) diff mean 285 U=0.135
There is no difference by U-Test mobi_cnnarticle new (95580) is 1.002 times old (95370) 99.50% confidence interval (-105.9, 521.5) diff mean 209 U=0.089
There is no difference by U-Test mobi_deviantart new (440394) is 0.999 times old (441006) 99.50% confidence interval (-4327.0, 3102.5) diff mean -612 U=0.337
There is no difference by U-Test mobi_facebook new (144895) is 0.995 times old (145609) 99.50% confidence interval (-4036.7, 2836.1) diff mean -714 U=0.065
There is no difference by U-Test mobi_forecastio new (339136) is 1.004 times old (337824) 99.50% confidence interval (-1442.1, 4240.9) diff mean 1312 U=0.588
There is no difference by U-Test mobi_googlenews new (138924) is 0.991 times old (140144) 99.50% confidence interval (-2284.1, -111.7) diff mean -1221 U=0.029
There is no difference by U-Test mobi_googlesearch new (174085) is 0.999 times old (174251) 99.50% confidence interval (-1559.6, 1232.1) diff mean -165 U=0.320
There is a difference mobi_reddit new (133395) is 1.005 times old (132680) 99.50% confidence interval (288.1, 1125.2) diff mean 715 U=0.001
There is no difference by U-Test mobi_slashdot new (355469) is 0.999 times old (355987) 99.50% confidence interval (-5991.5, 4716.9) diff mean -518 U=0.747
There is no difference by U-Test mobi_techcrunch new (185960) is 1.002 times old (185660) 99.50% confidence interval (-813.0, 1401.1) diff mean 300 U=0.861
There is no difference by U-Test mobi_theverge new (231183) is 1.006 times old (229762) 99.50% confidence interval (-169.4, 3011.1) diff mean 1421 U=0.008
There is no difference by U-Test mobi_wikipedia new (488847) is 1.012 times old (483039) 99.50% confidence interval (2930.5, 8554.6) diff mean 5807 U=0.188
There is no difference by U-Test mobi_youtube new (67035) is 0.999 times old (67070) 99.50% confidence interval (-389.5, 351.4) diff mean -36 U=0.222
There is no difference by U-Test tabl_digg new (292306) is 0.999 times old (292512) 99.50% confidence interval (-2324.4, 1729.4) diff mean -206 U=0.639
There is no difference by U-Test tabl_mozilla new (150559) is 0.998 times old (150854) 99.50% confidence interval (-1172.4, 564.1) diff mean -295 U=0.795
There is no difference by U-Test tabl_pravda new (229655) is 1.010 times old (227463) 99.50% confidence interval (788.0, 3628.3) diff mean 2192 U=0.004
There is no difference by U-Test tabl_worldjournal new (287506) is 1.002 times old (287001) 99.50% confidence interval (-814.7, 1817.1) diff mean 504 U=0.301

// tested systems are different run 1  / 5-samples 30-trials
There is no difference by U-Test desk_chalkboard new (4825461) is 1.008 times old (4788866) 99.50% confidence interval (6921.0, 64782.1) diff mean 36595 U=0.011
There is a difference desk_cnn new (179425) is 1.025 times old (175004) 99.50% confidence interval (3210.1, 5616.3) diff mean 4422 U=0.000
There is no difference by U-Test desk_css3gradients new (154852) is 1.001 times old (154653) 99.50% confidence interval (-500.2, 820.3) diff mean 199 U=0.442
There is no difference by U-Test desk_ebay new (83466) is 0.996 times old (83778) 99.50% confidence interval (-913.4, 113.3) diff mean -312 U=0.274
There is a difference desk_espn new (168409) is 1.020 times old (165169) 99.50% confidence interval (2258.8, 4235.3) diff mean 3240 U=0.001
There is a difference desk_facebook new (446082) is 1.020 times old (437355) 99.50% confidence interval (5602.3, 11673.3) diff mean 8727 U=0.001
There is no difference by U-Test desk_gmail new (474185) is 1.008 times old (470214) 99.50% confidence interval (333.1, 7579.3) diff mean 3971 U=0.593
There is a difference desk_googlecalendar new (525400) is 1.019 times old (515516) 99.50% confidence interval (6440.7, 13313.3) diff mean 9884 U=0.000
There is no difference by confidence interval desk_googledocs new (449486) is 1.004 times old (447610) 99.50% confidence interval (-1983.3, 5564.9) diff mean 1876 U=0.000
There is no difference by U-Test desk_googleimagesearch new (456170) is 1.003 times old (454994) 99.50% confidence interval (-1650.0, 3969.0) diff mean 1176 U=0.006
There is a difference desk_googlesearch new (290688) is 1.018 times old (285544) 99.50% confidence interval (3338.3, 6869.1) diff mean 5144 U=0.001
There is no difference by U-Test desk_googlespreadsheet new (2910339) is 1.012 times old (2876891) 99.50% confidence interval (19786.9, 47221.7) diff mean 33448 U=0.032
There is a difference desk_linkedin new (1082856) is 1.021 times old (1060476) 99.50% confidence interval (15247.3, 29652.7) diff mean 22380 U=0.000
There is no difference by U-Test desk_mapsvg new (2855481) is 1.000 times old (2854867) 99.50% confidence interval (-12182.7, 12890.7) diff mean 614 U=0.354
There is a difference desk_nytimes new (243053) is 1.055 times old (230299) 99.50% confidence interval (8645.2, 16958.4) diff mean 12754 U=0.000
There is a difference desk_pokemonwiki new (1297146) is 1.019 times old (1272657) 99.50% confidence interval (18153.9, 30932.4) diff mean 24489 U=0.000
There is no difference by U-Test desk_samoasvg new (2062459) is 1.004 times old (2054088) 99.50% confidence interval (-3767.6, 21161.8) diff mean 8370 U=0.228
There is no difference by U-Test desk_theverge new (127967) is 1.010 times old (126665) 99.50% confidence interval (286.7, 2308.2) diff mean 1303 U=0.014
There is no difference by U-Test desk_tiger8svg new (6160019) is 1.008 times old (6111690) 99.50% confidence interval (12308.7, 85771.4) diff mean 48329 U=0.013
There is no difference by U-Test desk_tigersvg new (2855877) is 1.005 times old (2840501) 99.50% confidence interval (-1934.3, 33031.0) diff mean 15377 U=0.237
There is a difference desk_twitter new (594548) is 1.026 times old (579353) 99.50% confidence interval (10877.5, 19381.4) diff mean 15196 U=0.000
There is no difference by U-Test desk_weather new (363337) is 1.010 times old (359574) 99.50% confidence interval (1344.7, 6087.1) diff mean 3763 U=0.003
There is no difference by U-Test desk_wikipedia new (1012098) is 1.003 times old (1008705) 99.50% confidence interval (-2050.7, 8378.9) diff mean 3393 U=0.017
There is no difference by U-Test desk_wowwiki new (229311) is 1.015 times old (225993) 99.50% confidence interval (1928.8, 4750.5) diff mean 3318 U=0.030
There is a difference desk_yahooanswers new (393619) is 1.029 times old (382618) 99.50% confidence interval (8665.8, 13187.5) diff mean 11001 U=0.000
There is no difference by U-Test desk_yahoosports new (640329) is 1.003 times old (638380) 99.50% confidence interval (-1185.3, 4970.7) diff mean 1949 U=0.017
There is no difference by U-Test desk_ynevsvg new (15693664) is 1.000 times old (15699251) 99.50% confidence interval (-78729.4, 66537.6) diff mean -5587 U=0.433
There is no difference by U-Test desk_youtube new (78609) is 1.010 times old (77799) 99.50% confidence interval (452.0, 1183.3) diff mean 810 U=0.032
There is no difference by U-Test mobi_amazon new (276989) is 1.021 times old (271270) 99.50% confidence interval (4485.7, 6957.5) diff mean 5719 U=0.001
There is a difference mobi_baidu new (269774) is 1.027 times old (262622) 99.50% confidence interval (6036.9, 8344.6) diff mean 7152 U=0.000
There is a difference mobi_booking new (513374) is 1.015 times old (506018) 99.50% confidence interval (4586.0, 10232.1) diff mean 7356 U=0.000
There is a difference mobi_capitalvolkswagen new (290136) is 1.029 times old (281930) 99.50% confidence interval (6963.2, 9505.8) diff mean 8206 U=0.000
There is a difference mobi_cnn new (176581) is 1.031 times old (171205) 99.50% confidence interval (4474.9, 6287.6) diff mean 5376 U=0.000
There is no difference by U-Test mobi_cnnarticle new (96359) is 1.001 times old (96257) 99.50% confidence interval (-240.9, 432.6) diff mean 102 U=0.044
There is a difference mobi_deviantart new (432589) is 1.024 times old (422624) 99.50% confidence interval (8056.5, 11919.1) diff mean 9965 U=0.000
There is no difference by U-Test mobi_facebook new (148560) is 0.990 times old (150083) 99.50% confidence interval (-2144.0, -962.4) diff mean -1523 U=0.045
There is a difference mobi_forecastio new (335567) is 1.027 times old (326793) 99.50% confidence interval (7196.3, 10339.7) diff mean 8774 U=0.000
There is a difference mobi_googlenews new (136487) is 1.028 times old (132805) 99.50% confidence interval (2980.0, 4417.8) diff mean 3681 U=0.000
There is a difference mobi_googlesearch new (170793) is 1.028 times old (166087) 99.50% confidence interval (3931.8, 5462.5) diff mean 4706 U=0.000
There is no difference by U-Test mobi_reddit new (133490) is 1.008 times old (132410) 99.50% confidence interval (329.3, 1909.6) diff mean 1080 U=0.713
There is no difference by U-Test mobi_slashdot new (358251) is 0.995 times old (359907) 99.50% confidence interval (-5241.2, 1272.6) diff mean -1656 U=0.625
There is a difference mobi_techcrunch new (188982) is 1.030 times old (183523) 99.50% confidence interval (4760.0, 6097.0) diff mean 5459 U=0.000
There is a difference mobi_theverge new (228549) is 1.014 times old (225410) 99.50% confidence interval (2037.2, 4305.9) diff mean 3140 U=0.000
There is a difference mobi_wikipedia new (483129) is 1.014 times old (476526) 99.50% confidence interval (3987.1, 9146.5) diff mean 6604 U=0.000
There is no difference by U-Test mobi_youtube new (67864) is 1.011 times old (67095) 99.50% confidence interval (469.8, 1082.4) diff mean 770 U=0.002
There is a difference tabl_digg new (293923) is 1.020 times old (288104) 99.50% confidence interval (4443.6, 7224.9) diff mean 5819 U=0.000
There is a difference tabl_mozilla new (153152) is 1.033 times old (148299) 99.50% confidence interval (4049.9, 5626.6) diff mean 4853 U=0.000
There is a difference tabl_pravda new (233456) is 1.028 times old (227001) 99.50% confidence interval (5133.3, 7708.7) diff mean 6455 U=0.000
There is a difference tabl_worldjournal new (307347) is 1.037 times old (296375) 99.50% confidence interval (6758.1, 15149.7) diff mean 10972 U=0.000

// tested systems are different run 2 / 5-samples 30-trials
There is no difference by U-Test desk_chalkboard new (5082344) is 1.006 times old (5051965) 99.50% confidence interval (-26843.1, 86485.9) diff mean 30378 U=0.063
There is a difference desk_cnn new (190292) is 1.024 times old (185872) 99.50% confidence interval (2782.7, 6051.4) diff mean 4419 U=0.000
There is no difference by U-Test desk_css3gradients new (157321) is 1.004 times old (156744) 99.50% confidence interval (-276.2, 1410.8) diff mean 578 U=0.321
There is no difference by U-Test desk_ebay new (84218) is 0.998 times old (84395) 99.50% confidence interval (-520.6, 148.5) diff mean -177 U=0.630
There is a difference desk_espn new (179212) is 1.031 times old (173773) 99.50% confidence interval (3744.4, 7072.0) diff mean 5438 U=0.000
There is no difference by U-Test desk_facebook new (461756) is 1.013 times old (455905) 99.50% confidence interval (2207.9, 9664.5) diff mean 5851 U=0.030
There is a difference desk_gmail new (503968) is 1.026 times old (491344) 99.50% confidence interval (8680.8, 16437.6) diff mean 12624 U=0.000
There is no difference by U-Test desk_googlecalendar new (554604) is 1.017 times old (545159) 99.50% confidence interval (4264.4, 14678.5) diff mean 9445 U=0.002
There is no difference by U-Test desk_googledocs new (473560) is 1.009 times old (469402) 99.50% confidence interval (-265.0, 8815.3) diff mean 4159 U=0.005
There is no difference by U-Test desk_googleimagesearch new (485676) is 1.021 times old (475827) 99.50% confidence interval (4920.0, 14538.1) diff mean 9848 U=0.001
There is no difference by U-Test desk_googlesearch new (302493) is 1.020 times old (296425) 99.50% confidence interval (3334.5, 8895.7) diff mean 6068 U=0.015
There is no difference by U-Test desk_googlespreadsheet new (3051546) is 1.000 times old (3051292) 99.50% confidence interval (-23851.3, 26444.9) diff mean 254 U=0.267
There is no difference by U-Test desk_linkedin new (1125550) is 1.017 times old (1107260) 99.50% confidence interval (7246.6, 28442.3) diff mean 18290 U=0.007
There is no difference by U-Test desk_mapsvg new (2910647) is 0.990 times old (2939267) 99.50% confidence interval (-57661.3, 1452.8) diff mean -28620 U=0.055
There is no difference by confidence interval desk_nytimes new (238573) is 1.018 times old (234342) 99.50% confidence interval (-3033.6, 11656.9) diff mean 4231 U=0.000
There is a difference desk_pokemonwiki new (1282481) is 1.017 times old (1260570) 99.50% confidence interval (16149.4, 27921.5) diff mean 21911 U=0.000
There is no difference by U-Test desk_samoasvg new (2030674) is 0.987 times old (2056569) 99.50% confidence interval (-37645.1, -14770.7) diff mean -25896 U=0.027
There is no difference by U-Test desk_theverge new (126174) is 1.004 times old (125689) 99.50% confidence interval (-152.1, 1116.8) diff mean 486 U=0.981
There is no difference by U-Test desk_tiger8svg new (6115317) is 1.005 times old (6086223) 99.50% confidence interval (1684.7, 57101.3) diff mean 29094 U=0.923
There is no difference by U-Test desk_tigersvg new (2892186) is 1.003 times old (2884011) 99.50% confidence interval (-11358.6, 27807.0) diff mean 8176 U=0.186
There is a difference desk_twitter new (602405) is 1.021 times old (590068) 99.50% confidence interval (8579.7, 16215.2) diff mean 12337 U=0.000
There is a difference desk_weather new (368299) is 1.015 times old (363031) 99.50% confidence interval (3180.4, 7500.8) diff mean 5268 U=0.000
There is no difference by U-Test desk_wikipedia new (1017810) is 1.009 times old (1008731) 99.50% confidence interval (4294.8, 13965.3) diff mean 9078 U=0.003
There is a difference desk_wowwiki new (229909) is 1.023 times old (224724) 99.50% confidence interval (3623.3, 6892.5) diff mean 5185 U=0.000
There is a difference desk_yahooanswers new (393135) is 1.026 times old (383003) 99.50% confidence interval (7901.4, 12391.5) diff mean 10132 U=0.000
There is no difference by U-Test desk_yahoosports new (638180) is 1.001 times old (637845) 99.50% confidence interval (-4676.0, 4693.6) diff mean 335 U=0.119
There is no difference by U-Test desk_ynevsvg new (15924444) is 1.006 times old (15829802) 99.50% confidence interval (6873.9, 182940.5) diff mean 94642 U=0.037
There is no difference by U-Test desk_youtube new (80030) is 1.007 times old (79459) 99.50% confidence interval (83.0, 1080.7) diff mean 571 U=0.029
There is a difference mobi_amazon new (282484) is 1.030 times old (274314) 99.50% confidence interval (6574.4, 9785.8) diff mean 8170 U=0.000
There is a difference mobi_baidu new (273227) is 1.025 times old (266482) 99.50% confidence interval (5277.6, 8231.9) diff mean 6745 U=0.000
There is no difference by U-Test mobi_booking new (533493) is 1.011 times old (527567) 99.50% confidence interval (43.2, 12224.2) diff mean 5927 U=0.373
There is no difference by U-Test mobi_capitalvolkswagen new (301662) is 1.022 times old (295073) 99.50% confidence interval (3972.1, 9195.3) diff mean 6589 U=0.334
There is a difference mobi_cnn new (182945) is 1.036 times old (176661) 99.50% confidence interval (4899.4, 7504.2) diff mean 6285 U=0.000
There is no difference by U-Test mobi_cnnarticle new (93006) is 1.004 times old (92673) 99.50% confidence interval (-2551.9, 3055.9) diff mean 334 U=0.003
There is no difference by U-Test mobi_deviantart new (446008) is 1.023 times old (435839) 99.50% confidence interval (7471.7, 12996.3) diff mean 10168 U=0.001
There is no difference by U-Test mobi_facebook new (146518) is 0.998 times old (146821) 99.50% confidence interval (-1224.5, 711.2) diff mean -303 U=0.835
There is a difference mobi_forecastio new (343768) is 1.030 times old (333875) 99.50% confidence interval (7586.3, 12200.1) diff mean 9893 U=0.000
There is a difference mobi_googlenews new (139414) is 1.022 times old (136457) 99.50% confidence interval (2046.6, 3792.0) diff mean 2957 U=0.000
There is a difference mobi_googlesearch new (173481) is 1.023 times old (169502) 99.50% confidence interval (2585.1, 5342.7) diff mean 3979 U=0.000
There is no difference by U-Test mobi_reddit new (132359) is 1.000 times old (132392) 99.50% confidence interval (-625.5, 565.5) diff mean -33 U=0.056
There is no difference by U-Test mobi_slashdot new (355703) is 0.995 times old (357411) 99.50% confidence interval (-5276.0, 1868.5) diff mean -1707 U=0.517
There is a difference mobi_techcrunch new (192183) is 1.029 times old (186776) 99.50% confidence interval (4387.8, 6399.4) diff mean 5406 U=0.000
There is no difference by U-Test mobi_theverge new (234978) is 1.008 times old (233151) 99.50% confidence interval (164.6, 3476.1) diff mean 1828 U=0.008
There is a difference mobi_wikipedia new (491246) is 1.014 times old (484681) 99.50% confidence interval (3241.6, 9755.8) diff mean 6565 U=0.000
There is no difference by U-Test mobi_youtube new (66758) is 0.997 times old (66939) 99.50% confidence interval (-441.5, 81.9) diff mean -181 U=0.034
There is a difference tabl_digg new (298993) is 1.017 times old (293882) 99.50% confidence interval (3895.0, 6279.7) diff mean 5110 U=0.000
There is a difference tabl_mozilla new (155444) is 1.027 times old (151397) 99.50% confidence interval (3046.5, 5019.0) diff mean 4046 U=0.000
There is a difference tabl_pravda new (240183) is 1.036 times old (231756) 99.50% confidence interval (6707.5, 10026.3) diff mean 8427 U=0.000
There is a difference tabl_worldjournal new (300245) is 1.024 times old (293264) 99.50% confidence interval (4685.3, 9205.7) diff mean 6982 U=0.000

// tested systems are different run 3 / 5-samples 100-trials
There is no difference by U-Test desk_chalkboard new (4833837) is 1.004 times old (4813334) 99.50% confidence interval (4919.6, 36041.9) diff mean 20503 U=0.026
There is a difference desk_cnn new (180428) is 1.034 times old (174503) 99.50% confidence interval (5225.3, 6632.5) diff mean 5924 U=0.000
There is no difference by U-Test desk_css3gradients new (154831) is 0.997 times old (155250) 99.50% confidence interval (-1123.0, 260.3) diff mean -419 U=0.212
There is no difference by U-Test desk_ebay new (83734) is 0.999 times old (83784) 99.50% confidence interval (-326.5, 188.6) diff mean -50 U=0.640
There is a difference desk_espn new (166248) is 1.019 times old (163138) 99.50% confidence interval (2614.5, 3646.3) diff mean 3110 U=0.000
There is a difference desk_facebook new (435484) is 1.015 times old (429210) 99.50% confidence interval (4994.3, 7564.6) diff mean 6274 U=0.000
There is a difference desk_gmail new (464605) is 1.017 times old (456923) 99.50% confidence interval (6526.3, 8850.5) diff mean 7682 U=0.000
There is a difference desk_googlecalendar new (512207) is 1.017 times old (503597) 99.50% confidence interval (7156.6, 10020.3) diff mean 8609 U=0.000
There is no difference by U-Test desk_googledocs new (442495) is 1.000 times old (442377) 99.50% confidence interval (-994.5, 1215.2) diff mean 118 U=0.057
There is a difference desk_googleimagesearch new (452048) is 1.010 times old (447693) 99.50% confidence interval (3074.3, 5668.0) diff mean 4355 U=0.000
There is a difference desk_googlesearch new (285339) is 1.020 times old (279620) 99.50% confidence interval (4973.2, 6456.0) diff mean 5720 U=0.000
There is a difference desk_googlespreadsheet new (2854104) is 1.005 times old (2839055) 99.50% confidence interval (8703.3, 21393.3) diff mean 15049 U=0.000
There is a difference desk_linkedin new (1066281) is 1.016 times old (1049207) 99.50% confidence interval (13437.4, 20549.9) diff mean 17074 U=0.000
There is no difference by U-Test desk_mapsvg new (2821852) is 0.999 times old (2825957) 99.50% confidence interval (-9389.0, 1175.8) diff mean -4105 U=0.316
There is a difference desk_nytimes new (229147) is 1.023 times old (223952) 99.50% confidence interval (4218.7, 6164.5) diff mean 5195 U=0.000
There is a difference desk_pokemonwiki new (1278507) is 1.011 times old (1264163) 99.50% confidence interval (10458.3, 18285.5) diff mean 14344 U=0.000
There is a difference desk_samoasvg new (2015936) is 0.994 times old (2028853) 99.50% confidence interval (-18693.4, -7089.1) diff mean -12917 U=0.001
There is a difference desk_theverge new (127765) is 1.016 times old (125763) 99.50% confidence interval (1555.4, 2459.2) diff mean 2002 U=0.000
There is a difference desk_tiger8svg new (6153066) is 1.003 times old (6131679) 99.50% confidence interval (3845.3, 38939.1) diff mean 21387 U=0.001
There is no difference by U-Test desk_tigersvg new (2862811) is 1.005 times old (2848652) 99.50% confidence interval (6114.7, 22482.1) diff mean 14159 U=0.190
There is a difference desk_twitter new (594139) is 1.021 times old (581726) 99.50% confidence interval (10441.2, 14428.7) diff mean 12413 U=0.000

// tested systems are different run 4 / 5-samples 100-trials
There is no difference by U-Test desk_chalkboard new (4815789) is 1.003 times old (4802395) 99.50% confidence interval (-1742.7, 28245.2) diff mean 13394 U=0.423
There is a difference desk_cnn new (179365) is 1.026 times old (174819) 99.50% confidence interval (3838.2, 5246.2) diff mean 4546 U=0.000
There is no difference by U-Test desk_css3gradients new (154167) is 1.002 times old (153841) 99.50% confidence interval (-1137.0, 1800.5) diff mean 326 U=0.075
There is no difference by U-Test desk_ebay new (83627) is 0.996 times old (83922) 99.50% confidence interval (-766.7, 131.8) diff mean -295 U=0.006
There is no difference by U-Test desk_espn new (171240) is 1.012 times old (169145) 99.50% confidence interval (1297.2, 2895.0) diff mean 2095 U=0.109
There is a difference desk_facebook new (440057) is 1.013 times old (434403) 99.50% confidence interval (4128.6, 7191.5) diff mean 5654 U=0.000
There is a difference desk_gmail new (469733) is 1.013 times old (463726) 99.50% confidence interval (4342.4, 7600.8) diff mean 6007 U=0.000
There is no difference by U-Test desk_googlecalendar new (531598) is 1.014 times old (524479) 99.50% confidence interval (3954.9, 10337.3) diff mean 7120 U=0.061
There is no difference by U-Test desk_googledocs new (452751) is 1.006 times old (450125) 99.50% confidence interval (581.0, 4679.5) diff mean 2626 U=0.621
There is a difference desk_googleimagesearch new (458072) is 1.016 times old (450746) 99.50% confidence interval (6004.1, 8683.1) diff mean 7326 U=0.000
There is a difference desk_googlesearch new (289873) is 1.017 times old (285114) 99.50% confidence interval (3853.8, 5700.1) diff mean 4759 U=0.000
There is no difference by U-Test desk_googlespreadsheet new (3044355) is 0.999 times old (3047610) 99.50% confidence interval (-16196.3, 9660.2) diff mean -3254 U=0.892
There is a difference desk_linkedin new (1122365) is 1.011 times old (1110280) 99.50% confidence interval (6374.5, 18094.4) diff mean 12085 U=0.000
There is no difference by U-Test desk_mapsvg new (2981696) is 1.001 times old (2980000) 99.50% confidence interval (-11886.2, 15387.5) diff mean 1697 U=0.284
There is a difference desk_nytimes new (246764) is 1.029 times old (239759) 99.50% confidence interval (5559.0, 8483.4) diff mean 7005 U=0.000
There is a difference desk_pokemonwiki new (1348258) is 1.016 times old (1327235) 99.50% confidence interval (13984.4, 28032.1) diff mean 21022 U=0.000
There is no difference by U-Test desk_samoasvg new (2146962) is 0.992 times old (2163630) 99.50% confidence interval (-26569.1, -7095.2) diff mean -16668 U=0.125
There is no difference by U-Test desk_theverge new (133556) is 1.013 times old (131877) 99.50% confidence interval (967.8, 2404.0) diff mean 1679 U=0.015
There is no difference by U-Test desk_tiger8svg new (6415778) is 1.005 times old (6385627) 99.50% confidence interval (485.7, 61085.5) diff mean 30151 U=0.931
There is no difference by U-Test desk_tigersvg new (2985604) is 1.007 times old (2965830) 99.50% confidence interval (4005.1, 35583.6) diff mean 19775 U=0.023
There is a difference desk_twitter new (621864) is 1.027 times old (605751) 99.50% confidence interval (13498.4, 18669.2) diff mean 16113 U=0.000
There is a difference desk_weather new (381823) is 1.026 times old (372010) 99.50% confidence interval (8271.6, 11293.3) diff mean 9813 U=0.000
There is no difference by U-Test desk_wikipedia new (1054910) is 1.007 times old (1047193) 99.50% confidence interval (2345.6, 13019.1) diff mean 7717 U=0.002
There is a difference desk_wowwiki new (236858) is 1.016 times old (233150) 99.50% confidence interval (2607.0, 4793.3) diff mean 3708 U=0.000
There is a difference desk_yahooanswers new (406901) is 1.026 times old (396588) 99.50% confidence interval (8702.2, 11878.2) diff mean 10313 U=0.000
There is no difference by U-Test desk_yahoosports new (656543) is 1.008 times old (651448) 99.50% confidence interval (1908.8, 8323.5) diff mean 5095 U=0.021
There is no difference by U-Test desk_ynevsvg new (17045035) is 1.003 times old (16991213) 99.50% confidence interval (-13992.9, 123284.8) diff mean 53822 U=0.330


*/