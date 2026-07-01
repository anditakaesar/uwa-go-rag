package domain

import "time"

type Base struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt *time.Time
	DeletedAt *time.Time
}

type Role struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	IsSystem    bool
}

type FetchRoleParam struct {
	ID       *int64
	Username *string
}
