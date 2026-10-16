package main

import (
	"testing"
)

func TestIsAllowedExec(t *testing.T) {
	testCases := []struct {
		name     string
		binary   string
		args     []string
		envp     []string
		expected bool
	}{
		{
			name:     "Legitimate clang command",
			binary:   "/usr/bin/clang",
			args:     []string{"clang", "-o", "/tmp/fiddle", "main.cpp"},
			envp:     []string{"PATH=/usr/bin", "HOME=/tmp"},
			expected: true,
		},
		{
			name:     "Blocked: LD_PRELOAD",
			binary:   "/usr/bin/clang",
			args:     []string{"clang", "-o", "/tmp/fiddle", "main.cpp"},
			envp:     []string{"LD_PRELOAD=/tmp/malicious.so"},
			expected: false,
		},
		{
			name:     "Blocked: LD_LIBRARY_PATH",
			binary:   "/usr/bin/clang",
			args:     []string{"clang", "-o", "/tmp/fiddle", "main.cpp"},
			envp:     []string{"LD_LIBRARY_PATH=/tmp"},
			expected: false,
		},
		{
			name:     "Blocked: PYTHONHOME",
			binary:   "/usr/bin/clang",
			args:     []string{"clang", "-o", "/tmp/fiddle", "main.cpp"},
			envp:     []string{"PYTHONHOME=/tmp"},
			expected: false,
		},
		{
			name:     "Blocked: Dangerous compiler flag",
			binary:   "/usr/bin/clang",
			args:     []string{"clang", "-fplugin=malicious.so", "-o", "/tmp/fiddle", "main.cpp"},
			envp:     []string{"PATH=/usr/bin"},
			expected: false,
		},
		{
			name:     "Legitimate shell command",
			binary:   "/bin/sh",
			args:     []string{"/bin/sh", "-c", "clang -o /tmp/fiddle main.cpp"},
			envp:     []string{"PATH=/usr/bin"},
			expected: true,
		},
		{
			name:     "Blocked: Shell command with dangerous env",
			binary:   "/bin/sh",
			args:     []string{"/bin/sh", "-c", "clang -o /tmp/fiddle main.cpp"},
			envp:     []string{"LD_PRELOAD=/tmp/malicious.so"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isAllowedExec(tc.binary, "fiddle", tc.args, tc.envp, true)
			if actual != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestMatchesPrefixes(t *testing.T) {
	testCases := []struct {
		name     string
		rawPath  string
		expected bool
	}{
		{
			name:     "Allowed /proc/self/maps",
			rawPath:  "/proc/self/maps",
			expected: true,
		},
		{
			name:     "Allowed /proc/self/comm",
			rawPath:  "/proc/self/comm",
			expected: true,
		},
		{
			name:     "Allowed /proc/self/task/<tid>/maps",
			rawPath:  "/proc/self/task/123/maps",
			expected: true,
		},
		{
			name:     "Allowed /proc/self/task/<tid>/comm",
			rawPath:  "/proc/self/task/123/comm",
			expected: true,
		},
		{
			name:     "Allowed /proc/self/task/<tid>/stat",
			rawPath:  "/proc/self/task/123/stat",
			expected: true,
		},
		{
			name:     "Blocked /proc/self/root/etc/passwd",
			rawPath:  "/proc/self/root/etc/passwd",
			expected: false,
		},
		{
			name:     "Blocked /proc/self/task/<tid>/root/etc/passwd",
			rawPath:  "/proc/self/task/123/root/etc/passwd",
			expected: false,
		},
		{
			name:     "Blocked /proc/self/task/<tid>/../<tid>/root/etc/passwd",
			rawPath:  "/proc/self/task/123/../456/root/etc/passwd",
			expected: false,
		},
		{
			name:     "Blocked /proc/self/task/<tid>/maps/../../root/etc/passwd",
			rawPath:  "/proc/self/task/123/maps/../../root/etc/passwd",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, actual := matchesPrefixes(tc.rawPath, readonlyAllowedPrefixes)
			if actual != tc.expected {
				t.Errorf("matchesPrefixes(%q) = (%q, %v), want %v", tc.rawPath, normalized, actual, tc.expected)
			}
		})
	}
}
