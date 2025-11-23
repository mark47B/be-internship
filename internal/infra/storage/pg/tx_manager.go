package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/mark47B/be-internship/internal/domain/repository"
)

type TxManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) repository.TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) Do(ctx context.Context, fn func(context.Context) error) error {
	_, err := m.DoTx(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}

func (m *TxManager) DoTx(ctx context.Context, fn func(context.Context) (any, error)) (any, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			if !errors.Is(err, sql.ErrTxDone) {
				log.Printf("WARNING: tx.Rollback() failed: %v", err)
			}
		}
	}()
	ctx = withTx(ctx, tx)

	result, err := fn(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

// txKey — приватный ключ для хранения *sql.Tx в контексте
type txKey struct{}

// withTx — добавляет транзакцию в контекст
func withTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// querier — общий интерфейс для *sql.DB
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
