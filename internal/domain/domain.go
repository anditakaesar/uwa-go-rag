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
	ID   *int64
	Name *string
}

type Permission struct {
	ID       int64
	Resource string
	Action   string
	Name     string
}

func ListPermissionName(permissions []Permission) []string {
	names := []string{}
	for _, p := range permissions {
		names = append(names, p.Name)
	}

	return names
}

type RolePermission struct {
	RoleID       int64
	PermissionID int64
}
