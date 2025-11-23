package repository

import "context"

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	DoTx(ctx context.Context, fn func(ctx context.Context) (any, error)) (any, error)
}
