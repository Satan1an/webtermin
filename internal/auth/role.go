package auth

// Role identifies what a panel user is allowed to do.
//
// The model is hierarchical: any check `AtLeast(have, RoleOperator)` matches
// users with role Operator or Admin. The admin role is special — only admins
// may manage panel users, change roles, view the audit log, and perform
// system-level mutations like creating Linux users.
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

var roleRank = map[Role]int{
	RoleViewer:   1,
	RoleOperator: 2,
	RoleAdmin:    3,
}

// ValidRole reports whether s is a recognised role.
func ValidRole(s string) bool {
	_, ok := roleRank[Role(s)]
	return ok
}

// AtLeast reports whether the role `have` is at least as privileged as `want`.
// Unknown / empty roles are treated as below viewer (denied everything).
func AtLeast(have, want Role) bool {
	h, ok := roleRank[have]
	if !ok {
		return false
	}
	w := roleRank[want]
	return h >= w
}

// AllRoles returns every defined role in ascending order of privilege.
// Useful for UI dropdowns.
func AllRoles() []Role {
	return []Role{RoleViewer, RoleOperator, RoleAdmin}
}
