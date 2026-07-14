package common

import (
	"fmt"

	"github.com/henvic/pgq"
)

type Pagination struct {
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Total int64 `json:"total"`
}

func (p *Pagination) GetOffset() int {
	offset := (p.Page - 1) * p.Size
	return offset
}

func (p *Pagination) Normalize() {
	if p.Size > 100 || p.Size < 1 {
		p.Size = 10
	}

	if p.Page < 1 {
		p.Page = 1
	}
}

func (p *Pagination) WrapPaging(sb *pgq.SelectBuilder) {
	if p.Size > 0 {
		*sb = sb.Limit(uint64(p.Size))
	}

	if p.Page > 0 {
		*sb = sb.Offset(uint64(p.GetOffset()))
	}
}

type SortDirection string

const (
	SORT_ASC  SortDirection = "ASC"
	SORT_DESC SortDirection = "DESC"
)

type Sort struct {
	Field     string
	Direction SortDirection
}

func (s *Sort) ToSQLSort() string {
	return fmt.Sprintf("%s %s", s.Field, s.Direction)
}

// Transaction key for context
type txCtxKey string

const TxKey txCtxKey = "TX_KEY"
