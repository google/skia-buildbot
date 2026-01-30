package readme_chromium

import (
	"bytes"
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
	f, err := Parse([]byte(example1))
	require.NoError(t, err)
	require.Equal(t, &ReadmeChromiumFile{
		originalContentLines: bytes.Split([]byte(example1), []byte("\n")),
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
			file, err := Parse([]byte(example1))
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
