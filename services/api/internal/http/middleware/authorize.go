package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// Permission represents a specific action on a resource.
type Permission string

const (
	// Video permissions
	PermVideoCreate  Permission = "video:create"
	PermVideoRead    Permission = "video:read"
	PermVideoUpdate  Permission = "video:update"
	PermVideoDelete  Permission = "video:delete"
	PermVideoList    Permission = "video:list"

	// User permissions
	PermUserCreate  Permission = "user:create"
	PermUserRead    Permission = "user:read"
	PermUserUpdate  Permission = "user:update"
	PermUserDelete  Permission = "user:delete"

	// Admin permissions
	PermAdminPanel   Permission = "admin:panel"
	PermAdminUsers   Permission = "admin:users"
	PermAdminVideos  Permission = "admin:videos"
	PermAdminSystem  Permission = "admin:system"

	// Moderation permissions
	PermModReportsList     Permission = "moderation:reports:list"
	PermModReportsAssign   Permission = "moderation:reports:assign"
	PermModReportsResolve  Permission = "moderation:reports:resolve"
	PermModActionsHide     Permission = "moderation:actions:hide"
	PermModActionsUnpublish Permission = "moderation:actions:unpublish"
	PermModActionsDelete   Permission = "moderation:actions:delete"
	PermModActionsSuspend  Permission = "moderation:actions:suspend"
	PermModActionsBan      Permission = "moderation:actions:ban"
	PermModActionsRestore  Permission = "moderation:actions:restore"
	PermModAuditView       Permission = "moderation:audit:view"
	PermModNotesManage     Permission = "moderation:notes:manage"
	PermModAppealsReview   Permission = "moderation:appeals:review"
)

// RolePermissions maps roles to their allowed permissions.
var RolePermissions = map[string][]Permission{
	"admin": {
		PermVideoCreate, PermVideoRead, PermVideoUpdate, PermVideoDelete, PermVideoList,
		PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete,
		PermAdminPanel, PermAdminUsers, PermAdminVideos, PermAdminSystem,
	},
	"moderator": {
		PermVideoRead, PermVideoUpdate, PermVideoList,
		PermUserRead,
		PermAdminPanel, PermAdminVideos,
	},
	"user": {
		PermVideoCreate, PermVideoRead, PermVideoUpdate, PermVideoDelete, PermVideoList,
	},
	"service": {
		PermVideoRead, PermVideoList, PermUserRead,
	},
}

// RequirePermission returns a middleware that checks if the authenticated user
// has the required permission.
func RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, claims := GetUserRole(r), GetTokenClaims(r)

			if role == "" {
				WriteUnauthorized(w, "authentication required")
				return
			}

			// Check role-based permissions
			permissions, ok := RolePermissions[role]
			if ok {
				for _, p := range permissions {
					if p == permission {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			// Check if the user has individual permissions in their claims
			if claims != nil {
				for _, p := range claims.Permissions {
					if Permission(p) == permission {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			WriteForbidden(w, fmt.Sprintf("insufficient permissions: %s required", permission))
		})
	}
}

// RequireAnyPermission returns a middleware that checks if the authenticated user
// has at least one of the required permissions.
func RequireAnyPermission(permissions ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, claims := GetUserRole(r), GetTokenClaims(r)

			if role == "" {
				WriteUnauthorized(w, "authentication required")
				return
			}

			// Check role-based permissions
			rolePerms, ok := RolePermissions[role]
			if ok {
				for _, required := range permissions {
					for _, p := range rolePerms {
						if p == required {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			// Check individual permissions in claims
			if claims != nil {
				for _, required := range permissions {
					for _, p := range claims.Permissions {
						if Permission(p) == required {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			var permNames []string
			for _, p := range permissions {
				permNames = append(permNames, string(p))
			}
			WriteForbidden(w, fmt.Sprintf("insufficient permissions: one of [%s] required", strings.Join(permNames, ", ")))
		})
	}
}

// RequireAllPermissions returns a middleware that checks if the authenticated user
// has all of the required permissions.
func RequireAllPermissions(permissions ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, claims := GetUserRole(r), GetTokenClaims(r)

			if role == "" {
				WriteUnauthorized(w, "authentication required")
				return
			}

			// Collect all permissions the user has
			userPerms := make(map[Permission]bool)

			rolePerms, ok := RolePermissions[role]
			if ok {
				for _, p := range rolePerms {
					userPerms[p] = true
				}
			}

			if claims != nil {
				for _, p := range claims.Permissions {
					userPerms[Permission(p)] = true
				}
			}

			// Check all required permissions
			for _, required := range permissions {
				if !userPerms[required] {
					WriteForbidden(w, fmt.Sprintf("insufficient permissions: %s required", required))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireResourceOwner checks that the authenticated user owns the resource
// or has elevated permissions. The resourceIDFunc extracts the resource owner ID
// from the request.
func RequireResourceOwner(resourceIDFunc func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r)
			role := GetUserRole(r)

			if userID == "" {
				WriteUnauthorized(w, "authentication required")
				return
			}

			// Admins and moderators can access any resource
			if role == "admin" || role == "moderator" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if the user owns the resource
			resourceOwnerID := resourceIDFunc(r)
			if resourceOwnerID == "" {
				WriteNotFound(w, "resource not found")
				return
			}

			if resourceOwnerID != userID {
				WriteForbidden(w, "you do not own this resource")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}