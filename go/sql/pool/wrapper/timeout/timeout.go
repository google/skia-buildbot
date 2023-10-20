// Package timeout provides a wrapper for pool.Pool that confirms every passed
// in context.Context has a timeout.
package timeout

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/ctxutil"
	"go.skia.org/infra/go/sql/pool"
)

// ContextTimeout implements pool.Pool and confirms that every passed in
// context.Context has a timeout.
type ContextTimeout struct {
	db pool.Pool
}

// New returns a ContextTimeout that wraps the given pool.Pool.
func New(db pool.Pool) ContextTimeout {
	return ContextTimeout{db: db}
}

// Close implements pool.Pool.
func (c ContextTimeout) Close() {
	c.db.Close()
}

// Acquire implements pool.Pool.
func (c ContextTimeout) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.Acquire(ctx)
}

// AcquireFunc implements pool.Pool.
func (c ContextTimeout) AcquireFunc(ctx context.Context, f func(*pgxpool.Conn) error) error {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.AcquireFunc(ctx, f)
}

// AcquireAllIdle implements pool.Pool.
func (c ContextTimeout) AcquireAllIdle(ctx context.Context) []*pgxpool.Conn {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.AcquireAllIdle(ctx)
}

// Config implements pool.Pool.
func (c ContextTimeout) Config() *pgxpool.Config {
	return c.db.Config()
}

// Exec implements pool.Pool.
func (c ContextTimeout) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.Exec(ctx, sql, arguments...)
}

// Query implements pool.Pool.
func (c ContextTimeout) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.Query(ctx, sql, args...)
}

// QueryRow implements pool.Pool.
func (c ContextTimeout) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.QueryRow(ctx, sql, args...)
}

// QueryFunc implements pool.Pool.
func (c ContextTimeout) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.QueryFunc(ctx, sql, args, scans, f)
}

// SendBatch implements pool.Pool.
func (c ContextTimeout) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.SendBatch(ctx, b)
}

// Begin implements pool.Pool.
func (c ContextTimeout) Begin(ctx context.Context) (pgx.Tx, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.Begin(ctx)
}

// BeginTx implements pool.Pool.
func (c ContextTimeout) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.BeginTx(ctx, txOptions)
}

// BeginFunc implements pool.Pool.
func (c ContextTimeout) BeginFunc(ctx context.Context, f func(pgx.Tx) error) error {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.BeginFunc(ctx, f)
}

// BeginTxFunc implements pool.Pool.
func (c ContextTimeout) BeginTxFunc(ctx context.Context, txOptions pgx.TxOptions, f func(pgx.Tx) error) error {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.BeginTxFunc(ctx, txOptions, f)
}

// CopyFrom implements pool.Pool.
func (c ContextTimeout) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

// Ping implements pool.Pool.
func (c ContextTimeout) Ping(ctx context.Context) error {
	ctxutil.ConfirmContextHasDeadline(ctx)
	return c.db.Ping(ctx)
}

// Confirm ContextTimeout implements pool.Pool.
var _ pool.Pool = ContextTimeout{}
