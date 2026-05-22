package roles

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomRole struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	AssignedUsers int    `json:"assigned_users"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type UpdateRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

var ValidPermissions = []string{
	"tunnels:create", "tunnels:read", "tunnels:update", "tunnels:delete", "tunnels:start", "tunnels:stop",
	"domains:create", "domains:read", "domains:update", "domains:delete",
	"networks:create", "networks:read", "networks:update", "networks:delete", "networks:join", "networks:leave",
	"billing:read", "billing:manage",
	"members:read", "members:invite", "members:remove", "members:change_role",
	"settings:read", "settings:update",
}

var RoleTemplates = map[string][]string{
	"Security Auditor": {"tunnels:read", "domains:read", "networks:read", "members:read", "billing:read", "settings:read"},
	"Billing Manager":  {"billing:read", "billing:manage", "tunnels:read"},
	"Developer":        {"tunnels:create", "tunnels:read", "tunnels:update", "tunnels:start", "tunnels:stop", "domains:create", "domains:read", "domains:update", "networks:create", "networks:read", "networks:join"},
	"Read-only Support": {"tunnels:read", "domains:read", "networks:read", "members:read", "billing:read", "settings:read"},
}

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.pool.Query(ctx,
		`SELECT id::text, name, permissions, created_at, updated_at FROM custom_roles ORDER BY created_at DESC`,
	)
	if err != nil {
		slog.Error("list roles failed", "error", err)
		respondJSON(w, http.StatusOK, map[string]interface{}{"roles": []interface{}{}})
		return
	}
	defer rows.Close()

	roles := make([]CustomRole, 0)
	for rows.Next() {
		var id, name string
		var perms []string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &perms, &createdAt, &updatedAt); err != nil {
			continue
		}

		var assignedUsers int
		h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role=$1 AND deleted_at IS NULL`, name).Scan(&assignedUsers)

		roles = append(roles, CustomRole{
			ID:            id,
			Name:          name,
			Permissions:   perms,
			AssignedUsers: assignedUsers,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		})
	}

	if roles == nil {
		roles = []CustomRole{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"roles":    roles,
		"templates": RoleTemplates,
	})
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if len(req.Permissions) == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "at least one permission is required")
		return
	}

	validPerms := filterValidPermissions(req.Permissions)
	if len(validPerms) == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "no valid permissions provided")
		return
	}

	ctx := r.Context()
	id := uuidStr()
	now := time.Now()

	_, err := h.pool.Exec(ctx,
		`INSERT INTO custom_roles (id, name, permissions, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		id, req.Name, validPerms, now, now,
	)
	if err != nil {
		slog.Error("create role failed", "error", err)
		respondError(w, http.StatusConflict, "conflict", "role name may already exist")
		return
	}

	role := CustomRole{
		ID:            id,
		Name:          req.Name,
		Permissions:   validPerms,
		AssignedUsers: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	respondJSON(w, http.StatusCreated, role)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	ctx := r.Context()
	now := time.Now()

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", argIdx))
		args = append(args, req.Name)
		argIdx++
	}
	if len(req.Permissions) > 0 {
		validPerms := filterValidPermissions(req.Permissions)
		setClauses = append(setClauses, fmt.Sprintf("permissions=$%d", argIdx))
		args = append(args, validPerms)
		argIdx++
	}

	if len(setClauses) == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "no fields to update")
		return
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at=$%d", argIdx))
	args = append(args, now)
	argIdx++
	args = append(args, id)

	query := fmt.Sprintf("UPDATE custom_roles SET %s WHERE id=$%d", strings.Join(setClauses, ", "), argIdx)

	tag, err := h.pool.Exec(ctx, query, args...)
	if err != nil || tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}

	var name string
	var perms []string
	var createdAt, updatedAt time.Time
	h.pool.QueryRow(ctx,
		`SELECT name, permissions, created_at, updated_at FROM custom_roles WHERE id=$1`, id,
	).Scan(&name, &perms, &createdAt, &updatedAt)

	var assignedUsers int
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role=$1 AND deleted_at IS NULL`, name).Scan(&assignedUsers)

	role := CustomRole{
		ID:            id,
		Name:          name,
		Permissions:   perms,
		AssignedUsers: assignedUsers,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	respondJSON(w, http.StatusOK, role)
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var name string
	err := h.pool.QueryRow(ctx, `SELECT name FROM custom_roles WHERE id=$1`, id).Scan(&name)
	if err != nil {
		respondError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}

	var assignedUsers int
	h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role=$1 AND deleted_at IS NULL`, name).Scan(&assignedUsers)
	if assignedUsers > 0 {
		respondError(w, http.StatusConflict, "has_users", fmt.Sprintf("role has %d assigned user(s); reassign them before deleting", assignedUsers))
		return
	}

	tag, err := h.pool.Exec(ctx, `DELETE FROM custom_roles WHERE id=$1`, id)
	if err != nil || tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates":        RoleTemplates,
		"valid_permissions": ValidPermissions,
	})
}

func filterValidPermissions(perms []string) []string {
	validSet := make(map[string]bool, len(ValidPermissions))
	for _, p := range ValidPermissions {
		validSet[p] = true
	}
	filtered := make([]string, 0, len(perms))
	seen := make(map[string]bool)
	for _, p := range perms {
		if validSet[p] && !seen[p] {
			filtered = append(filtered, p)
			seen[p] = true
		}
	}
	return filtered
}

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}

func uuidStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", fmtHex(b[0:4]), fmtHex(b[4:6]), fmtHex(b[6:8]), fmtHex(b[8:10]), fmtHex(b[10:]))
}

func fmtHex(b []byte) string {
	return fmt.Sprintf("%x", b)
}
