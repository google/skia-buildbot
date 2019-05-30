// Command-line application for interacting with BigTable backed Perf storage.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"cloud.google.com/go/bigtable"
	"github.com/jcgregorio/logger"
	"github.com/jcgregorio/slog"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
)

var (
	store *btts.BigTableTraceStore
)

// TODO(jcgregorio) Migrate this into its own module that we can use everywhere
// once we're happy with the design.
//
// cloudLoggerImpl implements sklog.CloudLogger.
type cloudLoggerImpl struct {
	stdLog slog.Logger
}

// newLogger creates a new cloudLoggerImpl that either logs to stdout, or does
// no logging, depending upon the value of enable.
func newLogger(enable bool) *cloudLoggerImpl {
	if enable {
		return &cloudLoggerImpl{
			stdLog: logger.NewFromOptions(&logger.Options{SyncWriter: os.Stderr}),
		}
	} else {
		return &cloudLoggerImpl{
			stdLog: logger.NewNopLogger(),
		}
	}
}

func (c *cloudLoggerImpl) CloudLog(reportName string, payload *sklog.LogPayload) {
	switch payload.Severity {
	case sklog.DEBUG:
		c.stdLog.Debug(payload.Payload)
	case sklog.INFO, sklog.NOTICE:
		c.stdLog.Info(payload.Payload)
	case sklog.WARNING:
		c.stdLog.Warning(payload.Payload)
	case sklog.ERROR:
		c.stdLog.Error(payload.Payload)
	case sklog.CRITICAL, sklog.ALERT:
		c.stdLog.Fatal(payload.Payload)
	}
}

func (c *cloudLoggerImpl) BatchCloudLog(reportName string, payloads ...*sklog.LogPayload) {
	for _, payload := range payloads {
		c.CloudLog(reportName, payload)
	}
}

func (c *cloudLoggerImpl) Flush() {
	_ = os.Stdout.Sync()
}

// flags
var (
	logToStdErr    bool
	bigTableConfig string
	tile           int32
	queryFlag      string
)

func main() {
	ctx := context.Background()

	cmd := cobra.Command{
		Use: "perf-tool [sub]",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			sklog.SetLogger(newLogger(logToStdErr))

			ts, err := auth.NewDefaultTokenSource(true, bigtable.Scope)
			if err != nil {
				return fmt.Errorf("Failed to auth: %s", err)
			}

			// Create the store client.
			cfg := config.PERF_BIGTABLE_CONFIGS[bigTableConfig]
			store, err = btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, false)
			if err != nil {
				return fmt.Errorf("Failed to create client: %s", err)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&bigTableConfig, "big_table_config", "nano", "The name of the config to use when using a BigTable trace store.")
	cmd.PersistentFlags().BoolVar(&logToStdErr, "logtostderr", false, "Otherwise logs are not produced.")

	configCmd := &cobra.Command{
		Use: "config [sub]",
	}
	configListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all the available configs.",
		Run:   configListAction,
	}
	configCmd.AddCommand(configListCmd)

	indicesCmd := &cobra.Command{
		Use: "indices [sub]",
	}
	indicesCmd.PersistentFlags().Int32Var(&tile, "tile", -1, "The tile to query")
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
	indicesWriteCmd.Flags().Int32Var(&tile, "tile", -1, "The tile to query")

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
	tracesCmd.PersistentFlags().Int32Var(&tile, "tile", -1, "The tile to query")
	tracesCmd.PersistentFlags().StringVar(&queryFlag, "query", "", "The query to run. Defaults to the empty query which matches all traces.")

	tracesCountCmd := &cobra.Command{
		Use:   "count",
		Short: "Prints the number of traces in the last (most recent) tile, or the tile specified by the --tile flag.",
		RunE:  tracesCountAction,
	}

	tracesListCmd := &cobra.Command{
		Use:  "list",
		Long: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
		RunE: tracesListAction,
	}

	tracesListByIndexCmd := &cobra.Command{
		Use:   "list-by-index",
		Short: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
		RunE:  tracesListByIndexAction,
	}

	tracesCmd.AddCommand(
		tracesCountCmd,
		tracesListCmd,
		tracesListByIndexCmd,
	)

	cmd.AddCommand(
		configCmd,
		indicesCmd,
		tilesCmd,
		tracesCmd,
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

func tilesLastAction(c *cobra.Command, args []string) error {
	tileKey, err := store.GetLatestTile()
	if err != nil {
		return err
	}
	fmt.Println(tileKey.Offset())
	return nil
}

func tracesCountAction(c *cobra.Command, args []string) error {
	var tileKey btts.TileKey
	if tile == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			return err
		}
	} else {
		tileKey = btts.TileKeyFromOffset(tile)
	}
	values, err := url.ParseQuery(queryFlag)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	count, err := store.QueryCount(context.Background(), tileKey, q)
	if err != nil {
		return err
	}
	fmt.Println(count)
	return nil
}

func tracesListAction(c *cobra.Command, args []string) error {
	var tileKey btts.TileKey
	if tile == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			return err
		}
	} else {
		tileKey = btts.TileKeyFromOffset(tile)
	}
	values, err := url.ParseQuery(queryFlag)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	ts, err := store.QueryTraces(context.Background(), tileKey, q)
	if err != nil {
		return err
	}
	for id := range ts {
		fmt.Println(id)
	}
	return nil
}

func tracesListByIndexAction(c *cobra.Command, args []string) error {
	var tileKey btts.TileKey
	if tile == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			return err
		}
	} else {
		tileKey = btts.TileKeyFromOffset(tile)
	}
	values, err := url.ParseQuery(queryFlag)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	ts, err := store.QueryTracesByIndex(context.Background(), tileKey, q)
	if err != nil {
		return err
	}
	for id := range ts {
		fmt.Println(id)
	}
	return nil
}

func indicesWriteAction(c *cobra.Command, args []string) error {
	var tileKey btts.TileKey
	if tile == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(tile)
	}
	return store.WriteIndices(context.Background(), tileKey)
}

func indicesWriteAllAction(c *cobra.Command, args []string) error {
	tileKey, err := store.GetLatestTile()
	if err != nil {
		return fmt.Errorf("Failed to get latest tile: %s", err)
	}
	// Empty query to match all traces.
	q, err := query.New(url.Values{})
	if err != nil {
		return err
	}
	for {
		if err := store.WriteIndices(context.Background(), tileKey); err != nil {
			return err
		}
		tileKey = tileKey.PrevTile()
		count, err := store.QueryCount(context.Background(), tileKey, q)
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
	var tileKey btts.TileKey
	if tile == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(tile)
	}
	count, err := store.CountIndices(context.Background(), tileKey)
	if err == nil {
		fmt.Println(count)
	}
	return err
}

func configListAction(c *cobra.Command, args []string) {
	for k := range config.PERF_BIGTABLE_CONFIGS {
		fmt.Println(k)
	}
}
