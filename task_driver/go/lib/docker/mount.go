package docker

import (
	"fmt"
	"strings"
)

// MountType* are acceptable values for Mount.Type.
const (
	MountTypeBind   = "bind"
	MountTypeTmpfs  = "tmpfs"
	MountTypeVolume = "volume"
)

// Mount represents a mount into a Docker container.
type Mount struct {
	Type         string
	Source       string
	Destination  string
	Readonly     bool
	VolumeDriver string
	VolumeOpts   []string
}

// Args returns the argument list (eg. for "docker run") which represents this
// Mount.
func (m Mount) Args() []string {
	parts := []string{
		fmt.Sprintf("type=%s", m.Type),
		fmt.Sprintf("destination=%s", m.Destination),
	}
	if m.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", m.Source))
	}
	if m.Readonly {
		parts = append(parts, "readonly")
	}
	if m.VolumeDriver != "" {
		parts = append(parts, fmt.Sprintf("volume-driver=%s", m.VolumeDriver))
	}
	for _, opt := range m.VolumeOpts {
		if strings.Contains(opt, ",") {
			opt = fmt.Sprintf("%q", opt)
		}
		parts = append(parts, fmt.Sprintf("volume-opt=%s", opt))
	}
	return []string{"--mount", strings.Join(parts, ",")}
}
