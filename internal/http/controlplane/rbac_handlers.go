package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
	"github.com/kumarabd/policy-machine/pkg/rbac"
	"gorm.io/datatypes"
)

// --- RBAC Subjects ---

// UpsertRBACSubjects upserts a batch of subjects.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.UpsertSubjectsRequest true "RBAC subjects batch upsert"
// @Success 204 "No Content"
// @Router /api/v1/rbac/subjects:upsert [post]
func (s *Server) UpsertRBACSubjects(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.UpsertSubjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for _, sSubj := range req.Subjects {
		if sSubj.Name == "" {
			continue
		}
		apiReq := api.CreateSubjectRequest{
			Name:     sSubj.Name,
			Kind:     sSubj.Kind,
			Metadata: sSubj.Metadata,
		}
		subject := mapper.CreateSubjectRequestToPostgres(apiReq)
		if subject.Tags == nil {
			subject.Tags = datatypes.JSON("{}")
		}
		if _, err := s.engine.GetDB().CreateSubject(r.Context(), tenantID, subject); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRBACSubject creates a subject node from RBAC Subject.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.Subject true "RBAC subject"
// @Success 201 {object} rbac.Subject
// @Router /api/v1/rbac/subjects [post]
func (s *Server) CreateRBACSubject(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.Subject
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	apiReq := api.CreateSubjectRequest{
		Name:     req.Name,
		Kind:     req.Kind,
		Metadata: req.Metadata,
	}
	subject := mapper.CreateSubjectRequestToPostgres(apiReq)
	if subject.Tags == nil {
		subject.Tags = datatypes.JSON("{}")
	}

	if _, err := s.engine.GetDB().CreateSubject(r.Context(), tenantID, subject); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	apiView := mapper.Subject(subject)
	res := rbac.Subject{
		ID:       apiView.ID.String(),
		Name:     apiView.Name,
		Kind:     apiView.Kind,
		Metadata: apiView.Metadata,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// GetRBACSubject returns a subject by ID.
// @Tags rbac
// @Produce json
// @Param id path string true "Subject ID"
// @Success 200 {object} rbac.Subject
// @Router /api/v1/rbac/subjects/{id} [get]
func (s *Server) GetRBACSubject(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	dbSubj, err := s.engine.GetDB().GetSubject(r.Context(), tenantID, id)
	if err != nil {
		httputil.HandleDBError(w, err, "Subject")
		return
	}
	apiSubj := mapper.Subject(dbSubj)
	res := rbac.Subject{
		ID:       apiSubj.ID.String(),
		Name:     apiSubj.Name,
		Kind:     apiSubj.Kind,
		Metadata: apiSubj.Metadata,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// ListRBACRoles lists roles (backed by custom subject attributes) with pagination.
// @Tags rbac
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} api.SearchResponse[rbac.Role]
// @Router /api/v1/rbac/roles [get]
func (s *Server) ListRBACRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	query := r.URL.Query().Get("query")
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	// Reuse ListSubjectSets (custom subject attributes)
	uas, nextCursor, hasMore, err := s.engine.GetDB().ListSubjectSets(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	items := make([]rbac.Role, len(uas))
	for i, ua := range uas {
		items[i] = rbac.Role{
			ID:          ua.ID.String(),
			Name:        ua.Name,
			Description: "",
			Permissions: nil, // Listing does not yet resolve full permissions
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(items)

	resp := api.SearchResponse[rbac.Role]{
		Items:      items,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- RBAC Roles ---

// UpsertRBACRoles upserts roles as custom subject attributes with permissions.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.UpsertRolesRequest true "RBAC roles batch upsert"
// @Success 204 "No Content"
// @Router /api/v1/rbac/roles:upsert [post]
func (s *Server) UpsertRBACRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.UpsertRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	db := s.engine.GetDB()

	// Ensure parent nodes for roles and clusterroles exist
	roleParent := &postgres.SubjectAttribute{Name: "role", AttributeType: postgres.AttributeTypeCustom}
	if _, err := db.CreateSubjectSet(r.Context(), tenantID, roleParent); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	clusterRoleParent := &postgres.SubjectAttribute{Name: "clusterrole", AttributeType: postgres.AttributeTypeCustom}
	if _, err := db.CreateSubjectSet(r.Context(), tenantID, clusterRoleParent); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	for _, role := range req.Roles {
		if role.Name == "" {
			continue
		}

		// Choose parent: clusterrole vs role
		parent := roleParent
		if strings.HasPrefix(role.Name, "clusterrole/") {
			parent = clusterRoleParent
		}

		// Create or get subject set (role) and attach to parent.
		ua := &postgres.SubjectAttribute{Name: role.Name, AttributeType: postgres.AttributeTypeCustom}
		if _, err := db.CreateSubjectAttributeWithParent(r.Context(), tenantID, ua, "", &parent.ID); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}

		// For each permission, ensure OA and create/extend association with verbs (ops).
		for _, p := range role.Permissions {
			if p.Action == "" || p.ObjectAttribute == "" {
				continue
			}
			oa := &postgres.ObjectAttribute{Name: p.ObjectAttribute, AttributeType: postgres.AttributeTypeCustom}
			if _, err := db.CreateObjectSet(r.Context(), tenantID, oa); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
			if _, err := db.CreateAssociationForAttributes(r.Context(), tenantID, ua.ID, oa.ID, []string{p.Action}); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRBACRole creates a role as a custom subject attribute with permissions.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.Role true "RBAC role"
// @Success 201 {object} rbac.Role
// @Router /api/v1/rbac/roles [post]
func (s *Server) CreateRBACRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.Role
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	db := s.engine.GetDB()

	// Ensure parent nodes for roles and clusterroles exist
	roleParent := &postgres.SubjectAttribute{Name: "role", AttributeType: postgres.AttributeTypeCustom}
	if _, err := db.CreateSubjectSet(r.Context(), tenantID, roleParent); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	clusterRoleParent := &postgres.SubjectAttribute{Name: "clusterrole", AttributeType: postgres.AttributeTypeCustom}
	if _, err := db.CreateSubjectSet(r.Context(), tenantID, clusterRoleParent); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Choose parent: clusterrole vs role
	parent := roleParent
	if strings.HasPrefix(req.Name, "clusterrole/") {
		parent = clusterRoleParent
	}

	// Create or get subject set (role) and attach to parent.
	ua := &postgres.SubjectAttribute{Name: req.Name, AttributeType: postgres.AttributeTypeCustom}
	if _, err := db.CreateSubjectAttributeWithParent(r.Context(), tenantID, ua, "", &parent.ID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// For each permission, ensure OA and create association
		for _, p := range req.Permissions {
		if p.Action == "" || p.ObjectAttribute == "" {
			continue
		}
		oa := &postgres.ObjectAttribute{Name: p.ObjectAttribute, AttributeType: postgres.AttributeTypeCustom}
		if _, err := db.CreateObjectSet(r.Context(), tenantID, oa); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if _, err := db.CreateAssociationForAttributes(r.Context(), tenantID, ua.ID, oa.ID, []string{p.Action}); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	res := rbac.Role{
		ID:          ua.ID.String(),
		Name:        ua.Name,
		Description: req.Description,
		Permissions: req.Permissions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// --- RBAC Bindings ---

// UpsertRBACBindings assigns subjects (by name) to roles (by name) in batch.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.UpsertBindingsRequest true "RBAC bindings batch upsert"
// @Success 204 "No Content"
// @Router /api/v1/rbac/bindings:upsert [post]
func (s *Server) UpsertRBACBindings(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.UpsertBindingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	db := s.engine.GetDB()

	for _, b := range req.Bindings {
		if b.RoleName == "" || len(b.SubjectNames) == 0 {
			continue
		}

		// Resolve role subject attribute by name.
		var ua postgres.SubjectAttribute
		if err := db.H.WithContext(r.Context()).
			Where("tenant_id = ? AND name = ?", tenantID, b.RoleName).
			First(&ua).Error; err != nil {
			if err == postgres.ErrNotFound {
				httputil.RespondError(w, http.StatusBadRequest, "ROLE_NOT_FOUND", "role "+b.RoleName+" not found")
				return
			}
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}

		// Resolve/create subjects by name and assign to role.
		for _, subjName := range b.SubjectNames {
			if subjName == "" {
				continue
			}

			// Find or create subject by external_id=name.
			subject := &postgres.Subject{
				ExternalID:  subjName,
				Display:     subjName,
				DisplayName: subjName,
			}
			if _, err := db.CreateSubject(r.Context(), tenantID, subject); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}

			edge := &postgres.AssignmentEdge{
				ChildType:  postgres.NodeSubject,
				ChildID:    subject.ID,
				ParentType: postgres.NodeUA,
				ParentID:   ua.ID,
			}
			if _, err := db.CreateRelationship(r.Context(), tenantID, edge); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRBACBinding assigns subjects to a role (subject attribute).
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.Binding true "RBAC binding"
// @Success 204 "No Content"
// @Router /api/v1/rbac/bindings [post]
func (s *Server) CreateRBACBinding(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.Binding
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid roleId")
		return
	}

	for _, sid := range req.SubjectIDs {
		subjectID, err := uuid.Parse(sid)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subjectId: "+sid)
			return
		}
		edge := &postgres.AssignmentEdge{
			ChildType:  postgres.NodeSubject,
			ChildID:    subjectID,
			ParentType: postgres.NodeUA,
			ParentID:   roleID,
		}
		if _, err := s.engine.GetDB().CreateRelationship(r.Context(), tenantID, edge); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	// Refresh engine for new assignments
	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- RBAC Objects ---

// UpsertRBACObjects upserts objects and their attributes.
// @Tags rbac
// @Accept json
// @Produce json
// @Param request body rbac.UpsertObjectsRequest true "RBAC objects upsert"
// @Success 204 "No Content"
// @Router /api/v1/rbac/objects:upsert [post]
func (s *Server) UpsertRBACObjects(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req rbac.UpsertObjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for _, o := range req.Objects {
		if o.Name == "" {
			continue
		}
		apiReq := api.CreateObjectRequest{
			Name:     o.Name,
			Kind:     o.Kind,
			Metadata: o.Metadata,
		}
		obj := mapper.CreateObjectRequestToPostgres(apiReq)
		if obj.Tags == nil {
			obj.Tags = datatypes.JSON("{}")
		}
		db := s.engine.GetDB()
		if _, err := db.CreateObject(r.Context(), tenantID, obj); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}

		// Compute diff for object attributes: remove disappeared attributes for this object.
		// Load current OBJECT -> OA(child) edges.
		var edges []postgres.AssignmentEdge
		if err := db.H.WithContext(r.Context()).
			Where("tenant_id = ? AND child_type = ? AND child_id = ? AND parent_type = ?",
				tenantID, postgres.NodeObject, obj.ID, postgres.NodeOA).
			Find(&edges).Error; err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}

		// Desired attribute values (value side) as a set.
		desiredValues := make(map[string]struct{})
		for _, value := range o.Attributes {
			if value == "" {
				continue
			}
			desiredValues[value] = struct{}{}
		}

		// For each existing edge, check if its value OA name is still desired; if not, remove the edge.
		for _, e := range edges {
			var oa postgres.ObjectAttribute
			if err := db.H.WithContext(r.Context()).
				Where("tenant_id = ? AND id = ?", tenantID, e.ParentID).
				First(&oa).Error; err != nil {
				if err == postgres.ErrNotFound {
					continue
				}
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}

			if _, ok := desiredValues[oa.Name]; ok {
				continue
			}

			// This value attribute is no longer present for the object; remove the relationship.
			edge := &postgres.AssignmentEdge{
				ChildType:  postgres.NodeObject,
				ChildID:    obj.ID,
				ParentType: postgres.NodeOA,
				ParentID:   oa.ID,
			}
			if _, err := db.DeleteRelationship(r.Context(), tenantID, edge); err != nil {
				if err == postgres.ErrNotFound {
					continue
				}
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
		}

		// Ensure attributes and assignments.
		// For each key:value, we create:
		// - parent OA with name=key
		// - child OA with name=value
		// - OA(child) -> OA(parent) edge
		// - OBJECT -> OA(child) edge
		for key, value := range o.Attributes {
			if key == "" || value == "" {
				continue
			}

			// Parent attribute: key
			parentOA := &postgres.ObjectAttribute{Name: key, AttributeType: postgres.AttributeTypeCustom}
			if _, err := db.CreateObjectSet(r.Context(), tenantID, parentOA); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}

			// Child attribute: value
			childOA := &postgres.ObjectAttribute{Name: value, AttributeType: postgres.AttributeTypeCustom}
			if _, err := db.CreateObjectSet(r.Context(), tenantID, childOA); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}

			// OA(child) -> OA(parent) edge
			attrEdge := &postgres.AssignmentEdge{
				ChildType:  postgres.NodeOA,
				ChildID:    childOA.ID,
				ParentType: postgres.NodeOA,
				ParentID:   parentOA.ID,
			}
			if _, err := db.CreateRelationship(r.Context(), tenantID, attrEdge); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}

			// OBJECT -> OA(child) edge
			objEdge := &postgres.AssignmentEdge{
				ChildType:  postgres.NodeObject,
				ChildID:    obj.ID,
				ParentType: postgres.NodeOA,
				ParentID:   childOA.ID,
			}
			if _, err := db.CreateRelationship(r.Context(), tenantID, objEdge); err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
		}
	}

	if err := s.engine.Refresh(r.Context()); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "REFRESH_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListRBACObjects lists objects with pagination using the generic object list.
// @Tags rbac
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} api.SearchResponse[rbac.Object]
// @Router /api/v1/rbac/objects [get]
func (s *Server) ListRBACObjects(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	query := r.URL.Query().Get("query")
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	// Reuse ListObjects from DB directly (no attribute filters here)
	objs, nextCursor, hasMore, err := s.engine.GetDB().ListObjects(r.Context(), tenantID, query, limit, cursor, nil)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	items := make([]rbac.Object, len(objs))
	for i, o := range objs {
		apiObj := mapper.Object(&o)
		items[i] = rbac.Object{
			ID:       apiObj.ID.String(),
			Name:     apiObj.Name,
			Kind:     apiObj.Kind,
			Metadata: apiObj.Metadata,
		}
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(items)

	resp := api.SearchResponse[rbac.Object]{
		Items:      items,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

