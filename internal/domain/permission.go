package domain

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
