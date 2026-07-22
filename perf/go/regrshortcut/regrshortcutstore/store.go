package regrshortcutstore

import (
	"context"
	"crypto/md5"
	"database/sql"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/types"
)

// RegressionsShortcutStore implements the regrshortcut.Store interface.
type RegressionsShortcutStore struct {
	// db is the underlying database.
	db pool.Pool
}

// New returns a *RegressionsShortcutStore
func New(db pool.Pool) *RegressionsShortcutStore {
	return &RegressionsShortcutStore{
		db: db,
	}
}

// Create implements the regrshortcut.Store interface.
func (rss *RegressionsShortcutStore) Create(ctx context.Context, regrIdList []string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "regrshortcutstore.Create")
	defer span.End()

	slices.Sort(regrIdList)
	shortcut := rss.calcHash(regrIdList)

	if _, err := rss.db.Exec(ctx, `INSERT INTO RegressionsShortcuts(sid, anomaly_ids) VALUES ($1, $2)`, shortcut, regrIdList); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			// Shortcut is already present, we continue gracefully.
			// We don't guard against md5 collisions.
			return shortcut, nil
		}
		return "", skerr.Fmt("failed to write new regressions shortcut: %s", err)
	}
	return shortcut, nil
}

// Get implements the regrshortcut.Store interface.
func (rss *RegressionsShortcutStore) Get(ctx context.Context, shortcut string) (sql.NullBool, []string, error) {
	ctx, span := trace.StartSpan(ctx, "regrshortcutstore.Get")
	defer span.End()

	if !strings.HasPrefix(shortcut, "\\x") {
		shortcut = "\\x" + shortcut
	}
	var isLegacy sql.NullBool
	var regrIdList []string
	if err := rss.db.QueryRow(ctx, `SELECT is_legacy, anomaly_ids FROM RegressionsShortcuts WHERE sid = $1`, shortcut).Scan(&isLegacy, &regrIdList); err != nil {
		if err == pgx.ErrNoRows {
			return sql.NullBool{}, []string{}, nil
		}
		return sql.NullBool{}, nil, skerr.Wrapf(err, "failed to get regressions shortcut: %s", shortcut)
	}
	return isLegacy, regrIdList, nil
}

func (rss *RegressionsShortcutStore) calcHash(regrIdList []string) string {
	hash := md5.Sum([]byte(strings.Join(regrIdList, ",")))
	return string(types.TraceIDForSQLFromTraceIDAsBytes(hash[:]))
}
