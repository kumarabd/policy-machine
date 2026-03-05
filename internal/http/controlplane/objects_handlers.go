package controlplane

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/internal/api/mapper"
	httputil "github.com/kumarabd/policy-machine/internal/http"
	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/api"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ListObjects returns paginated list of objects
func (s *Server) ListObjects(w http.ResponseWriter, r *http.Request) {

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

	// Get attribute filters (can be multiple)
	attributeFilters := r.URL.Query()["attribute"]

	objects, nextCursor, hasMore, err := s.engine.GetDB().ListObjects(r.Context(), tenantID, query, limit, cursor, attributeFilters)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	objs := make([]api.Object, len(objects))
	for i := range objects {
		objs[i] = mapper.Object(&objects[i])
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(objs)

	response := api.SearchResponse[api.Object]{
		Items:      objs,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObject creates a new object
func (s *Server) CreateObject(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req httputil.CreateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	obj := mapper.CreateObjectRequestToPostgres(api.CreateObjectRequest(req))
	if obj.Tags == nil {
		obj.Tags = datatypes.JSON("{}")
	}

	revision, err := s.engine.GetDB().CreateObject(r.Context(), tenantID, obj)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.ObjectResponse{
		Object:   mapper.Object(obj),
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetObject returns an object by ID
func (s *Server) GetObject(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	obj, err := s.engine.GetDB().GetObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := mapper.Object(obj)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateObject updates an object
func (s *Server) UpdateObject(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	var req httputil.UpdateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["external_id"] = req.Name
		updates["display_name"] = req.Name
	}
	if req.Kind != "" {
		updates["kind"] = req.Kind
		updates["attribute_type"] = req.Kind
	}
	if len(req.Metadata) > 0 {
		tagsJSON, _ := json.Marshal(req.Metadata)
		updates["tags"] = datatypes.JSON(tagsJSON)
	}

	if len(updates) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No fields to update")
		return
	}

	revision, err := s.engine.GetDB().UpdateObject(r.Context(), tenantID, id, updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	obj, _ := s.engine.GetDB().GetObject(r.Context(), tenantID, id)

	// Get all attributes (both native and custom) assigned to this object
	attributes, err := s.engine.GetDB().GetObjectAttributes(r.Context(), tenantID, id)
	if err != nil {
		// Log error but continue - attributes are optional
		attributes = []postgres.ObjectAttribute{}
	}

	resObj := mapper.Object(obj)
	for _, attr := range attributes {
		if resObj.Metadata == nil {
			resObj.Metadata = make(map[string]string)
		}
		resObj.Metadata["attr:"+attr.Name] = string(attr.AttributeType)
	}

	response := api.ObjectResponse{
		Object:   resObj,
		Revision: revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObject deletes an object
func (s *Server) DeleteObject(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	_, err = s.engine.GetDB().DeleteObject(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListObjectAttributesCustom returns paginated list of custom object attributes (with members).
func (s *Server) ListObjectAttributesCustom(w http.ResponseWriter, r *http.Request) {

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

	oas, nextCursor, hasMore, err := s.engine.GetDB().ListObjectSets(r.Context(), tenantID, query, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	groups := make([]api.ObjectAttribute, len(oas))
	for i := range oas {
		groups[i] = mapper.ObjectAttribute(&oas[i])
	}

	var nextCursorPtr *string
	if hasMore {
		nextCursorPtr = &nextCursor
	}
	total := len(groups)

	response := api.SearchResponse[api.ObjectAttribute]{
		Items:      groups,
		NextCursor: nextCursorPtr,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateObjectAttributeCustom creates a new custom object attribute.
func (s *Server) CreateObjectAttributeCustom(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	var req api.CreateObjectAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	oa := mapper.CreateObjectAttributeRequestToPostgres(req)
	oa.AttributeType = postgres.AttributeTypeCustom

	revision, err := s.engine.GetDB().CreateObjectSet(r.Context(), tenantID, oa)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	response := api.ObjectAttributeResponse{
		Attribute: mapper.ObjectAttribute(oa),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetObjectAttributeCustom returns a custom object attribute by ID (with members).
func (s *Server) GetObjectAttributeCustom(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid attribute ID")
		return
	}

	oa, err := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Validate that this is a custom attribute
	if oa.AttributeType != postgres.AttributeTypeCustom {
		httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "custom object attribute not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapper.ObjectAttribute(oa))
}

// UpdateObjectAttributeCustom updates a custom object attribute.
func (s *Server) UpdateObjectAttributeCustom(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid attribute ID")
		return
	}

	var req api.UpdateObjectAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}

	revision, err := s.engine.GetDB().UpdateObjectSet(r.Context(), tenantID, id, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	oa, _ := s.engine.GetDB().GetObjectSet(r.Context(), tenantID, id)

	response := api.ObjectAttributeResponse{
		Attribute: mapper.ObjectAttribute(oa),
		Revision:  revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteObjectAttributeCustom deletes an object attribute (should be custom attribute for object-sets API).
func (s *Server) DeleteObjectAttributeCustom(w http.ResponseWriter, r *http.Request) {

	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object attribute ID")
		return
	}

	_, err = s.engine.GetDB().DeleteObjectSet(r.Context(), tenantID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.RespondError(w, http.StatusNotFound, "NOT_FOUND", "object attribute not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetObjectAttributes returns the complete attribute tree for an object
// @Summary Get object attribute tree
// @Description Returns all attributes (native and custom) assigned to an object and their hierarchical relationships, forming a tree where the object is the root node
// @Tags generic
// @Produce json
// @Param id path string true "Object ID"
// @Success 200 {object} api.AttributeSubgraphResponse
// @Router /api/v1/objects/{id}/attributes [get]
func (s *Server) GetObjectAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := httputil.GetTenantID(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusBadRequest, "MISSING_TENANT", "Tenant ID required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	attributes, edges, err := s.engine.GetDB().GetObjectAttributeSubgraph(r.Context(), tenantID, id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Convert to API response format
	nodes := make([]api.AttributeNode, len(attributes))
	for i, attr := range attributes {
		nodes[i] = api.AttributeNode{
			ID:            attr.ID,
			Name:          attr.Name,
			AttributeType: string(attr.AttributeType),
		}
	}

	// Convert edges (include object->OA edges and OA->OA edges)
	apiEdges := make([]api.AttributeEdge, 0, len(edges))
	for _, edge := range edges {
		// Include object->OA edges and OA->OA edges
		if (edge.ChildType == postgres.NodeObject && edge.ParentType == postgres.NodeOA) ||
			(edge.ChildType == postgres.NodeOA && edge.ParentType == postgres.NodeOA) {
			apiEdges = append(apiEdges, api.AttributeEdge{
				ChildID:  edge.ChildID,
				ParentID: edge.ParentID,
			})
		}
	}

	response := api.AttributeSubgraphResponse{
		Nodes: nodes,
		Edges: apiEdges,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
