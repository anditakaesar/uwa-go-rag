package common

import "fmt"

type Pagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
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
