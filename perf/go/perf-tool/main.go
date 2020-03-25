// Command-line application for interacting with BigTable backed Perf storage.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/btts"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

var (
	ts             oauth2.TokenSource
	store          tracestore.TraceStore
	configFilename string
	instanceConfig *config.InstanceConfig
)

// flags
var (
	indicesTileFlag types.TileNumber
	tracesTileFlag  types.TileNumber

	tracesQueryFlag string

	ingestStartFlag  string
	ingestEndFlag    string
	ingestDryrunFlag bool
)

func main() {
	ctx := context.Background()

	cmd := cobra.Command{
		Use: "perf-tool [sub]",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			glog_and_cloud.SetLogger(glog_and_cloud.NewStdErrCloudLogger(glog_and_cloud.SLogStderr))

			var err error
			ts, err = auth.NewDefaultTokenSource(true, bigtable.Scope)
			if err != nil {
				return fmt.Errorf("Failed to auth: %s", err)
			}

			instanceConfig, err = config.InstanceConfigFromFile(configFilename)
			if err != nil {
				return skerr.Wrap(err)
			}
			// Create the store client.
			store, err = btts.NewBigTableTraceStoreFromConfig(ctx, instanceConfig, ts, false)
			if err != nil {
				return fmt.Errorf("Failed to create client: %s", err)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&configFilename, "config_filename", "./configs/nano.json", "The filename of the config file to use.")
	err := cmd.MarkPersistentFlagRequired("config_filename")
	if err != nil {
		sklog.Fatal(err)
	}

	configCmd := &cobra.Command{
		Use: "config [sub]",
	}
	configPubSubCmd := &cobra.Command{
		Use:   "create-pubsub-topics",
		Short: "Create PubSub topics for the given big_table_config.",
		RunE:  configCreatePubSubTopicsAction,
	}
	configCmd.AddCommand(configPubSubCmd)

	indicesCmd := &cobra.Command{
		Use: "indices [sub]",
	}
	indicesCmd.PersistentFlags().Int32Var((*int32)(&indicesTileFlag), "tile", -1, "The tile to query")
	indicesCountCmd := &cobra.Command{
		Use:   "count",
		Short: "Counts the number of index rows.",
		Long:  "Counts the index rows for the last (most recent) tile, or the tile specified by --tile.",
		RunE:  indicesCountAction,
	}
	indicesWriteCmd := &cobra.Command{
		Use:   "write",
		Short: "Write indices",
		Long:  "Rewrites the indices for the last (most recent) tile, or the tile specified by --tile.",
		RunE:  indicesWriteAction,
	}
	indicesWriteAllCmd := &cobra.Command{
		Use:   "write-all",
		Short: "Write indices for all tiles.",
		Long:  "Rewrites the indices for all tiles, --tiles is ignored. Starts with latest tile and keeps moving to previous tiles until it finds a tile with no traces.",
		RunE:  indicesWriteAllAction,
	}
	indicesWriteCmd.Flags().Int32Var((*int32)(&indicesTileFlag), "tile", -1, "The tile to query")

	indicesCmd.AddCommand(
		indicesCountCmd,
		indicesWriteCmd,
		indicesWriteAllCmd,
	)

	tilesCmd := &cobra.Command{
		Use: "tiles [sub]",
	}
	tilesLast := &cobra.Command{
		Use:   "last",
		Short: "Prints the offset of the last (most recent) tile.",
		RunE:  tilesLastAction,
	}

	tilesCmd.AddCommand(
		tilesLast,
	)

	tracesCmd := &cobra.Command{
		Use: "traces [sub]",
	}
	tracesCmd.PersistentFlags().Int32Var((*int32)(&tracesTileFlag), "tile", -1, "The tile to query")
	tracesCmd.PersistentFlags().StringVar(&tracesQueryFlag, "query", "", "The query to run. Defaults to the empty query which matches all traces.")

	tracesListByIndexCmd := &cobra.Command{
		Use:   "list",
		Short: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
		RunE:  tracesListByIndexAction,
	}

	tracesCmd.AddCommand(
		tracesListByIndexCmd,
	)

	ingestCmd := &cobra.Command{
		Use: "ingest [sub]",
	}

	ingestForceReingestCmd := &cobra.Command{
		Use:   "force-reingest",
		Short: "Force re-ingestion of files.",
		RunE:  ingestForceReingestAction,
	}

	ingestForceReingestCmd.Flags().StringVar(&ingestStartFlag, "start", "", "Start the ingestion at this time, of the form: 2006-01-02. Default to one week ago.")
	ingestForceReingestCmd.Flags().StringVar(&ingestEndFlag, "end", "", "Ingest up to this time, of the form: 2006-01-02. Defaults to now.")
	ingestForceReingestCmd.Flags().BoolVar(&ingestDryrunFlag, "dryrun", false, "Just display the list of files to send.")

	ingestCmd.AddCommand(ingestForceReingestCmd)

	cmd.AddCommand(
		configCmd,
		indicesCmd,
		tilesCmd,
		tracesCmd,
		ingestCmd,
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

func tilesLastAction(c *cobra.Command, args []string) error {
	tileNumber, err := store.GetLatestTile()
	if err != nil {
		return err
	}
	fmt.Println(tileNumber)
	return nil
}

func tracesListByIndexAction(c *cobra.Command, args []string) error {
	var tileNumber types.TileNumber
	if tracesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return err
		}
	} else {
		tileNumber = tracesTileFlag
	}
	values, err := url.ParseQuery(tracesQueryFlag)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	ts, err := store.QueryTracesByIndex(context.Background(), tileNumber, q)
	if err != nil {
		return err
	}
	for id, trace := range ts {
		fmt.Println(id, trace)
	}
	return nil
}

func indicesWriteAction(c *cobra.Command, args []string) error {
	var tileNumber types.TileNumber
	if indicesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileNumber = indicesTileFlag
	}
	return store.WriteIndices(context.Background(), tileNumber)
}

