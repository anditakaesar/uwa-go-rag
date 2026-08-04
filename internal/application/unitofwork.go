package application

import "context"

// adapters
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
