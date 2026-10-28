package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// --- TransactionManager ---

type PostgresTransactionManager struct {
	pool *pgxpool.Pool
}

func NewPostgresTransactionManager(pool *pgxpool.Pool) *PostgresTransactionManager {
	return &PostgresTransactionManager{pool: pool}
}

func (tm *PostgresTransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
		}
	}()

	err = fn(context.WithValue(ctx, txKey, tx))
	return err
}
