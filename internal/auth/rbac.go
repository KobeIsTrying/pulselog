package auth

// Role is an organization membership role. Permissions are defined only here.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type Permission string

const (
	PermOrgManage      Permission = "org.manage"
	PermMembersManage  Permission = "members.manage"
	PermProjectsManage Permission = "projects.manage"
	PermServicesManage Permission = "services.manage"
	PermAPIKeysManage  Permission = "apikeys.manage"
	PermLogsRead       Permission = "logs.read"
)

var rolePerms = map[Role][]Permission{
	RoleOwner: {
		PermOrgManage, PermMembersManage, PermProjectsManage,
		PermServicesManage, PermAPIKeysManage, PermLogsRead,
	},
	RoleAdmin: {
		PermServicesManage, PermAPIKeysManage, PermLogsRead,
	},
	RoleMember: {PermLogsRead},
	RoleViewer: {PermLogsRead},
}

func ParseRole(s string) (Role, bool) {
	r := Role(s)
	_, ok := rolePerms[r]
	return r, ok
}

func HasPermission(role Role, perm Permission) bool {
	for _, p := range rolePerms[role] {
		if p == perm {
			return true
		}
	}
	return false
}

func ValidRoles() []string {
	return []string{string(RoleOwner), string(RoleAdmin), string(RoleMember), string(RoleViewer)}
}
