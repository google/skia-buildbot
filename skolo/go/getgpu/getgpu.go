// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows

// This Windows-only program detects and prints out the GPU vendor / device name and version.
package main

import (
	"fmt"

	"github.com/yusufpapurcu/wmi"
	"go.skia.org/infra/go/gpus"
)

type gpuQueryResult struct {
	DriverVersion  string
	PNPDeviceID    string
	VideoProcessor string
}

func main() {
	var results []gpuQueryResult
	err := wmi.Query("SELECT DriverVersion, PNPDeviceID, VideoProcessor FROM Win32_VideoController", &results)
	if err != nil {
		panic(err)
	}

	for _, gpu := range results {
		venID, devID := gpus.WindowsVendorAndDeviceID(gpu.PNPDeviceID)
		venName, devName := gpus.IDsToNames(venID, "Unknown", devID, gpu.VideoProcessor)
		fmt.Println(venName, devName, gpu.DriverVersion)
	}
}
