package main

/*
   Migrate the data for a roller. Intended to be run locally, in place.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"io"

	"cloud.google.com/go/datastore"
	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

var (
	config    = flag.String("config_file", "", "Configuration file for the roller.")
	internal  = flag.Bool("internal", false, "If set, makes the roller internal.")
	external  = flag.Bool("external", false, "If set, makes the roller external.")
	rename    = flag.String("rename", "", "If set, change the roller's name.")
	dryRun    = flag.Bool("dry_run", false, "If set, just log the changes to be made without making them.")
	deleteOld = flag.Bool("delete_old_data", false, "If set, delete the old data.")
)

type migrateConfig struct {
	namespace string
	roller    string
}

func main() {
	common.Init()
	ctx := context.Background()

	if *internal && *external {
		sklog.Fatal("--internal and --external are mutually exclusive.")
	}

	// Load the config.
	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*config, func(r io.Reader) error {
		return json5.NewDecoder(r).Decode(&cfg)
	}); err != nil {
		sklog.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		sklog.Fatalf("Invalid roller config: %s %s", *config, err)
	}

	// Create the old and new migrateConfigs.
	old := migrateConfig{
		namespace: ds.AUTOROLL_NS,
		roller:    cfg.RollerName,
	}
	if cfg.IsInternal {
		old.namespace = ds.AUTOROLL_INTERNAL_NS
	}

	new := migrateConfig{
		namespace: old.namespace,
		roller:    old.roller,
	}
	if *internal {
		new.namespace = ds.AUTOROLL_INTERNAL_NS
	} else if *external {
		new.namespace = ds.AUTOROLL_NS
	}
	if *rename != "" {
		new.roller = *rename
	}

	// Initialize datastore.
	ts, err := auth.NewDefaultTokenSource(true)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, new.namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	// Ensure that we actually have changes to make.
	if deepequal.DeepEqual(old, new) {
		sklog.Fatal("New config is the same as the old config!")
	}

	// Migrate the data.
	kinds := []ds.Kind{ds.KIND_AUTOROLL_MODE, ds.KIND_AUTOROLL_ROLL, ds.KIND_AUTOROLL_STATUS, ds.KIND_AUTOROLL_STRATEGY}
	initData := func(kind ds.Kind) interface{} {
		switch kind {
		case ds.KIND_AUTOROLL_MODE:
			return &[]*modes.ModeChange{}
		case ds.KIND_AUTOROLL_ROLL:
			return &[]*recent_rolls.DsRoll{}
		case ds.KIND_AUTOROLL_STATUS:
			return &[]*status.DsStatusWrapper{}
		case ds.KIND_AUTOROLL_STRATEGY:
			return &[]*strategy.StrategyChange{}
		default:
			sklog.Fatalf("Unknown kind %s", kind)
		}
		return nil
	}
	oldKeys := map[ds.Kind][]*datastore.Key{}
	for _, kind := range kinds {
		q := datastore.NewQuery(string(kind)).Namespace(old.namespace).Filter("roller =", old.roller)
		data := initData(kind)
		keys, err := ds.DS.GetAll(ctx, q, data)
		if err != nil {
			sklog.Fatal(err)
		}
		oldKeys[kind] = keys
		newKeys := make([]*datastore.Key, 0, len(keys))
		for _, k := range keys {
			newKey := &datastore.Key{
				Kind: k.Kind,
				ID:   k.ID,
				Name: k.Name,
				Parent: &datastore.Key{
					Kind:      k.Parent.Kind,
					ID:        k.Parent.ID,
					Name:      k.Parent.Name,
					Namespace: new.namespace,
				},
				Namespace: new.namespace,
			}
			newKeys = append(newKeys, newKey)
		}
		sklog.Infof("Inserting %d entries of kind %q in namespace %q for roller %q", len(keys), kind, new.namespace, new.roller)
		if !*dryRun {
			if err := util.ChunkIter(len(keys), ds.MAX_MODIFICATIONS, func(start, end int) error {
				_, err := ds.DS.PutMulti(ctx, newKeys, data)
				return err
			}); err != nil {
				sklog.Fatal(err)
			}
		}
	}

	// Delete the old data.
	if *deleteOld {
		for kind, keys := range oldKeys {
			sklog.Infof("Deleting %d entries of kind %q in namespace %q for roller %q", len(keys), kind, old.namespace, old.roller)
			if !*dryRun {
				if err := util.ChunkIter(len(keys), ds.MAX_MODIFICATIONS, func(start, end int) error {
					return ds.DS.DeleteMulti(ctx, keys)
				}); err != nil {
					sklog.Fatal(err)
				}
			}
		}
	}

	// Rewrite the config file.
	cfg.RollerName = new.roller
	if new.namespace == ds.AUTOROLL_INTERNAL_NS {
		cfg.IsInternal = true
	} else {
		cfg.IsInternal = false
	}
	if err := util.WithWriteFile(*config, func(w io.Writer) error {
		b, err := json.MarshalIndent(&cfg, "", "  ")
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}); err != nil {
		sklog.Fatal(err)
	}
}
