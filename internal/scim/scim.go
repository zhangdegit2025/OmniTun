package scim

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	scimUserSchema  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimListSchema  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
)

type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type SCIMGroupRef struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

type SCIMGroupMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

type SCIMMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location"`
	Version      string    `json:"version"`
}

type SCIMUser struct {
	Schemas    []string        `json:"schemas"`
	ID         string          `json:"id"`
	ExternalID string          `json:"externalId,omitempty"`
	UserName   string          `json:"userName"`
	Name       SCIMUserName    `json:"name"`
	Emails     []SCIMEmail     `json:"emails"`
	Active     bool            `json:"active"`
	Groups     []SCIMGroupRef  `json:"groups,omitempty"`
	Meta       SCIMMeta        `json:"meta"`
}

type SCIMUserName struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

type SCIMGroup struct {
	Schemas     []string           `json:"schemas"`
	ID          string             `json:"id"`
	DisplayName string             `json:"displayName"`
	Members     []SCIMGroupMember  `json:"members,omitempty"`
	Meta        SCIMMeta           `json:"meta"`
}

type SCIMListResponse struct {
	Schemas      []string    `json:"schemas"`
	TotalResults int         `json:"totalResults"`
	StartIndex   int         `json:"startIndex"`
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    interface{} `json:"Resources"`
}

type SCIMMetaResponse struct {
	Schemas []string `json:"schemas"`
}

type SCIMError struct {
	Schemas  []string `json:"schemas"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail"`
	Status   int      `json:"status,string"`
}

type ServiceProviderConfig struct {
	Schemas               []string            `json:"schemas"`
	DocumentationURI      string              `json:"documentationUri"`
	Patch                 SupportedConfig     `json:"patch"`
	Bulk                  BulkConfig          `json:"bulk"`
	Filter                FilterConfig        `json:"filter"`
	ChangePassword        SupportedConfig     `json:"changePassword"`
	Sort                  SupportedConfig     `json:"sort"`
	Etag                  SupportedConfig     `json:"etag"`
	AuthenticationSchemes []AuthScheme        `json:"authenticationSchemes"`
}

type SupportedConfig struct {
	Supported bool `json:"supported"`
}

type BulkConfig struct {
	Supported   bool `json:"supported"`
	MaxOperations int `json:"maxOperations"`
	MaxPayloadSize int `json:"maxPayloadSize"`
}

type FilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type AuthScheme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SpecURI     string `json:"specUri"`
	Type        string `json:"type"`
	Primary     bool   `json:"primary"`
}

