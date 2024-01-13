// Package graphsshortcut handles storing and retrieving shortcuts for graphs.
package graphsshortcut

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sort"
)

// GraphConfig represent the configurations used to populate a single graph.
type GraphConfig struct {
	Queries  []string `json:"queries"`
	Formulas []string `json:"formulas"`
	Keys     string   `json:"keys"`
}

// GraphsShortcut is a list of GraphConfigs, it is used in the Store interface.
type GraphsShortcut struct {
	Graphs []GraphConfig `json:"graphs"`
}

// Store is an interface for things that persists Graphs Shortcuts.
type Store interface {
	// InsertShortcut adds the shortcut content into the database. The id of the
	// shortcut is returned.
	InsertShortcut(ctx context.Context, shortcut *GraphsShortcut) (string, error)

	// GetShortcut retrieves parsed graph configs for the given id.
	GetShortcut(ctx context.Context, id string) (*GraphsShortcut, error)
}

func (s GraphsShortcut) GetID() string {
	if len(s.Graphs) == 0 {
		return ""
	}

	h := md5.New()
	for _, g := range s.Graphs {
		_, _ = io.WriteString(h, "GRAPH")
		sort.Strings(g.Formulas)
		sort.Strings(g.Queries)
		for _, q := range g.Queries {
			_, _ = io.WriteString(h, q)
		}

		for _, f := range g.Formulas {
			_, _ = io.WriteString(h, f)
		}

		_, _ = io.WriteString(h, g.Keys)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
