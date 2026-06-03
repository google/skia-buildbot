package promoter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore/schema"
	"go.skia.org/infra/perf/go/trace_visibility/store"
)

const (
	// Standard database timeout.
	dbTimeout = 15 * time.Minute

	// Bulk updates visibility for matching trace IDs by primary key.
	updateTraceVisibility = `UPDATE TraceParams SET is_public = $1 WHERE trace_id = ANY($2)`

	// batchSelectLimit is safe for memory and database performance when reading candidate trace IDs
	// to promote. It prevents retrieving millions of records into the application memory at once,
	// while keeping the number of query loops small during major sweeps.
	batchSelectLimit = 50000

	// chunkUpdateSize specifies the batch size for SQL UPDATE queries. Large lists of keys
	// updated at once (e.g., trace_id = ANY($2)) can exceed database transaction capacity
	// (such as Spanner mutation limits) or cause excessive row lock contention. A chunk size
	// of 1000 is a safe, stable standard in Perf.
	chunkUpdateSize = 1000

	// updateConcurrency is the number of database workers running parallel updates.
	updateConcurrency = 5

	// maxPaginationIterations is a safety loop limit to prevent infinite execution/sweeping loops.
	maxPaginationIterations = 200
)

// Promoter manages the background promotion of historical traces to public.
type Promoter struct {
	db          pool.Pool
	configStore store.Store
}

// New creates a new Promoter instance.
func New(db pool.Pool, configStore store.Store) *Promoter {
	return &Promoter{
		db:          db,
		configStore: configStore,
	}
}

// StartBackgroundLoop launches the background promotion sweep periodically.
func (p *Promoter) StartBackgroundLoop(ctx context.Context, checkInterval time.Duration) {
	sklog.Infof("Starting background trace visibility promoter loop (interval: %v)...", checkInterval)
	go util.RepeatCtx(ctx, checkInterval, func(ctx context.Context) {
		if err := p.Promote(ctx); err != nil {
			sklog.Errorf("Failed background trace promotion loop: %s", err)
		}
	})
}

// Promote performs the dynamic index-free promotion of historical traces to public.
func (p *Promoter) Promote(ctx context.Context) error {
	sklog.Info("Starting background trace visibility promotion sweep...")

	// Create a query context with deadline if not already set.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dbTimeout)
		defer cancel()
	}

	// 1. Retrieve the list of active expected public rules (synced by the Checker)
	rules, err := p.configStore.GetAll(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to get active visibility rules from DB")
	}
	if len(rules) == 0 {
		sklog.Info("No active visibility rules found. Sweep skipped.")
		return nil
	}

	// 2. Promotion Phase: Query and bulk promote matching internal trace IDs in dynamic batches
	totalPromoted, err := p.promoteMatchingTraces(ctx, rules)
	if err != nil {
		return err
	}

	if totalPromoted > 0 {
		sklog.Infof("Promotion sweep complete. Total traces promoted to public: %d", totalPromoted)
	} else {
		sklog.Info("All matching historical traces are already public.")
	}

	sklog.Info("Background trace visibility sweep completed successfully.")
	return nil
}

func (p *Promoter) promoteMatchingTraces(ctx context.Context, rules []schema.PublicTraceRulesSchema) (int, error) {
	var ruleExprs []string
	for _, rule := range rules {
		ruleExprs = append(ruleExprs, rule.RuleExpression)
	}

	totalPromoted := 0
	var lastTID []byte
	for iteration := 0; iteration < maxPaginationIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			sklog.Infof("Gracefully exiting promotion sweep loop: context is done (already promoted %d traces in this sweep).", totalPromoted)
			return totalPromoted, ctx.Err()
		}

		sqlQuery, args := buildPromotionQuery(ruleExprs, batchSelectLimit, lastTID)
		if sqlQuery == "" {
			return totalPromoted, nil
		}

		// Query a limited batch of internal traces
		rows, err := p.db.Query(ctx, sqlQuery, args...)
		if err != nil {
			if ctx.Err() != nil {
				sklog.Infof("Gracefully exiting promotion sweep loop due to context deadline/cancellation (already promoted %d traces in this sweep).", totalPromoted)
				return totalPromoted, ctx.Err()
			}
			return totalPromoted, skerr.Wrapf(err, "failed to query internal traces for active rules")
		}

		var batchTIDs [][]byte
		for rows.Next() {
			var tid []byte
			if err := rows.Scan(&tid); err != nil {
				rows.Close()
				return totalPromoted, skerr.Wrapf(err, "failed to scan internal trace ID")
			}
			batchTIDs = append(batchTIDs, tid)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			return totalPromoted, skerr.Wrapf(err, "error during trace ID rows iteration")
		}

		if len(batchTIDs) == 0 {
			break
		}

		// Update the cursor to the last trace_id in the batch
		lastTID = batchTIDs[len(batchTIDs)-1]

		sklog.Infof("Promoting batch of %d matching internal historical traces to public...", len(batchTIDs))
		if err := p.promoteBatch(ctx, batchTIDs); err != nil {
			if ctx.Err() != nil {
				sklog.Infof("Gracefully exiting promotion sweep loop due to context deadline/cancellation during updates (already promoted %d traces in this sweep).", totalPromoted)
				return totalPromoted, ctx.Err()
			}
			return totalPromoted, err
		}

		totalPromoted += len(batchTIDs)
		if len(batchTIDs) < batchSelectLimit {
			break
		}
	}

	return totalPromoted, nil
}

func (p *Promoter) promoteBatch(ctx context.Context, tids [][]byte) error {
	err := util.ChunkIterParallelPool(ctx, len(tids), chunkUpdateSize, updateConcurrency, func(ctx context.Context, startIdx, endIdx int) error {
		chunk := tids[startIdx:endIdx]
		_, err := p.db.Exec(ctx, updateTraceVisibility, true, chunk)
		return err
	})
	if err != nil {
		return skerr.Wrapf(err, "failed to execute database visibility updates")
	}
	return nil
}

func buildPromotionQuery(ruleExprs []string, limit int, lastTID []byte) (string, []interface{}) {
	var clauses []string
	var args []interface{}
	placeholderIdx := 1
	for _, expr := range ruleExprs {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			continue
		}
		// Use static key name (parts[0]) directly in JSON path so the optimizer can utilize functional indexes.
		// Since tag/param keys are validated config identifiers, they are safe to put directly in the SQL query string.
		clauses = append(clauses, fmt.Sprintf("params ->> '%s' = $%d", parts[0], placeholderIdx))
		args = append(args, parts[1])
		placeholderIdx++
	}

	if len(clauses) == 0 {
		return "", nil
	}

	var cursorClause string
	if len(lastTID) > 0 {
		cursorClause = fmt.Sprintf(" AND trace_id > $%d", placeholderIdx)
		args = append(args, lastTID)
	}

	sqlQuery := fmt.Sprintf("SELECT trace_id FROM TraceParams WHERE COALESCE(is_public, false) = false AND (%s)%s ORDER BY trace_id ASC LIMIT %d", strings.Join(clauses, " OR "), cursorClause, limit)
	return sqlQuery, args
}