type ResourceType struct {
	Schemas    []string `json:"schemas"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Endpoint   string   `json:"endpoint"`
	Schema     string   `json:"schema"`
}

type Schema struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Attributes  []interface{} `json:"attributes"`
}

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := query.Get("filter")
	startIndex := 1
	count := 50
	if si := query.Get("startIndex"); si != "" {
		fmt.Sscanf(si, "%d", &startIndex)
	}
	if c := query.Get("count"); c != "" {
		fmt.Sscanf(c, "%d", &count)
	}

	ctx := r.Context()

	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	argIdx := 1

	if filter != "" {
		if strings.HasPrefix(filter, "userName eq ") {
			val := strings.Trim(strings.TrimPrefix(filter, "userName eq "), `"`)
			where += fmt.Sprintf(" AND email = $%d", argIdx)
			args = append(args, val)
			argIdx++
		} else if strings.HasPrefix(filter, "externalId eq ") {
			val := strings.Trim(strings.TrimPrefix(filter, "externalId eq "), `"`)
			where += fmt.Sprintf(" AND id::text = $%d", argIdx)
			args = append(args, val)
			argIdx++
		}
	}

	var total int
	if err := h.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total); err != nil {
		slog.Error("scim list users count failed", "error", err)
		writeSCIMError(w, http.StatusInternalServerError, "")
		return
	}

	offset := startIndex - 1
	if offset < 0 {
		offset = 0
	}

	sel := fmt.Sprintf(`SELECT id::text, email, display_name, role, created_at, updated_at FROM users %s ORDER BY created_at ASC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	selArgs := append(args, count, offset)
	rows, err := h.pool.Query(ctx, sel, selArgs...)
	if err != nil {
		slog.Error("scim list users failed", "error", err)
		writeSCIMError(w, http.StatusInternalServerError, "")
		return
	}
	defer rows.Close()

	users := make([]SCIMUser, 0)
	for rows.Next() {
		var id, email, displayName, role string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &email, &displayName, &role, &createdAt, &updatedAt); err != nil {
			continue
		}
		users = append(users, h.userToSCIM(id, email, displayName, createdAt, updatedAt))
	}

	resp := SCIMListResponse{
		Schemas:      []string{scimListSchema},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: count,
		Resources:    users,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var u SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax")
		return
	}
	if u.UserName == "" {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue")
		return
	}

	ctx := r.Context()
	email := u.UserName
	displayName := u.Name.GivenName + " " + u.Name.FamilyName
	if displayName == " " {
		displayName = u.UserName
	}

	externalID := sql.NullString{}
	if u.ExternalID != "" {
		externalID = sql.NullString{String: u.ExternalID, Valid: true}
	}

	orgSlug := emailToSlug(email)
	orgID := uuidStr()

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, plan) VALUES ($1, $2, $3, 'free')`,
		orgID, email, orgSlug,
	); err != nil {
		slog.Error("scim create org failed", "error", err)
	}

	userID := uuidStr()
	now := time.Now()

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO users (id, organization_id, email, display_name, role, external_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'member', $5, $6, $7)`,
		userID, orgID, email, displayName, externalID, now, now,
	); err != nil {
		slog.Error("scim create user failed", "error", err)
		writeSCIMError(w, http.StatusConflict, "uniqueness")
		return
	}

	result := h.userToSCIM(userID, email, displayName, now, now)
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var email, displayName string
	var createdAt, updatedAt time.Time
	err := h.pool.QueryRow(ctx,
		`SELECT email, display_name, created_at, updated_at FROM users WHERE id=$1 AND deleted_at IS NULL`, id,
	).Scan(&email, &displayName, &createdAt, &updatedAt)
	if err != nil {
		writeSCIMError(w, http.StatusNotFound, "")
		return
	}

	u := h.userToSCIM(id, email, displayName, createdAt, updatedAt)
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) ReplaceUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var u SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax")
		return
	}

	ctx := r.Context()
	displayName := u.Name.GivenName + " " + u.Name.FamilyName
	if displayName == " " {
		displayName = u.UserName
	}

	active := u.Active
	var deletedAt interface{}
	if !active {
		deletedAt = time.Now()
	} else {
		deletedAt = nil
	}

	externalID := sql.NullString{}
	if u.ExternalID != "" {
		externalID = sql.NullString{String: u.ExternalID, Valid: true}
	}

	now := time.Now()
	tag, err := h.pool.Exec(ctx,
		`UPDATE users SET email=$1, display_name=$2, updated_at=$3, deleted_at=$4, external_id=$5
		 WHERE id=$6 AND deleted_at IS NULL`,
		u.UserName, displayName, now, deletedAt, externalID, id,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeSCIMError(w, http.StatusNotFound, "")
		return
	}

	result := h.userToSCIM(id, u.UserName, displayName, now, now)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var patch SCIMPatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax")
		return
	}

	ctx := r.Context()

	if len(patch.Operations) == 0 {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue")
		return
	}

	now := time.Now()
	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if valueMap, ok := op.Value.(map[string]interface{}); ok {
				for k, v := range valueMap {
					switch k {
					case "active":
						if active, ok := v.(bool); ok {
							var deletedAt interface{}
							if !active {
								deletedAt = now
							} else {
								deletedAt = nil
							}
							h.pool.Exec(ctx, `UPDATE users SET updated_at=$1, deleted_at=$2 WHERE id=$3`, now, deletedAt, id)
						}
					case "userName":
						if username, ok := v.(string); ok {
							h.pool.Exec(ctx, `UPDATE users SET email=$1, updated_at=$2 WHERE id=$3`, username, now, id)
						}
					case "displayName":
						if dn, ok := v.(string); ok {
							h.pool.Exec(ctx, `UPDATE users SET display_name=$1, updated_at=$2 WHERE id=$3`, dn, now, id)
						}
					case "name.givenName":
						if gn, ok := v.(string); ok {
							h.pool.Exec(ctx, `UPDATE users SET display_name=$1, updated_at=$2 WHERE id=$3`, gn, now, id)
						}
					}
				}
			}
		}
	}

	var email, displayName string
	var createdAt time.Time
	if err := h.pool.QueryRow(ctx,
		`SELECT email, display_name, created_at FROM users WHERE id=$1 AND deleted_at IS NULL`, id,
	).Scan(&email, &displayName, &createdAt); err != nil {
		writeSCIMError(w, http.StatusNotFound, "")
		return
	}

	result := h.userToSCIM(id, email, displayName, createdAt, now)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	tag, err := h.pool.Exec(ctx,
		`UPDATE users SET deleted_at=$1, updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		time.Now(), id,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeSCIMError(w, http.StatusNotFound, "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startIndex := 1
	count := 50
	if si := r.URL.Query().Get("startIndex"); si != "" {
		fmt.Sscanf(si, "%d", &startIndex)
	}
	if c := r.URL.Query().Get("count"); c != "" {
		fmt.Sscanf(c, "%d", &count)
	}

	rows, err := h.pool.Query(ctx,
		`SELECT id::text, name, slug, created_at, updated_at FROM organizations WHERE deleted_at IS NULL ORDER BY created_at ASC`,
	)
	if err != nil {
		slog.Error("scim list groups failed", "error", err)
		writeSCIMError(w, http.StatusInternalServerError, "")
		return
	}
	defer rows.Close()

	groups := make([]SCIMGroup, 0)
	for rows.Next() {
		var id, name, slug string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &slug, &createdAt, &updatedAt); err != nil {
			continue
		}
		groups = append(groups, h.groupToSCIM(id, name, createdAt, updatedAt))
	}

	if groups == nil {
		groups = []SCIMGroup{}
	}

	resp := SCIMListResponse{
		Schemas:      []string{scimListSchema},
		TotalResults: len(groups),
		StartIndex:   startIndex,
		ItemsPerPage: count,
		Resources:    groups,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var g SCIMGroup
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax")
		return
	}
	if g.DisplayName == "" {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue")
		return
	}

	ctx := r.Context()
	orgID := uuidStr()
	slug := strings.ToLower(strings.ReplaceAll(g.DisplayName, " ", "-"))
	now := time.Now()

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, plan) VALUES ($1, $2, $3, 'free')`,
		orgID, g.DisplayName, slug,
	); err != nil {
		slog.Error("scim create group failed", "error", err)
		writeSCIMError(w, http.StatusConflict, "uniqueness")
		return
	}

	result := h.groupToSCIM(orgID, g.DisplayName, now, now)
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) PatchGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var patch SCIMPatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalidSyntax")
		return
	}

	ctx := r.Context()

	if len(patch.Operations) == 0 {
		writeSCIMError(w, http.StatusBadRequest, "invalidValue")
		return
	}

	now := time.Now()
	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if v, ok := op.Value.(map[string]interface{}); ok {
				if dn, ok := v["displayName"].(string); ok {
					h.pool.Exec(ctx, `UPDATE organizations SET name=$1, updated_at=$2 WHERE id=$3`, dn, now, id)
				}
			}
		case "add":
			if members, ok := op.Value.([]interface{}); ok {
				for _, m := range members {
					if member, ok := m.(map[string]interface{}); ok {
						if userID, ok := member["value"].(string); ok {
							h.pool.Exec(ctx,
								`UPDATE users SET organization_id=$1, updated_at=$2 WHERE id=$3`,
								id, now, userID,
							)
						}
					}
				}
			}
		case "remove":
			if path, ok := op.Path.(string); ok {
				if strings.HasPrefix(path, "members[value eq ") {
					userID := strings.Trim(strings.TrimPrefix(path, "members[value eq "), `"]`)
					h.pool.Exec(ctx,
						`UPDATE users SET organization_id=NULL, updated_at=$1 WHERE id=$2 AND organization_id=$3`,
						now, userID, id,
					)
				}
			}
		}
	}

	var name string
	var createdAt time.Time
	if err := h.pool.QueryRow(ctx,
		`SELECT name, created_at FROM organizations WHERE id=$1 AND deleted_at IS NULL`, id,
	).Scan(&name, &createdAt); err != nil {
		writeSCIMError(w, http.StatusNotFound, "")
		return
	}

	result := h.groupToSCIM(id, name, createdAt, now)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	cfg := ServiceProviderConfig{
		Schemas:          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		DocumentationURI: "https://omnitun.io/docs/scim",
		Patch:            SupportedConfig{Supported: true},
		Bulk:             BulkConfig{Supported: false, MaxOperations: 10, MaxPayloadSize: 1048576},
		Filter:           FilterConfig{Supported: true, MaxResults: 200},
		ChangePassword:   SupportedConfig{Supported: true},
		Sort:             SupportedConfig{Supported: false},
		Etag:             SupportedConfig{Supported: false},
		AuthenticationSchemes: []AuthScheme{
			{
				Name:        "OAuth Bearer Token",
				Description: "Authentication scheme using the OAuth Bearer Token Standard",
				SpecURI:     "http://www.rfc-editor.org/info/rfc6750",
				Type:        "oauthbearertoken",
				Primary:     true,
			},
		},
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) GetResourceTypes(w http.ResponseWriter, r *http.Request) {
	types := []ResourceType{
		{
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			ID:       "User",
			Name:     "User",
			Endpoint: "/scim/v2/Users",
			Schema:   scimUserSchema,
		},
		{
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			ID:       "Group",
			Name:     "Group",
			Endpoint: "/scim/v2/Groups",
			Schema:   scimGroupSchema,
		},
	}
	writeJSON(w, http.StatusOK, types)
}