func indicesWriteAllAction(c *cobra.Command, args []string) error {
	tileNumber, err := store.GetLatestTile()
	if err != nil {
		return fmt.Errorf("Failed to get latest tile: %s", err)
	}
	for {
		if err := store.WriteIndices(context.Background(), tileNumber); err != nil {
			return err
		}
		sklog.Infof("Wrote index for tile %d", tileNumber)
		tileNumber = tileNumber.Prev()
		count, err := store.TraceCount(context.Background(), tileNumber)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func indicesCountAction(c *cobra.Command, args []string) error {
	var tileNumber types.TileNumber
	if indicesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileNumber = indicesTileFlag
	}
	count, err := store.CountIndices(context.Background(), tileNumber)
	if err == nil {
		fmt.Println(count)
	}
	return err
}

func createPubSubTopic(ctx context.Context, client *pubsub.Client, topicName string) error {
	topic := client.Topic(topicName)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("Topic %q already exists\n", topicName)
		return nil
	}

	_, err = client.CreateTopic(ctx, topicName)
	return fmt.Errorf("Failed to create topic %q: %s", topicName, err)
}

func configCreatePubSubTopicsAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project)
	if err != nil {
		return err
	}
	if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.SourceConfig.Topic); err != nil {
		return err
	}
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.FileIngestionTopicName); err != nil {
			return err
		}
	}

	return nil
}

func ingestForceReingestAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(true, storage.ScopeReadOnly)
	if err != nil {
		return skerr.Wrap(err)
	}

	pubSubClient, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		return skerr.Wrap(err)
	}
	topic := pubSubClient.Topic(instanceConfig.IngestionConfig.SourceConfig.Topic)

	now := time.Now()
	startTime := now.Add(-7 * 24 * time.Hour)
	if ingestStartFlag != "" {
		startTime, err = time.Parse("2006-01-02", ingestStartFlag)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	endTime := now
	if ingestEndFlag != "" {
		endTime, err = time.Parse("2006-01-02", ingestEndFlag)
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, prefix := range instanceConfig.IngestionConfig.SourceConfig.Sources {
		u, err := url.Parse(prefix)
		if err != nil {
			return skerr.Wrap(err)
		}

		dirs := fileutil.GetHourlyDirs(u.Path[1:], startTime.Unix(), endTime.Unix())
		for _, dir := range dirs {
			sklog.Infof("Directory: %q", dir)
			err := gcs.AllFilesInDir(gcsClient, u.Host, dir, func(item *storage.ObjectAttrs) {
				// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
				// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
				sklog.Infof("File: %q", item.Name)
				b, err := json.Marshal(storage.ObjectAttrs{
					Name:   item.Name,
					Bucket: u.Host,
				})
				if err != nil {
					sklog.Errorf("Failed to serialize event: %s", err)
					return
				}
				if ingestDryrunFlag {
					fmt.Println(item.Name, item.Bucket)
					return
				}
				topic.Publish(ctx, &pubsub.Message{
					Data: b,
				})
			})
			if err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}
