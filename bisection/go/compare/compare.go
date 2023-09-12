package compare

import (
	"fmt"
	"math"

	bpb "go.skia.org/infra/bisection/go/proto"
)

// the low threshold by default is p = 0.01
const lowThreshold = 0.01

// TODO(b/299537769) the high threshold will need to be replaced by the high
// threshold function here:
// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/compare/thresholds.py;drc=511350a8196b221b1e3949030a92e9d4e7c705b8;l=26
func getHighThreshold(performance_mode bool) float64 {
	if performance_mode {
		return 0.99
	}
	return 0.66
}

// TODO(b/299537769) this stats function will need to be eventually replaced by the KS test
// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/compare/kolmogorov_smirnov.py
// for now, create dummy KS_test to get the ball rolling
func kolmogorovSmirnov(a []float64, b []float64) (float64, error) {
	return 0.05, nil
}

// TODO(b/299537769) this stats function will need to be eventually replaced by the KS test
// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/compare/kolmogorov_smirnov.py
// for now, create dummy KS_test to get the ball rolling
func mannWhitneyU(a []float64, b []float64) (float64, error) {
	return 0.15, nil
}

// Compares if samples collected from two CLs perform significantly different
// according to the Kolmogorov Smirnov test and the Mann Whitney U test.
func CompareSamples(
	req *bpb.GetPerformanceDifferenceRequest,
) (
	*bpb.GetPerformanceDifferenceResponse, error,
) {
	a := req.GetSamplesA()
	b := req.GetSamplesB()

	if len(a) == 0 || len(b) == 0 {
		return nil, fmt.Errorf("Commit(s) has sample size of 0. Sample size of A %v and sample size of B %v", len(a), len(b))
	}

	ks_pval, err := kolmogorovSmirnov(a, b)
	if err != nil {
		return nil, fmt.Errorf("Kolmogorov Smirnov statistical test failed with err %v", err)
	}
	mwu_pval, err := mannWhitneyU(a, b)
	if err != nil {
		return nil, fmt.Errorf("Mann Whitney U statistical test failed with err %v", err)
	}
	pval := math.Min(ks_pval, mwu_pval)

	high := getHighThreshold(true)

	resp := &bpb.GetPerformanceDifferenceResponse{
		State:         bpb.State_UNKNOWN,
		PValue:        pval,
		LowThreshold:  lowThreshold,
		HighThreshold: high,
	}

	if pval <= lowThreshold {
		resp.State = bpb.State_DIFFERENT
	} else if pval >= high {
		resp.State = bpb.State_SAME
	}

	return resp, nil
}
