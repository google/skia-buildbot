package readme_chromium

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const example1 = `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`

func TestParse(t *testing.T) {
	f, err := Parse(example1)
	require.NoError(t, err)
	require.Equal(t, &ReadmeChromiumFile{
		originalContentLines: strings.Split(example1, "\n"),
		fields: []*field{
			{
				Name:       "Name",
				LineNo:     1,
				StartIndex: 6,
				EndIndex:   22,
				Required:   true,
			},
			{
				Name:       "Short Name",
				LineNo:     2,
				StartIndex: 12,
				EndIndex:   20,
				Required:   false,
			},
			{
				Name:       "URL",
				LineNo:     3,
				StartIndex: 5,
				EndIndex:   39,
				Required:   true,
			},
			{
				Name:       "Version",
				LineNo:     6,
				StartIndex: 9,
				EndIndex:   13,
				Required:   true,
			},
			{
				Name:       "Date",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "Revision",
				LineNo:     8,
				StartIndex: 10,
				EndIndex:   50,
				Required:   false,
			},
			{
				Name:       "Update Mechanism",
				LineNo:     9,
				StartIndex: 18,
				EndIndex:   24,
				Required:   true,
			},
			{
				Name:       "License",
				LineNo:     4,
				StartIndex: 9,
				EndIndex:   21,
				Required:   true,
			},
			{
				Name:       "License File",
				LineNo:     5,
				StartIndex: 14,
				EndIndex:   21,
				Required:   false,
			},
			{
				Name:       "Shipped",
				LineNo:     11,
				StartIndex: 9,
				EndIndex:   12,
				Required:   false,
			},
			{
				Name:       "Security Critical",
				LineNo:     10,
				StartIndex: 19,
				EndIndex:   22,
				Required:   false,
			},
			{
				Name:       "License Android Compatible",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "CPEPrefix",
				LineNo:     7,
				StartIndex: 11,
				EndIndex:   38,
				Required:   false,
			},
		},
		Name:             "Protocol Buffers",
		ShortName:        "protobuf",
		URL:              "https://github.com/google/protobuf",
		License:          "BSD-3-Clause",
		LicenseFile:      "LICENSE",
		Version:          "33.0",
		CPEPrefix:        "cpe:/a:google:protobuf:33.0",
		Revision:         "a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551",
		UpdateMechanism:  "Manual",
		SecurityCritical: true,
		Shipped:          true,
	}, f)
}