func (h *Handler) GetSchemas(w http.ResponseWriter, r *http.Request) {
	userAttrs := []interface{}{
		map[string]interface{}{"name": "userName", "type": "string", "required": true, "caseExact": false, "mutability": "readWrite", "returned": "default", "uniqueness": "server"},
		map[string]interface{}{"name": "name", "type": "complex", "required": false, "mutability": "readWrite", "returned": "default", "subAttributes": []interface{}{
			map[string]interface{}{"name": "givenName", "type": "string", "required": false, "mutability": "readWrite", "returned": "default"},
			map[string]interface{}{"name": "familyName", "type": "string", "required": false, "mutability": "readWrite", "returned": "default"},
		}},
		map[string]interface{}{"name": "emails", "type": "complex", "multiValued": true, "mutability": "readWrite", "returned": "default"},
		map[string]interface{}{"name": "active", "type": "boolean", "mutability": "readWrite", "returned": "default"},
		map[string]interface{}{"name": "externalId", "type": "string", "mutability": "readWrite", "returned": "default"},
	}

	schemas := []Schema{
		{
			ID:          scimUserSchema,
			Name:        "User",
			Description: "User Account",
			Attributes:  userAttrs,
		},
		{
			ID:          scimGroupSchema,
			Name:        "Group",
			Description: "Group",
			Attributes: []interface{}{
				map[string]interface{}{"name": "displayName", "type": "string", "required": true, "mutability": "readWrite", "returned": "default"},
				map[string]interface{}{"name": "members", "type": "complex", "multiValued": true, "mutability": "readWrite", "returned": "default"},
			},
		},
	}

	resp := map[string]interface{}{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": len(schemas),
		"Resources":    schemas,
	}
	writeJSON(w, http.StatusOK, resp)
}

