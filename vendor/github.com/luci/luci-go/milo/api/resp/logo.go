// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package resp

// A collection of structs used for defining OS, platform, and device logos.
// Each logo defined in this file should have a matching icon in the
// application's static/logo/ folder.

type LogoBase struct {
	Img string
	Alt string
}

type Logo struct {
	LogoBase
	// Subtitle is text that goes under the logo.  This is most commonly used
	// to specify the version of the OS or type of device.
	Subtitle string
	// Count specifies how many of the OS or device there are.
	Count int
}

// Define our known base logos
var (
	Windows = LogoBase{
		Img: "windows.svg",
		Alt: "Microsoft Windows",
	}

	OSX = LogoBase{
		Img: "apple.svg",
		Alt: "Apple OSX",
	}

	Ubuntu = LogoBase{
		Img: "ubuntu.svg",
		Alt: "Canonical Ubuntu",
	}

	// These are devices.
	IOS = LogoBase{
		Img: "ios_device.png",
		Alt: "Apple iOS",
	}

	Android = LogoBase{
		Img: "android_device.png",
		Alt: "Google Android",
	}

	ChromeOS = LogoBase{
		Img: "cros_device.png",
		Alt: "Google ChromeOS",
	}
)