func TestNewContent(t *testing.T) {
	test := func(name string, fn func(*ReadmeChromiumFile), expect string) {
		t.Run(name, func(t *testing.T) {
			file, err := Parse(example1)
			require.NoError(t, err)
			fn(file)
			actual, err := file.NewContent()
			require.NoError(t, err)
			require.Equal(t, expect, string(actual))
		})
	}

	test("Name", func(file *ReadmeChromiumFile) {
		file.Name = "new name"
	}, `
Name: new name
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("ShortName", func(file *ReadmeChromiumFile) {
		file.ShortName = "new"
	}, `
Name: Protocol Buffers
Short Name: new
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("URL", func(file *ReadmeChromiumFile) {
		file.URL = "http://fake.com"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: http://fake.com
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("Version", func(file *ReadmeChromiumFile) {
		file.Version = "1.1.1"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 1.1.1
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("Date", func(file *ReadmeChromiumFile) {
		// Not present in this example, so the file is unchanged.
		file.Date = "new"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("Revision", func(file *ReadmeChromiumFile) {
		file.Revision = "new"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: new
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("UpdateMechanism", func(file *ReadmeChromiumFile) {
		file.UpdateMechanism = "Autoroll"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Autoroll
Security Critical: yes
Shipped: yes
`)

	test("License", func(file *ReadmeChromiumFile) {
		file.License = "new"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: new
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("LicenseFile", func(file *ReadmeChromiumFile) {
		file.LicenseFile = "new"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: new
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("Shipped", func(file *ReadmeChromiumFile) {
		file.Shipped = false
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: no
`)

	test("SecurityCritical", func(file *ReadmeChromiumFile) {
		file.SecurityCritical = false
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: no
Shipped: yes
`)

	test("LicenseAndroidCompatible", func(file *ReadmeChromiumFile) {
		// Not in the original file, so we can't update it.
		file.LicenseAndroidCompatible = true
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("CPEPrefix", func(file *ReadmeChromiumFile) {
		file.CPEPrefix = "new"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 33.0
CPEPrefix: new
Revision: a79f2d2e9fadd75e94f3fe40a0399bf0a5d90551
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`)

	test("Multiple Fields", func(file *ReadmeChromiumFile) {
		file.Version = "100.1.1"
		file.Revision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		file.UpdateMechanism = "Autoroll"
	}, `
Name: Protocol Buffers
Short Name: protobuf
URL: https://github.com/google/protobuf
License: BSD-3-Clause
License File: LICENSE
Version: 100.1.1
CPEPrefix: cpe:/a:google:protobuf:33.0
Revision: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Update Mechanism: Autoroll
Security Critical: yes
Shipped: yes
`)
}

const exampleMulti = `Name: OpenXR SDK
Short Name: OpenXR
URL: https://github.com/KhronosGroup/OpenXR-SDK
Version: 1.1.53
Revision: 75c53b6e853dc12c7b3c771edc9c9c841b15faaa
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSE
Security Critical: yes
Shipped: yes

Description:
OpenXR is a royalty-free, open standard that provides high-performance access to
Augmented Reality (AR) and Virtual Reality (VR)—collectively known as
XR—platforms and devices.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the OpenXR loader in Chromium, along with its dependencies. The
readme was expanded with information about transitive dependencies that are
copied directly into the OpenXR SDK repository. An openxr.def file works around
the fact that attributes aren't exposed by default for the compiler we use on
windows in component builds.

Added dev/xr_android.h for prototyping xr_android extensions that are currently
under active development and not in any openxr release at present. This file is
expected to be superceded by any official definitions and may require additional
work before a roll containing those definitions can be conducted.

Copied src/.clang-format into src_overrides/.clang-format and disabled
clang-format in src_overrides/src/external to mimic how khronos gitlab seems to
behave. This allows forked files to more closely match the base-files and allow
for easier "Compare with clipboard" comparisons.

The following changes should be reflected in 'src_overrides/patches':
* Forked android_utilites.cpp and manifest_file.cpp to allow for customizing to
ignore loading in Android ContentProvider supplied paths while investigating and
waiting for upstreaming.
* Forked AndroidManifest.xml.in to remove unnecessary fields that prevent
merging with Chrome's AndroidManifest.xml

-------------------- DEPENDENCY DIVIDER --------------------

Name: JNIPP
Short Name: JNIPP
URL: https://github.com/mitchdowd/jnipp
Version: v1.0.0-13-gcdd6293
Revision: cdd6293fca985993129f5ef5441709fc49ee507f
Update Mechanism: Manual
License: MIT
License File: src/src/external/jnipp/LICENSE
Security Critical: yes
Shipped: yes

Description:
JNIPP is just a C++ wrapper for the standard Java Native Interface (JNI).It
tries to take some of the long-winded annoyance out of integrating your Java
and C++ code.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium.

-------------------- DEPENDENCY DIVIDER --------------------

Name: android-jni-wrappers
Short Name: android-jni-wrappers
URL: https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers
Version: N/A
Date: 2023-12-13
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSES/Apache-2.0.txt
Security Critical: yes
Shipped: yes

Description:
Python tool to generate C++ wrappers for (mostly Android-related) JNI/Java
objects. Generated files are typically slightly hand-modified.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium, along with its dependencies. Since it
is a transitive dependency that was directly included in OpenXR SDK repository,
the exact revision is unknown. The library also does not have any versioned
releases. The library contains auto-generated files with unknown hand-made
modifications. The library is triple-licensed, and the copy from OpenXR SDK
repository does not include a LICENSE file.
`

func TestParseMulti(t *testing.T) {
	files, err := ParseMulti(exampleMulti)
	require.NoError(t, err)
	require.Len(t, files, 3)
	require.Equal(t, &ReadmeChromiumFile{
		originalContentLines: strings.Split(dependencyDividerRegex.Split(exampleMulti, -1)[0], "\n"),
		fields: []*field{
			{
				Name:       "Name",
				LineNo:     0,
				StartIndex: 6,
				EndIndex:   16,
				Required:   true,
			},
			{
				Name:       "Short Name",
				LineNo:     1,
				StartIndex: 12,
				EndIndex:   18,
				Required:   false,
			},
			{
				Name:       "URL",
				LineNo:     2,
				StartIndex: 5,
				EndIndex:   47,
				Required:   true,
			},
			{
				Name:       "Version",
				LineNo:     3,
				StartIndex: 9,
				EndIndex:   15,
				Required:   true,
			},
			{
				Name:       "Date",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "Revision",
				LineNo:     4,
				StartIndex: 10,
				EndIndex:   50,
				Required:   false,
			},
			{
				Name:       "Update Mechanism",
				LineNo:     5,
				StartIndex: 18,
				EndIndex:   24,
				Required:   true,
			},
			{
				Name:       "License",
				LineNo:     6,
				StartIndex: 9,
				EndIndex:   19,
				Required:   true,
			},
			{
				Name:       "License File",
				LineNo:     7,
				StartIndex: 14,
				EndIndex:   25,
				Required:   false,
			},
			{
				Name:       "Shipped",
				LineNo:     9,
				StartIndex: 9,
				EndIndex:   12,
				Required:   false,
			},
			{
				Name:       "Security Critical",
				LineNo:     8,
				StartIndex: 19,
				EndIndex:   22,
				Required:   false,
			},
			{
				Name:       "License Android Compatible",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "CPEPrefix",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
		},
		Name:             "OpenXR SDK",
		ShortName:        "OpenXR",
		URL:              "https://github.com/KhronosGroup/OpenXR-SDK",
		License:          "Apache-2.0",
		LicenseFile:      "src/LICENSE",
		Version:          "1.1.53",
		CPEPrefix:        "",
		Revision:         "75c53b6e853dc12c7b3c771edc9c9c841b15faaa",
		UpdateMechanism:  "Manual",
		SecurityCritical: true,
		Shipped:          true,
	}, files[0])

	require.Equal(t, &ReadmeChromiumFile{
		originalContentLines: strings.Split(dependencyDividerRegex.Split(exampleMulti, -1)[1], "\n"),
		fields: []*field{
			{
				Name:       "Name",
				LineNo:     2,
				StartIndex: 6,
				EndIndex:   11,
				Required:   true,
			},
			{
				Name:       "Short Name",
				LineNo:     3,
				StartIndex: 12,
				EndIndex:   17,
				Required:   false,
			},
			{
				Name:       "URL",
				LineNo:     4,
				StartIndex: 5,
				EndIndex:   39,
				Required:   true,
			},
			{
				Name:       "Version",
				LineNo:     5,
				StartIndex: 9,
				EndIndex:   27,
				Required:   true,
			},
			{
				Name:       "Date",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "Revision",
				LineNo:     6,
				StartIndex: 10,
				EndIndex:   50,
				Required:   false,
			},
			{
				Name:       "Update Mechanism",
				LineNo:     7,
				StartIndex: 18,
				EndIndex:   24,
				Required:   true,
			},
			{
				Name:       "License",
				LineNo:     8,
				StartIndex: 9,
				EndIndex:   12,
				Required:   true,
			},
			{
				Name:       "License File",
				LineNo:     9,
				StartIndex: 14,
				EndIndex:   44,
				Required:   false,
			},
			{
				Name:       "Shipped",
				LineNo:     11,
				StartIndex: 9,
				EndIndex:   12,
				Required:   false,
			},
			{
				Name:       "Security Critical",
				LineNo:     10,
				StartIndex: 19,
				EndIndex:   22,
				Required:   false,
			},
			{
				Name:       "License Android Compatible",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "CPEPrefix",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
		},
		Name:                     "JNIPP",
		ShortName:                "JNIPP",
		URL:                      "https://github.com/mitchdowd/jnipp",
		Version:                  "v1.0.0-13-gcdd6293",
		Date:                     "",
		Revision:                 "cdd6293fca985993129f5ef5441709fc49ee507f",
		UpdateMechanism:          "Manual",
		License:                  "MIT",
		LicenseFile:              "src/src/external/jnipp/LICENSE",
		Shipped:                  true,
		SecurityCritical:         true,
		LicenseAndroidCompatible: false,
	}, files[1])
	require.Equal(t, &ReadmeChromiumFile{
		originalContentLines: strings.Split(dependencyDividerRegex.Split(exampleMulti, -1)[2], "\n"),
		fields: []*field{
			{
				Name:       "Name",
				LineNo:     2,
				StartIndex: 6,
				EndIndex:   26,
				Required:   true,
			},
			{
				Name:       "Short Name",
				LineNo:     3,
				StartIndex: 12,
				EndIndex:   32,
				Required:   false,
			},
			{
				Name:       "URL",
				LineNo:     4,
				StartIndex: 5,
				EndIndex:   73,
				Required:   true,
			},
			{
				Name:       "Version",
				LineNo:     5,
				StartIndex: 9,
				EndIndex:   12,
				Required:   true,
			},
			{
				Name:       "Date",
				LineNo:     6,
				StartIndex: 6,
				EndIndex:   16,
				Required:   false,
			},
			{
				Name:       "Revision",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "Update Mechanism",
				LineNo:     7,
				StartIndex: 18,
				EndIndex:   24,
				Required:   true,
			},
			{
				Name:       "License",
				LineNo:     8,
				StartIndex: 9,
				EndIndex:   19,
				Required:   true,
			},
			{
				Name:       "License File",
				LineNo:     9,
				StartIndex: 14,
				EndIndex:   41,
				Required:   false,
			},
			{
				Name:       "Shipped",
				LineNo:     11,
				StartIndex: 9,
				EndIndex:   12,
				Required:   false,
			},
			{
				Name:       "Security Critical",
				LineNo:     10,
				StartIndex: 19,
				EndIndex:   22,
				Required:   false,
			},
			{
				Name:       "License Android Compatible",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
			{
				Name:       "CPEPrefix",
				LineNo:     0,
				StartIndex: 0,
				EndIndex:   0,
				Required:   false,
			},
		},
		Name:                     "android-jni-wrappers",
		ShortName:                "android-jni-wrappers",
		URL:                      "https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers",
		Version:                  "N/A",
		Date:                     "2023-12-13",
		Revision:                 "",
		UpdateMechanism:          "Manual",
		License:                  "Apache-2.0",
		LicenseFile:              "src/LICENSES/Apache-2.0.txt",
		Shipped:                  true,
		SecurityCritical:         true,
		LicenseAndroidCompatible: false,
	}, files[2])
}

func TestWriteMulti(t *testing.T) {
	files, err := ParseMulti(exampleMulti)
	require.NoError(t, err)
	require.Len(t, files, 3)

	files[0].Revision = "newRev"
	files[1].ShortName = "namechange"
	files[2].SecurityCritical = false

	actual, err := WriteMulti(files)
	require.NoError(t, err)
	expect := `Name: OpenXR SDK
Short Name: OpenXR
URL: https://github.com/KhronosGroup/OpenXR-SDK
Version: 1.1.53
Revision: newRev
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSE
Security Critical: yes
Shipped: yes

Description:
OpenXR is a royalty-free, open standard that provides high-performance access to
Augmented Reality (AR) and Virtual Reality (VR)—collectively known as
XR—platforms and devices.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the OpenXR loader in Chromium, along with its dependencies. The
readme was expanded with information about transitive dependencies that are
copied directly into the OpenXR SDK repository. An openxr.def file works around
the fact that attributes aren't exposed by default for the compiler we use on
windows in component builds.

Added dev/xr_android.h for prototyping xr_android extensions that are currently
under active development and not in any openxr release at present. This file is
expected to be superceded by any official definitions and may require additional
work before a roll containing those definitions can be conducted.

Copied src/.clang-format into src_overrides/.clang-format and disabled
clang-format in src_overrides/src/external to mimic how khronos gitlab seems to
behave. This allows forked files to more closely match the base-files and allow
for easier "Compare with clipboard" comparisons.

The following changes should be reflected in 'src_overrides/patches':
* Forked android_utilites.cpp and manifest_file.cpp to allow for customizing to
ignore loading in Android ContentProvider supplied paths while investigating and
waiting for upstreaming.
* Forked AndroidManifest.xml.in to remove unnecessary fields that prevent
merging with Chrome's AndroidManifest.xml

-------------------- DEPENDENCY DIVIDER --------------------

Name: JNIPP
Short Name: namechange
URL: https://github.com/mitchdowd/jnipp
Version: v1.0.0-13-gcdd6293
Revision: cdd6293fca985993129f5ef5441709fc49ee507f
Update Mechanism: Manual
License: MIT
License File: src/src/external/jnipp/LICENSE
Security Critical: yes
Shipped: yes

Description:
JNIPP is just a C++ wrapper for the standard Java Native Interface (JNI).It
tries to take some of the long-winded annoyance out of integrating your Java
and C++ code.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium.

-------------------- DEPENDENCY DIVIDER --------------------

Name: android-jni-wrappers
Short Name: android-jni-wrappers
URL: https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers
Version: N/A
Date: 2023-12-13
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSES/Apache-2.0.txt
Security Critical: no
Shipped: yes

Description:
Python tool to generate C++ wrappers for (mostly Android-related) JNI/Java
objects. Generated files are typically slightly hand-modified.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium, along with its dependencies. Since it
is a transitive dependency that was directly included in OpenXR SDK repository,
the exact revision is unknown. The library also does not have any versioned
releases. The library contains auto-generated files with unknown hand-made
modifications. The library is triple-licensed, and the copy from OpenXR SDK
repository does not include a LICENSE file.
`
	require.Equal(t, expect, actual)
}
