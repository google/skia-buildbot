package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	btdb "go.skia.org/infra/task_driver/go/db/bigtable"
	"golang.org/x/oauth2/google"
)

var (
	// Flags.
	btInstance = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject  = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	id         = flag.String("id", "", "ID of Task Driver whose messages should be retrieved.")
	output     = flag.String("output", "", "Optional, write results to this file.")
)

func main() {
	common.Init()
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, bigtable.Scope)
	if err != nil {
		sklog.Fatal(err)
	}
	db, err := btdb.NewBigTableDB(ctx, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	msgs, err := db.GetMessagesForTaskDriver(ctx, *id)
	if err != nil {
		sklog.Fatal(err)
	}
	b, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	if *output == "" {
		fmt.Println(string(b))
	} else {
		if err := util.WithWriteFile(*output, func(w io.Writer) error {
			_, err := w.Write(b)
			return err
		}); err != nil {
			sklog.Fatal(err)
		}
	}
}