type SCIMPatchOp struct {
	Schemas    []string               `json:"schemas"`
	Operations []SCIMPatchOperation   `json:"Operations"`
}

type SCIMPatchOperation struct {
	Op    string      `json:"op"`
	Path  interface{} `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

func (h *Handler) userToSCIM(id, email, displayName string, createdAt, updatedAt time.Time) SCIMUser {
	parts := strings.SplitN(displayName, " ", 2)
	givenName := displayName
	familyName := ""
	if len(parts) == 2 {
		givenName = parts[0]
		familyName = parts[1]
	}

	return SCIMUser{
		Schemas:  []string{scimUserSchema},
		ID:       id,
		UserName: email,
		Name: SCIMUserName{
			GivenName:  givenName,
			FamilyName: familyName,
		},
		Emails: []SCIMEmail{
			{Value: email, Type: "work", Primary: true},
		},
		Active: true,
		Meta: SCIMMeta{
			ResourceType: "User",
			Created:      createdAt,
			LastModified: updatedAt,
			Location:     "/scim/v2/Users/" + id,
			Version:      fmt.Sprintf("W/\"%x\"", updatedAt.Unix()),
		},
	}
}

func (h *Handler) groupToSCIM(id, displayName string, createdAt, updatedAt time.Time) SCIMGroup {
	return SCIMGroup{
		Schemas:     []string{scimGroupSchema},
		ID:          id,
		DisplayName: displayName,
		Meta: SCIMMeta{
			ResourceType: "Group",
			Created:      createdAt,
			LastModified: updatedAt,
			Location:     "/scim/v2/Groups/" + id,
			Version:      fmt.Sprintf("W/\"%x\"", updatedAt.Unix()),
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeSCIMError(w http.ResponseWriter, status int, scimType string) {
	err := SCIMError{
		Schemas: []string{scimErrorSchema},
		Status:  status,
		Detail:  http.StatusText(status),
	}
	if scimType != "" {
		err.ScimType = scimType
	}
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(err)
}

func SCIMAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeSCIMError(w, http.StatusUnauthorized, "")
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeSCIMError(w, http.StatusUnauthorized, "")
				return
			}
			if parts[1] != token {
				writeSCIMError(w, http.StatusUnauthorized, "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func uuidStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", fmtHex(b[0:4]), fmtHex(b[4:6]), fmtHex(b[6:8]), fmtHex(b[8:10]), fmtHex(b[10:]))
}

func fmtHex(b []byte) string {
	return strings.ToLower(fmt.Sprintf("%x", b))
}

func emailToSlug(email string) string {
	slug := ""
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r == '@' || r == '.' || r == '_' {
			slug += "-"
		}
	}
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

func RegisterRoutes(r chi.Router, h *Handler, scimToken string) {
	r.Route("/scim/v2", func(r chi.Router) {
		r.Use(SCIMAuthMiddleware(scimToken))

		r.Get("/Users", h.ListUsers)
		r.Post("/Users", h.CreateUser)
		r.Get("/Users/{id}", h.GetUser)
		r.Put("/Users/{id}", h.ReplaceUser)
		r.Patch("/Users/{id}", h.PatchUser)
		r.Delete("/Users/{id}", h.DeleteUser)

		r.Get("/Groups", h.ListGroups)
		r.Post("/Groups", h.CreateGroup)
		r.Patch("/Groups/{id}", h.PatchGroup)

		r.Get("/ServiceProviderConfig", h.GetServiceProviderConfig)
		r.Get("/ResourceTypes", h.GetResourceTypes)
		r.Get("/Schemas", h.GetSchemas)
	})
}
