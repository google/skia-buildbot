package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/firestore/testutils"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestFirestoreDB(t *testing.T) {

	ctx := context.Background()
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()
	d, err := NewDB(context.Background(), c)
	require.NoError(t, err)
	rollerID := "my-roller"

	// No configs.
	cfgs, err := d.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, cfgs, 0)
	cfg, err := d.Get(ctx, rollerID)
	require.Equal(t, ErrNotFound, err)
	require.Nil(t, cfg)

	// Add a config.
	cfg = getConfig(t)
	require.NoError(t, d.Put(ctx, rollerID, cfg))

	// Ensure that we validate the config in Put().
	cfg = &config.Config{}
	require.Error(t, d.Put(ctx, rollerID, cfg))

	// Verify that we can find the config.
	cfgs, err = d.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, cfgs, 1)
	require.NotNil(t, cfgs[0])
	cfg, err = d.Get(ctx, rollerID)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NoError(t, cfg.Validate())

	// Delete the config.
	require.NoError(t, d.Delete(ctx, rollerID))

	// Verify that the config is gone.
	cfgs, err = d.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, cfgs, 0)
	cfg, err = d.Get(ctx, rollerID)
	require.Equal(t, ErrNotFound, err)
	require.Nil(t, cfg)
}

// getConfig returns a valid roller config.
func getConfig(t *testing.T) *config.Config {
	cfgStr := `
roller_name:  "skia-autoroll"
child_bug_link:  "https://bugs.chromium.org/p/skia/issues/entry"
child_display_name:  "Skia"
parent_bug_link:  "https://bugs.chromium.org/p/chromium/issues/entry"
parent_display_name:  "Chromium"
parent_waterfall:  "https://build.chromium.org"
owner_primary:  "borenet"
owner_secondary:  "rmistry"
contacts:  "borenet@google.com"
service_account:  "chromium-autoroll@skia-public.iam.gserviceaccount.com"
reviewer:  "https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener"
supports_manual_rolls:  true
commit_msg:  {
	bug_project:  "chromium"
	child_log_url_tmpl:  "https://skia.googlesource.com/skia.git/+log/{{.RollingFrom}}..{{.RollingTo}}"
	cq_extra_trybots:  "luci.chromium.try:android_optional_gpu_tests_rel"
	cq_extra_trybots:  "luci.chromium.try:linux-blink-rel"
	cq_extra_trybots:  "luci.chromium.try:linux-chromeos-compile-dbg"
	cq_extra_trybots:  "luci.chromium.try:linux_optional_gpu_tests_rel"
	cq_extra_trybots:  "luci.chromium.try:mac_optional_gpu_tests_rel"
	cq_extra_trybots:  "luci.chromium.try:win_optional_gpu_tests_rel"
	cq_do_not_cancel_trybots:  true
	include_log:  true
	include_revision_count:  true
	include_tbr_line:  true
	include_tests:  true
	built_in:  DEFAULT
}
gerrit:  {
	url:  "https://chromium-review.googlesource.com"
	project:  "chromium/src"
	config:  CHROMIUM_BOT_COMMIT
}
kubernetes:  {
	cpu:  "1"
	memory:  "2Gi"
	readiness_failure_threshold:  10
	readiness_initial_delay_seconds:  30
	readiness_period_seconds:  30
	image:  "gcr.io/skia-public/autoroll-be:2022-02-14T15_10_50Z-borenet-0056074-clean"
}
parent_child_repo_manager:  {
	gitiles_parent:  {
	gitiles:  {
		branch:  "main"
		repo_url:  "https://chromium.googlesource.com/chromium/src.git"
	}
	dep:  {
		primary:  {
		id:  "https://skia.googlesource.com/skia.git"
		path:  "DEPS"
		}
	}
	gerrit:  {
		url:  "https://chromium-review.googlesource.com"
		project:  "chromium/src"
		config:  CHROMIUM_BOT_COMMIT
	}
	}
	gitiles_child:  {
	gitiles:  {
		branch:  "main"
		repo_url:  "https://skia.googlesource.com/skia.git"
	}
	}
}
notifiers:  {
	log_level:  WARNING
	email:  {
	emails:  "borenet@google.com"
	}
}	
`
	cfg := new(config.Config)
	require.NoError(t, prototext.Unmarshal([]byte(cfgStr), cfg))
	return cfg
}
