package main

import (
	"fmt"
	"github.com/aclements/go-moremath/stats"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func must(err error) {
	if err != nil {
		fmt.Printf("err: %s\n", err)
		panic(err)
	}
}

var samplesPerRun int = 10
var trials int = 50

type Stats struct {
	count float64
	mean float64
	m2 float64
}

var commonFlags = []string{"--config", "gl", "-v", "--samples", strconv.Itoa(samplesPerRun), "--match"}
var outputCheck = regexp.MustCompile("^curr/maxrss\\s+loops\\s+min\\s+median\\s+mean\\s+max\\s+stddev\\s+samples\\s+config\\s+bench.*$")

func runNanobench(binPath, skp string, samples []float64) string {
	bin, pathError := exec.LookPath(binPath)
	must(pathError)
	cmd := &exec.Cmd {
		Path: bin,
		Args: append([]string{bin}, append(commonFlags, skp)...),
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

func p(s Stats) (float64, float64, float64) {
	return s.count, s.mean, s.m2
}

func addSample(old Stats, sample float64) Stats {
	var count, mean, m2 float64
	count, mean, m2 = p(old)
	count += 1
	delta := sample - mean
	mean +=  delta / count

	m2 += delta * (sample - mean)
	return Stats{count, mean, m2}
}


// print mbar - 1.96 * stddev / sqrt(m0), mbar + 1.96 * stddev / sqrt(m0)
func ninetyFiveConfidenceInterval(s Stats) (float64, float64) {
	count, mean, m2 := p(s)
	stdDev := math.Sqrt(m2/(count - 1))
	diff := 3.391 * stdDev / math.Sqrt(count)
	return mean - diff, mean + diff
}

func statsToString(s Stats) string {
	count, mean, m2 := p(s)
	if count <= 0 {
		panic("error count <= 0")
	}
	if count > 1 {
		stddev := math.Sqrt(m2/(count - 1))
		percent := 100 * math.Abs(stddev / mean)
		low, high := ninetyFiveConfidenceInterval(s)
		return fmt.Sprintf("%0.2f ± %2.0f%% 99.95%% (%2.0f, %2.0f)", mean, percent, low, high)
	} else {
		return fmt.Sprintf("%0.2f", mean)
	}
}

func compareStats(oldStats, newStats, diffStats Stats) string {
	if oldStats.count == 1 {
		return fmt.Sprintf("new is %0.2f times old", newStats.mean / oldStats.mean)
	} else {
		oldLow, oldHigh := ninetyFiveConfidenceInterval(oldStats)
		newLow, newHigh := ninetyFiveConfidenceInterval(newStats)
		if (newHigh < oldLow || oldHigh < newLow) {
			return fmt.Sprintf("new is %0.4f times old", newStats.mean/oldStats.mean)
		} else {
			return "has no difference"
		}
	}
}

func skpFromPath(path string) string {
	_, file := filepath.Split(path)
	return strings.TrimSuffix(file, filepath.Ext(file))
}

func main() {

	skpPaths, err := filepath.Glob("./skps/*.skp")
	if err != nil {
		panic(err)
	}

	/*
	desk_cnn new (176105.850000) is 1.017 times old (173189.791667) p=0.000
	desk_googledocs new (449473.358333) is 1.012 times old (444127.350000) p=0.000
	desk_googlesearch new (285980.683333) is 1.013 times old (282356.566667) p=0.000
	desk_mapsvg new (2830132.250000) is 0.989 times old (2860349.941667) p=0.000
	desk_nytimes new (228535.983333) is 1.020 times old (223991.125000) p=0.000
	desk_pokemonwiki new (1282339.566667) is 1.008 times old (1272165.933333) p=0.000
	desk_theverge new (128043.225000) is 1.010 times old (126725.566667) p=0.000
	desk_weather new (361517.716667) is 1.009 times old (358390.358333) p=0.000
	desk_wikipedia new (1015888.541667) is 1.015 times old (1000762.433333) p=0.000
	desk_wowwiki new (226360.708333) is 1.011 times old (223873.466667) p=0.000
	desk_yahooanswers new (388464.683333) is 1.015 times old (382906.375000) p=0.000
	mobi_amazon new (278609.366667) is 1.011 times old (275533.508333) p=0.000
	mobi_baidu new (271110.316667) is 1.008 times old (268859.750000) p=0.000
	mobi_capitalvolkswagen new (294703.250000) is 1.023 times old (287982.275000) p=0.000
	mobi_cnn new (177145.491667) is 1.012 times old (175065.991667) p=0.000
	mobi_googlenews new (137367.766667) is 1.013 times old (135548.275000) p=0.000
	mobi_techcrunch new (186406.866667) is 1.019 times old (182903.833333) p=0.000
	mobi_theverge new (228585.666667) is 1.013 times old (225547.141667) p=0.000
	mobi_youtube new (67035.266667) is 0.994 times old (67411.825000) p=0.000
	tabl_digg new (293614.733333) is 1.015 times old (289188.183333) p=0.000
	tabl_mozilla new (152822.333333) is 1.019 times old (149906.591667) p=0.000
	tabl_pravda new (233194.608333) is 1.017 times old (229354.975000) p=0.000
	tabl_worldjournal new (292698.475000) is 1.015 times old (288464.241667) p=0.000
	 */
	skpPaths = []string{"skps/desk_nytimes.skp", "skps/desk_cnn.skp", "skps/tabl_mozilla.skp", "skps/mobi_techcrunch.skp"}

	totalSamples := trials * samplesPerRun
	oldSamples := make([]float64, totalSamples)
	newSamples := make([]float64, totalSamples)
	for _, skpPath := range skpPaths {
		var benchName string = skpFromPath(skpPath)

		if benchName == "desk_carsvg" {
			continue
		}

		for trial := 0; trial < trials/2; trial++ {
			startSample := trial * samplesPerRun
			endSample := startSample + samplesPerRun
			runNanobench(os.Args[1], benchName, oldSamples[startSample:endSample])
			runNanobench(os.Args[2], benchName, newSamples[startSample:endSample])
		}

		for trial := trials/2; trial < trials; trial++ {
			startSample := trial * samplesPerRun
			endSample := startSample + samplesPerRun
			runNanobench(os.Args[2], benchName, newSamples[startSample:endSample])
			runNanobench(os.Args[1], benchName, oldSamples[startSample:endSample])
		}

		u, err := stats.MannWhitneyUTest(oldSamples, newSamples, stats.LocationDiffers)
		must(err)

		sort.Float64s(oldSamples)
		sort.Float64s(newSamples)

		lastQuartile := (trials / 4) * samplesPerRun

		lowestQuartileOld := oldSamples[:lastQuartile]
		lowestQuartileNew := newSamples[:lastQuartile]

		if u.P > 0.001 {
			fmt.Println(benchName, "there is no difference p=", u.P)
		} else {
			oldMean := stats.Mean(lowestQuartileOld)
			newMean := stats.Mean(lowestQuartileNew)
			fmt.Printf("%s new (%f) is %0.3f times old (%f) p=%0.3f\n",
				benchName, newMean, newMean / oldMean, oldMean, u.P)
		}
	}

	os.Exit(0)
}
// Release Master
// desk_nytimes.skp_1 new is 0.9835 times old 95% confidence -4916.96 to -2290.86 218713.47 ± 34% 215109.56 ± 34% -3603.91 ± 588%

// Master Release
// desk_nytimes.skp_1 new is 1.0032 times old 95% confidence -803.41 to 2178.49 217127.70 ± 34% 217815.24 ± 34% 687.54 ± 3499%
// desk_nytimes.skp_1 new is 1.0106 times old 95% confidence 923.97 to 3701.42 218945.61 ± 34% 221258.30 ± 34% 2312.70 ± 969%

// Master Master
// desk_nytimes.skp_1 new is 0.9891 times old 95% confidence -4439.71 to -403.95 221786.37 ± 36% 219364.54 ± 34% -2421.83 ± 1344%
// desk_nytimes.skp_1 new is 1.0030 times old 95% confidence -738.62 to 2026.74 216702.10 ± 34% 217346.16 ± 34% 644.06 ± 3464%


// Master Master - only capture sampltes
// new is 1.0102 times old 95% confidence -32.28 to 4916.90 239757.87 ±  7% 242200.18 ±  8% 2442.31 ± 1034%
//  new is 1.0179 times old 95% confidence 1897.01 to 6835.27 243401.87 ±  8% 247768.01 ±  9% 4366.14 ± 577%
//  new is 1.0067 times old 95% confidence -691.51 to 3913.15 241181.36 ±  7% 242792.18 ±  8% 1610.82 ± 1458%

// Master Master - lowest quartile
// new is 1.0019 times old 95% confidence 353.17 to 486.01 224679.83 ±  1% 225099.42 ±  1% 419.59 ± 81%
// new is 0.9970 times old 95% confidence -786.24 to -591.72 226398.14 ±  1% 225709.16 ±  1% -688.98 ± 72%

// Master Master - lowest quartile - confidence intervals on old/new
//  new is 0.9988 times old with a 99.95% confidence -393.37 to -140.67 225339.32 ±  1% 99.95% (224678, 226001) 225072.30 ±  1% 99.95% (224313, 225832) -267.02 ± 140% 99.95% (-393, -141)
//  new is 0.9948 times old with a 99.95% confidence -1258.98 to -1077.96 222888.32 ±  1% 99.95% (222220, 223557) 221719.85 ±  1% 99.95% (221119, 222321) -1168.47 ± 23% 99.95% (-1259, -1078)

// Master Release - lowest quartile - confidence intervals on old/new
//  new is 1.0121 times old with a 99.95% confidence 2479.26 to 2892.80 222818.72 ±  1% 99.95% (222067, 223570) 225504.75 ±  1% 99.95% (224944, 226065) 2686.03 ± 23% 99.95% (2479, 2893)
//  new is 1.0146 times old with a 99.95% confidence 3002.87 to 3480.23 222492.48 ±  1% 99.95% (221759, 223226) 225734.03 ±  1% 99.95% (225232, 226236) 3241.55 ± 22% 99.95% (3003, 3480)

/*
{0 0 0}
desk_carsvg has no difference 10207672.02 ±  2% 99.95% (10130561, 10284784) 10202694.24 ±  2% 99.95% (10128662, 10276726) -4977.78 ± 845% 99.95% (-25141, 15185)
{0 0 0}
desk_chalkboard has no difference 4730065.92 ±  1% 99.95% (4714147, 4745985) 4731712.20 ±  1% 99.95% (4712928, 4750496) 1646.28 ± 409% 99.95% (-1584, 4877)
{0 0 0}
desk_cnn has no difference 173561.10 ±  1% 99.95% (172734, 174389) 174082.62 ±  1% 99.95% (173342, 174823) 521.52 ± 95% 99.95% (284, 759)
{0 0 0}
desk_css3gradients has no difference 155293.90 ±  1% 99.95% (154621, 155967) 155148.90 ±  1% 99.95% (154472, 155826) -145.00 ± 369% 99.95% (-402, 112)
{0 0 0}
desk_ebay has no difference 83883.28 ±  1% 99.95% (83660, 84107) 83775.92 ±  0% 99.95% (83604, 83948) -107.36 ± 118% 99.95% (-168, -46)
{0 0 0}
desk_espn has no difference 164144.70 ±  1% 99.95% (163348, 164941) 163514.76 ±  1% 99.95% (162820, 164210) -629.94 ± 76% 99.95% (-859, -401)
{0 0 0}
desk_facebook has no difference 429376.30 ±  1% 99.95% (427640, 431113) 427685.48 ±  1% 99.95% (426338, 429033) -1690.82 ± 53% 99.95% (-2117, -1264)
{0 0 0}
desk_gmail has no difference 462850.62 ±  1% 99.95% (460978, 464723) 462257.94 ±  1% 99.95% (460668, 463848) -592.68 ± 152% 99.95% (-1025, -161)
{0 0 0}
desk_googlecalendar has no difference 502036.04 ±  1% 99.95% (500489, 503583) 501959.18 ±  1% 99.95% (500292, 503626) -76.86 ± 742% 99.95% (-350, 197)
{0 0 0}
desk_googledocs has no difference 439483.52 ±  1% 99.95% (438309, 440658) 439271.34 ±  1% 99.95% (437806, 440737) -212.18 ± 319% 99.95% (-537, 113)
{0 0 0}
desk_googleimagesearch new is 1.0088 times old 444412.94 ±  1% 99.95% (442475, 446351) 448314.96 ±  1% 99.95% (446435, 450195) 3902.02 ± 22% 99.95% (3486, 4318)
{0 0 0}
desk_googlesearch has no difference 280419.80 ±  1% 99.95% (279599, 281241) 281083.62 ±  1% 99.95% (279934, 282233) 663.82 ± 122% 99.95% (274, 1054)
{0 0 0}
desk_googlespreadsheet has no difference 2745400.36 ±  0% 99.95% (2739029, 2751772) 2746710.88 ±  1% 99.95% (2739744, 2753678) 1310.52 ± 248% 99.95% (-251, 2872)
{0 0 0}
desk_linkedin has no difference 1044482.32 ±  1% 99.95% (1040335, 1048630) 1050597.62 ±  1% 99.95% (1047493, 1053702) 6115.30 ± 42% 99.95% (4878, 7353)
{0 0 0}
desk_mapsvg has no difference 2810624.20 ±  0% 99.95% (2805498, 2815750) 2811586.48 ±  0% 99.95% (2805534, 2817639) 962.28 ± 375% 99.95% (-770, 2695)
{0 0 0}
desk_nytimes has no difference 219869.78 ±  1% 99.95% (219081, 220659) 220747.70 ±  1% 99.95% (219980, 221516) 877.92 ± 54% 99.95% (651, 1105)
{0 0 0}
desk_pokemonwiki has no difference 1263048.20 ±  1% 99.95% (1258900, 1267197) 1263491.56 ±  1% 99.95% (1258437, 1268546) 443.36 ± 872% 99.95% (-1411, 2298)
{0 0 0}
desk_samoasvg new is 0.9934 times old 2004390.22 ±  1% 99.95% (1998907, 2009873) 1991191.98 ±  1% 99.95% (1985028, 1997356) -13198.24 ± 18% 99.95% (-14330, -12066)
{0 0 0}
desk_theverge has no difference 125370.02 ±  1% 99.95% (124887, 125853) 124596.02 ±  1% 99.95% (124246, 124946) -774.00 ± 53% 99.95% (-970, -578)
{0 0 0}
desk_tiger8svg has no difference 5987782.68 ±  1% 99.95% (5970912, 6004654) 6002190.92 ±  0% 99.95% (5988393, 6015989) 14408.24 ± 80% 99.95% (8848, 19969)
{0 0 0}
desk_tigersvg has no difference 2759686.36 ±  1% 99.95% (2751610, 2767763) 2764757.92 ±  1% 99.95% (2754788, 2774728) 5071.56 ± 115% 99.95% (2284, 7859)
{0 0 0}
desk_twitter has no difference 572472.36 ±  1% 99.95% (570356, 574588) 574164.60 ±  1% 99.95% (571938, 576391) 1692.24 ± 34% 99.95% (1413, 1971)
{0 0 0}
desk_weather new is 1.0070 times old 348177.46 ±  1% 99.95% (347189, 349166) 350618.24 ±  1% 99.95% (349630, 351607) 2440.78 ± 15% 99.95% (2265, 2617)
{0 0 0}
desk_wikipedia has no difference 986703.54 ±  1% 99.95% (983568, 989839) 987410.82 ±  0% 99.95% (985328, 989493) 707.28 ± 377% 99.95% (-572, 1987)
{0 0 0}
desk_wowwiki has no difference 221007.02 ±  1% 99.95% (219557, 222457) 222950.90 ±  1% 99.95% (222006, 223896) 1943.88 ± 64% 99.95% (1351, 2537)
{0 0 0}
desk_yahooanswers has no difference 377087.80 ±  1% 99.95% (376022, 378153) 376954.32 ±  1% 99.95% (375979, 377930) -133.48 ± 300% 99.95% (-326, 59)
{0 0 0}
desk_yahoosports has no difference 629290.44 ±  1% 99.95% (627407, 631173) 633436.04 ±  1% 99.95% (631065, 635807) 4145.60 ± 32% 99.95% (3503, 4788)
{0 0 0}
desk_ynevsvg has no difference 15251521.60 ±  1% 99.95% (15212650, 15290393) 15193632.22 ±  1% 99.95% (15144392, 15242873) -57889.38 ± 46% 99.95% (-70750, -45029)
{0 0 0}
desk_youtube has no difference 77426.32 ±  1% 99.95% (77215, 77638) 77411.50 ±  1% 99.95% (77192, 77631) -14.82 ± 595% 99.95% (-57, 27)
{0 0 0}
mobi_amazon has no difference 270139.74 ±  1% 99.95% (269395, 270885) 270009.08 ±  1% 99.95% (269168, 270850) -130.66 ± 345% 99.95% (-347, 85)
{0 0 0}
mobi_baidu has no difference 261897.82 ±  1% 99.95% (261029, 262767) 261438.34 ±  1% 99.95% (260537, 262340) -459.48 ± 122% 99.95% (-729, -190)
{0 0 0}
mobi_booking has no difference 506159.02 ±  1% 99.95% (504878, 507440) 507962.64 ±  1% 99.95% (506005, 509920) 1803.62 ± 87% 99.95% (1049, 2558)
{0 0 0}
mobi_capitalvolkswagen has no difference 281338.66 ±  1% 99.95% (280423, 282254) 280488.50 ±  1% 99.95% (279621, 281356) -850.16 ± 69% 99.95% (-1133, -567)
{0 0 0}
mobi_cnn has no difference 171045.18 ±  1% 99.95% (170557, 171533) 171908.50 ±  1% 99.95% (171440, 172377) 863.32 ± 20% 99.95% (781, 946)
{0 0 0}
mobi_cnnarticle has no difference 96053.92 ±  1% 99.95% (95575, 96533) 96111.06 ±  1% 99.95% (95871, 96351) 57.14 ± 975% 99.95% (-210, 324)
{0 0 0}
mobi_deviantart has no difference 422580.52 ±  1% 99.95% (421415, 423746) 421669.74 ±  1% 99.95% (420288, 423052) -910.78 ± 98% 99.95% (-1337, -485)
{0 0 0}
mobi_facebook has no difference 145306.04 ±  1% 99.95% (144953, 145659) 145216.56 ±  0% 99.95% (144945, 145488) -89.48 ± 282% 99.95% (-210, 31)
{0 0 0}
mobi_forecastio has no difference 328917.50 ±  1% 99.95% (327841, 329994) 329874.64 ±  1% 99.95% (329016, 330733) 957.14 ± 58% 99.95% (689, 1225)
{0 0 0}
mobi_googlenews has no difference 133812.16 ±  1% 99.95% (133427, 134198) 134330.52 ±  1% 99.95% (133959, 134702) 518.36 ± 24% 99.95% (458, 578)
{0 0 0}
mobi_googlesearch has no difference 168138.48 ±  1% 99.95% (167568, 168709) 168851.46 ±  1% 99.95% (168207, 169496) 712.98 ± 32% 99.95% (602, 824)
{0 0 0}
mobi_reddit has no difference 134469.82 ±  0% 99.95% (134283, 134657) 134437.38 ±  0% 99.95% (134270, 134605) -32.44 ± 271% 99.95% (-75, 10)
{0 0 0}
mobi_slashdot has no difference 358140.82 ±  2% 99.95% (354614, 361668) 361128.04 ±  1% 99.95% (359223, 363033) 2987.22 ± 139% 99.95% (992, 4982)
{0 0 0}
mobi_techcrunch has no difference 183471.10 ±  1% 99.95% (182960, 183982) 183722.42 ±  1% 99.95% (183177, 184268) 251.32 ± 101% 99.95% (130, 373)
{0 0 0}
mobi_theverge has no difference 227112.40 ±  1% 99.95% (226040, 228184) 226548.26 ±  1% 99.95% (225671, 227425) -564.14 ± 92% 99.95% (-813, -315)
{0 0 0}
mobi_wikipedia new is 1.0099 times old 474814.76 ±  1% 99.95% (473274, 476356) 479520.38 ±  1% 99.95% (478140, 480901) 4705.62 ± 14% 99.95% (4387, 5024)
{0 0 0}
mobi_youtube has no difference 68460.20 ±  1% 99.95% (68229, 68692) 67930.54 ±  1% 99.95% (67602, 68259) -529.66 ± 43% 99.95% (-639, -420)
{0 0 0}
tabl_digg has no difference 288871.70 ±  1% 99.95% (287945, 289799) 289118.04 ±  1% 99.95% (287827, 290409) 246.34 ± 484% 99.95% (-325, 818)
{0 0 0}
tabl_mozilla has no difference 149543.72 ±  1% 99.95% (149061, 150026) 149146.28 ±  1% 99.95% (148748, 149544) -397.44 ± 54% 99.95% (-500, -295)
{0 0 0}
tabl_pravda has no difference 228478.82 ±  1% 99.95% (227910, 229048) 228075.86 ±  1% 99.95% (227323, 228829) -402.96 ± 103% 99.95% (-603, -203)
{0 0 0}
tabl_worldjournal has no difference 286448.98 ±  1% 99.95% (285630, 287268) 285751.82 ±  1% 99.95% (284994, 286510) -697.16 ± 51% 99.95% (-869, -526)

// Points to - x86

desk_chalkboard there is no difference p= 0.0024471475131699894
desk_cnn new (176105.850000) is 1.017 times old (173189.791667) p=0.000
desk_css3gradients there is no difference p= 0.16883450552749552
desk_ebay there is no difference p= 0.030091496611309384
desk_espn there is no difference p= 0.023371018683649238
desk_facebook there is no difference p= 0.004876329451399375
desk_gmail there is no difference p= 0.5926726229661687
desk_googlecalendar there is no difference p= 0.003562490813404886
desk_googledocs new (449473.358333) is 1.012 times old (444127.350000) p=0.000
desk_googleimagesearch there is no difference p= 0.001885821740901011
desk_googlesearch new (285980.683333) is 1.013 times old (282356.566667) p=0.000
desk_googlespreadsheet there is no difference p= 0.15676632595815337
desk_linkedin there is no difference p= 0.5757507524645976
desk_mapsvg new (2830132.250000) is 0.989 times old (2860349.941667) p=0.000
desk_nytimes new (228535.983333) is 1.020 times old (223991.125000) p=0.000
desk_pokemonwiki new (1282339.566667) is 1.008 times old (1272165.933333) p=0.000
desk_samoasvg there is no difference p= 0.006089218126887452
desk_theverge new (128043.225000) is 1.010 times old (126725.566667) p=0.000
desk_tiger8svg there is no difference p= 0.12386060841900308
desk_tigersvg there is no difference p= 0.019487387236966712
desk_twitter there is no difference p= 0.26926710655670116
desk_weather new (361517.716667) is 1.009 times old (358390.358333) p=0.000
desk_wikipedia new (1015888.541667) is 1.015 times old (1000762.433333) p=0.000
desk_wowwiki new (226360.708333) is 1.011 times old (223873.466667) p=0.000
desk_yahooanswers new (388464.683333) is 1.015 times old (382906.375000) p=0.000
desk_yahoosports there is no difference p= 0.9473600405404741
desk_ynevsvg there is no difference p= 0.30580791930437723
desk_youtube there is no difference p= 0.052667054169648786
mobi_amazon new (278609.366667) is 1.011 times old (275533.508333) p=0.000
mobi_baidu new (271110.316667) is 1.008 times old (268859.750000) p=0.000
mobi_booking there is no difference p= 0.26277278122328207
mobi_capitalvolkswagen new (294703.250000) is 1.023 times old (287982.275000) p=0.000
mobi_cnn new (177145.491667) is 1.012 times old (175065.991667) p=0.000
mobi_cnnarticle there is no difference p= 0.8185726828604165
mobi_deviantart there is no difference p= 0.25002046824662294
mobi_facebook there is no difference p= 0.40874703552526004
mobi_forecastio there is no difference p= 0.019919414311591978
mobi_googlenews new (137367.766667) is 1.013 times old (135548.275000) p=0.000
mobi_googlesearch there is no difference p= 0.03219760567788299
mobi_reddit there is no difference p= 0.466012016757548
mobi_slashdot there is no difference p= 0.6546040912516142
mobi_techcrunch new (186406.866667) is 1.019 times old (182903.833333) p=0.000
mobi_theverge new (228585.666667) is 1.013 times old (225547.141667) p=0.000
mobi_wikipedia there is no difference p= 0.013670022431348674
mobi_youtube new (67035.266667) is 0.994 times old (67411.825000) p=0.000
tabl_digg new (293614.733333) is 1.015 times old (289188.183333) p=0.000
tabl_mozilla new (152822.333333) is 1.019 times old (149906.591667) p=0.000
tabl_pravda new (233194.608333) is 1.017 times old (229354.975000) p=0.000
tabl_worldjournal new (292698.475000) is 1.015 times old (288464.241667) p=0.000

desk_cnn new (176105.850000) is 1.017 times old (173189.791667) p=0.000
desk_googledocs new (449473.358333) is 1.012 times old (444127.350000) p=0.000
desk_googlesearch new (285980.683333) is 1.013 times old (282356.566667) p=0.000
desk_mapsvg new (2830132.250000) is 0.989 times old (2860349.941667) p=0.000
desk_nytimes new (228535.983333) is 1.020 times old (223991.125000) p=0.000
desk_pokemonwiki new (1282339.566667) is 1.008 times old (1272165.933333) p=0.000
desk_theverge new (128043.225000) is 1.010 times old (126725.566667) p=0.000
desk_weather new (361517.716667) is 1.009 times old (358390.358333) p=0.000
desk_wikipedia new (1015888.541667) is 1.015 times old (1000762.433333) p=0.000
desk_wowwiki new (226360.708333) is 1.011 times old (223873.466667) p=0.000
desk_yahooanswers new (388464.683333) is 1.015 times old (382906.375000) p=0.000
mobi_amazon new (278609.366667) is 1.011 times old (275533.508333) p=0.000
mobi_baidu new (271110.316667) is 1.008 times old (268859.750000) p=0.000
mobi_capitalvolkswagen new (294703.250000) is 1.023 times old (287982.275000) p=0.000
mobi_cnn new (177145.491667) is 1.012 times old (175065.991667) p=0.000
mobi_googlenews new (137367.766667) is 1.013 times old (135548.275000) p=0.000
mobi_techcrunch new (186406.866667) is 1.019 times old (182903.833333) p=0.000
mobi_theverge new (228585.666667) is 1.013 times old (225547.141667) p=0.000
mobi_youtube new (67035.266667) is 0.994 times old (67411.825000) p=0.000
tabl_digg new (293614.733333) is 1.015 times old (289188.183333) p=0.000
tabl_mozilla new (152822.333333) is 1.019 times old (149906.591667) p=0.000
tabl_pravda new (233194.608333) is 1.017 times old (229354.975000) p=0.000
tabl_worldjournal new (292698.475000) is 1.015 times old (288464.241667) p=0.000

mobi_capitalvolkswagen new (294096.983333) is 1.018 times old (288962.400000) p=0.000
mobi_capitalvolkswagen new (292100.975000) is 1.017 times old (287283.283333) p=0.000
mobi_capitalvolkswagen new (293273.033333) is 1.019 times old (287810.883333) p=0.000
reverse
mobi_capitalvolkswagen new (287251.358333) is 0.986 times old (291227.950000) p=0.000
mobi_capitalvolkswagen new (286954.625000) is 0.980 times old (292749.516667) p=0.000



mobi_youtube there is no difference p= 0.9365561113267362
mobi_youtube there is no difference p= 0.9998252795316475
//reverse
mobi_youtube there is no difference p= 0.24867043045090376


desk_weather new (357377.441667) is 1.009 times old (354221.683333) p=0.000
// reverse
desk_weather new (355175.283333) is 0.984 times old (361019.750000) p=0.000



// points to - arm64
desk_chalkboard there is no difference p= 0.18910863832835495
desk_cnn there is no difference p= 0.3379343719619836
desk_css3gradients there is no difference p= 0.12136694463945963
desk_ebay there is no difference p= 0.7393309654478297
desk_espn new (383499.000000) is 0.980 times old (391275.766667) p=0.000
desk_facebook new (432132.625000) is 0.990 times old (436599.475000) p=0.000
desk_gmail new (430294.516667) is 0.988 times old (435351.666667) p=0.000
desk_googlecalendar there is no difference p= 0.1449102724408604
desk_googledocs there is no difference p= 0.3317886164835562
desk_googleimagesearch new (512618.300000) is 0.976 times old (525334.450000) p=0.000
desk_googlesearch there is no difference p= 0.44082064320481207
desk_googlespreadsheet there is no difference p= 0.5838447503384538
desk_linkedin there is no difference p= 0.5995046292814012
desk_mapsvg there is no difference p= 0.37822360644427877
desk_micrographygirlsvg there is no difference p= 0.0017229119079054112
desk_nytimes there is no difference p= 0.20083748241032834
desk_pokemonwiki there is no difference p= 0.06808735065382376
desk_samoasvg there is no difference p= 0.6841078077082803
desk_theverge there is no difference p= 0.8930440088914904
desk_tiger8svg there is no difference p= 0.5549433355174336
desk_tigersvg there is no difference p= 0.820360091745814
desk_twitter new (654887.716667) is 0.984 times old (665431.483333) p=0.000
desk_weather there is no difference p= 0.6494746846814743
desk_wikipedia there is no difference p= 0.0033723245531704915
desk_wowwiki there is no difference p= 0.27088392253654825
desk_yahooanswers new (378045.200000) is 0.989 times old (382292.608333) p=0.000
desk_yahoosports there is no difference p= 0.9876826998142934
desk_ynevsvg new (18954810.975000) is 0.997 times old (19018892.966667) p=0.000
desk_youtube there is no difference p= 0.49620120602055184
mobi_amazon there is no difference p= 0.021389517450583817
mobi_baidu there is no difference p= 0.7033476037984079
mobi_booking there is no difference p= 0.012975797768314479
mobi_capitalvolkswagen there is no difference p= 0.001092582697804989
mobi_cnn new (202626.525000) is 0.981 times old (206589.658333) p=0.001
mobi_cnnarticle there is no difference p= 0.37544522063745367
mobi_deviantart there is no difference p= 0.14624332749077396
mobi_facebook there is no difference p= 0.20199635995708487
mobi_forecastio new (414014.050000) is 0.981 times old (422038.058333) p=0.000
mobi_googlenews there is no difference p= 0.26352012548497594
mobi_googlesearch new (151530.100000) is 0.982 times old (154293.108333) p=0.000
mobi_reddit there is no difference p= 0.9062174089828643
mobi_slashdot there is no difference p= 0.9416953678124619
mobi_techcrunch there is no difference p= 0.6658666455826986
mobi_theverge there is no difference p= 0.11477462725861409
mobi_wikipedia there is no difference p= 0.688617005091531
mobi_youtube there is no difference p= 0.621061584500512
tabl_digg there is no difference p= 0.012274917895997994
tabl_mozilla there is no difference p= 0.7733787772944061
tabl_pravda there is no difference p= 0.06174607816035249
tabl_worldjournal new (307712.450000) is 0.987 times old (311649.850000) p=0.000

desk_espn new (383499.000000) is 0.980 times old (391275.766667) p=0.000
desk_facebook new (432132.625000) is 0.990 times old (436599.475000) p=0.000
desk_gmail new (430294.516667) is 0.988 times old (435351.666667) p=0.000
desk_googleimagesearch new (512618.300000) is 0.976 times old (525334.450000) p=0.000
desk_twitter new (654887.716667) is 0.984 times old (665431.483333) p=0.000
desk_yahooanswers new (378045.200000) is 0.989 times old (382292.608333) p=0.000
desk_ynevsvg new (18954810.975000) is 0.997 times old (19018892.966667) p=0.000
mobi_cnn new (202626.525000) is 0.981 times old (206589.658333) p=0.001
mobi_forecastio new (414014.050000) is 0.981 times old (422038.058333) p=0.000
mobi_googlesearch new (151530.100000) is 0.982 times old (154293.108333) p=0.000
tabl_worldjournal new (307712.450000) is 0.987 times old (311649.850000) p=0.000

//draw 10 no cache
desk_nytimes new (318632.500000) is 1.415 times old (225252.108333) p=0.000
desk_cnn new (272666.508333) is 1.579 times old (172683.758333) p=0.000
tabl_mozilla new (303703.566667) is 2.028 times old (149765.658333) p=0.000
mobi_techcrunch new (249549.450000) is 1.355 times old (184107.058333) p=0.000


*/