// android_skia_checkout contains util methods for interacting with
// the Skia repository within Android.
package android_skia_checkout

import (
	"context"
	"fmt"
	"path"

	"go.skia.org/infra/go/exec"
)

var (
	// Files within the Skia checkout.
	SkqpUserConfigRelPath           = path.Join("skqp", "include", "config", "SkUserConfig.h")
	SkUserConfigRelPath             = path.Join("include", "config", "SkUserConfig.h")
	SkUserConfigAndroidRelPath      = path.Join("android", "include", "config", "SkUserConfig.h")
	SkUserConfigLinuxRelPath        = path.Join("linux", "include", "config", "SkUserConfig.h")
	SkUserConfigMacRelPath          = path.Join("mac", "include", "config", "SkUserConfig.h")
	SkUserConfigWinRelPath          = path.Join("win", "include", "config", "SkUserConfig.h")
	SkUserConfigRenderengineRelPath = path.Join("renderengine", "include", "config", "SkUserConfig.h")
	AndroidBpRelPath                = path.Join("Android.bp")
	VmaAndroidInclude               = path.Join("vma_android", "include", "vk_mem_alloc.h")
	VmaAndroidLicense               = path.Join("vma_android", "LICENSE.txt")

	FilesGeneratedByGnToGp = []string{SkqpUserConfigRelPath, SkUserConfigAndroidRelPath, SkUserConfigLinuxRelPath, SkUserConfigMacRelPath, SkUserConfigWinRelPath, AndroidBpRelPath, SkUserConfigRenderengineRelPath, VmaAndroidInclude, VmaAndroidLicense}
)

const (
	// The remote pointing to googleplex repo that is automatically present
	// in the Skia repository within Android.
	BUILT_IN_REMOTE = "goog"
)

func RunGnToBp(ctx context.Context, skiaCheckout string) error {
	if _, syncErr := exec.RunCwd(ctx, skiaCheckout, "./bin/sync"); syncErr != nil {
		return fmt.Errorf("bin/sync error: %s", syncErr)
	}
	if _, fetchGNErr := exec.RunCwd(ctx, skiaCheckout, "./bin/fetch-gn"); fetchGNErr != nil {
		return fmt.Errorf("Failed to install GN: %s", fetchGNErr)
	}

	// Generate and add files created by gn/gn_to_bp.py
	gnPath := fmt.Sprintf("%s/bin/gn", skiaCheckout)
	if _, gnToBpErr := exec.RunCommand(ctx, &exec.Command{
		Dir:  skiaCheckout,
		Name: "python3",
		Args: []string{"gn/gn_to_bp.py", "--gn", gnPath},
	}); gnToBpErr != nil {
		return fmt.Errorf("Failed to run gn_to_bp: %s", gnToBpErr)
	}
	return nil
}
