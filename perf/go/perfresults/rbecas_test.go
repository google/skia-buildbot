package perfresults

import (
	"context"
	"testing"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/stretchr/testify/assert"
)

func Test_RBE_LoadPerfResults_ReturnPerfResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_LoadPerfResults_ReturnPerfResults")

	// CAS Output from: https://chrome-swarming.appspot.com/task?id=68f6c580c2e5d711
	cas := makeCAS("d127f8323a5016001b6d44bdc784a41aacb982909f721878589258d3dfc30616", 752)

	// Load from the same output twice so they can be merged
	pr, err := c.LoadPerfResults(ctx, cas, cas)
	assert.NoError(t, err)
	assert.Contains(t, pr, "speedometer3")

	assert.Len(t, pr["speedometer3"].Histograms, 21)
	assert.Len(t, pr["speedometer3"].GetSampleValues("Charts-chartjs"), 20, "concating two samples should add together")
}

func Test_RBE_FetchPerfDigests_ReturnListDigests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_FetchPerfDigests_ReturnListDigests")

	cas := makeCAS("d127f8323a5016001b6d44bdc784a41aacb982909f721878589258d3dfc30616", 752)
	digests, err := c.fetchPerfDigests(ctx, cas)
	assert.NoError(t, err)

	expected := map[string]digest.Digest{
		"rendering.mobile":            {Hash: "06d37aeeb0d7a2d1040082b2cf17c6caf9d2d8deae8b69e4bcdc58d6f6647be4", Size: 1985714},
		"speedometer2-predictable":    {Hash: "258720d2a653d825d92aff2c1085abe54f270ebce6072d72057b24534a569c88", Size: 26728},
		"system_health.common_mobile": {Hash: "aede2a355ae63200cd11e1791226dc8919d517158d0a623323655e6e75327690", Size: 1271968},
		"speedometer2":                {Hash: "69034a473e9e1a845b2bdb46e0ce660a822ba49f890ca89d5088f97b198291b4", Size: 26553},
		"speedometer3":                {Hash: "0b20a718825dc5805bb3b4d8b2cff4633a900e7cb3a8050164fcac59ceb2c58a", Size: 30817},
		"speedometer3-predictable":    {Hash: "cd3b62c0c75a5690b434bcea50bec753a948a172d6ea27a7eb0c27fa8b3c0326", Size: 31124},
	}

	assert.Subset(t, digests, expected)
}

func Test_RBE_FetchPerfDigests_Empty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_FetchPerfDigests_Empty")

	cas := makeCAS("d8a9ce0076c037b00dc8f75261db1c6811e0d54a39ee243370af1b8df05264f6", 435)
	digests, err := c.fetchPerfDigests(ctx, cas)
	assert.NoError(t, err)
	assert.Len(t, digests, 0)
}

func Test_RBE_FetchPerfDigests_InvalidPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_FetchPerfDigests_InvalidPath")

	cas := makeCAS("ec9d563cd54d4915acf6f894207355af52d6f850ae9734e9274c058b99cb15f7", 1362)
	_, err := c.fetchPerfDigests(ctx, cas)
	assert.ErrorContains(t, err, "perf file location")
}

func Test_RBE_LoadPerfResult_ReturnValid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_LoadPerfResult_ReturnValid")

	cas := makeCAS("d127f8323a5016001b6d44bdc784a41aacb982909f721878589258d3dfc30616", 752)
	digests, err := c.fetchPerfDigests(ctx, cas)
	assert.NoError(t, err)

	pr, err := c.loadPerfResult(ctx, digests["speedometer2"])
	assert.NoError(t, err)
	assert.Len(t, pr.Histograms, 18)

	pr, err = c.loadPerfResult(ctx, digests["speedometer3"])
	assert.NoError(t, err)
	assert.Len(t, pr.Histograms, 21)
}

func Test_RBE_LoadPerfResult_InvalidDigest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newRBEReplay(t, ctx, "chrome-swarming", "RBE_LoadPerfResult_InvalidDigest")

	check_error := func(d digest.Digest) {
		pr, err := c.loadPerfResult(ctx, d)
		assert.Error(t, err)
		assert.Nil(t, pr)
	}

	// it should load a directory but fail at parsing
	check_error(digest.Digest{Hash: "d127f8323a5016001b6d44bdc784a41aacb982909f721878589258d3dfc30616", Size: 752})

	// invalid json file content
	check_error(digest.Digest{Hash: "94f74174df883c2b1e30e26bfe765b9e15fa39473db34f882275b67fa89a579a", Size: 6504})

	// invalid digest
	check_error(digest.Digest{Hash: "94f74174df883c2b1e30e26bfe765b9e15fa39473db34f882275b67fa89a579b", Size: 6504})
}
