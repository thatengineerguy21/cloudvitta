package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Transactor defines the contract for executing queries within an atomic database transaction.
// NOTE: This abstracts transaction lifecycle boundaries (Begin/Commit/Rollback), which sqlc does not
// generate, rather than wrapping Queries methods into a repository layer.
type Transactor interface {
	ExecTx(ctx context.Context, fn func(q Querier) error) error
}

// PoolTransactor wraps *pgxpool.Pool to implement transaction execution.
type PoolTransactor struct {
	pool *pgxpool.Pool
}

// NewTransactor creates a new PoolTransactor instance.
func NewTransactor(pool *pgxpool.Pool) *PoolTransactor {
	return &PoolTransactor{pool: pool}
}

// ExecTx runs fn within an isolated database transaction, rolling back on error and committing on success.
func (t *PoolTransactor) ExecTx(ctx context.Context, fn func(q Querier) error) error {
	if t == nil || t.pool == nil {
		return fmt.Errorf("store: database pool unavailable for transaction")
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := New(tx)
	if err := fn(qtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit transaction: %w", err)
	}
	return nil
}
