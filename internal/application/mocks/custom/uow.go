package custom

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type IMockUow struct {
	mock.Mock
}

func (m *IMockUow) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	m.Mock.Called(ctx, fn)
	return fn(ctx)
}
