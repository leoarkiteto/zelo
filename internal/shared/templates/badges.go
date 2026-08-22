package templates

// RoleBadgeClass maps an RBAC role to its badge style.
func RoleBadgeClass(role string) string {
	switch role {
	case "syndic":
		return "badge badge-warning"
	case "tenant":
		return "badge badge-neutral"
	default:
		return "badge badge-info"
	}
}
