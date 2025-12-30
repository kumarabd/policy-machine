package mock

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kumarabd/policy-machine/pkg/api"
)

// ListScopes returns mock scopes
func ListScopes(w http.ResponseWriter, r *http.Request) {
	// For now, return empty scopes list
	// In a real implementation, scopes would be derived from subject-sets and object-sets
	// or stored as separate entities
	scopes := []api.PolicyScope{}

	response := api.SearchResponse[api.PolicyScope]{
		Items: scopes,
		Total: func() *int { v := 0; return &v }(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper function to check if a string matches query (case-insensitive)
func matchesQuery(str, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(str), strings.ToLower(query))
}

// GetMeta returns mock metadata
func GetMeta(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	response := api.MetaResponse{
		ServiceVersion: "dev",
		TenantID:       data.TenantID,
		Revision:       data.Revision,
		AppliedSeq:     data.Revision,
		Now:            time.Now(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCurrentRevision returns mock revision
func GetCurrentRevision(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	response := api.RevisionResponse{
		TenantID: data.TenantID,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPolicyChanges returns mock policy changes
func GetPolicyChanges(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	afterSeq := int64(0)
	if seqStr := r.URL.Query().Get("after_seq"); seqStr != "" {
		if seq, err := strconv.ParseInt(seqStr, 10, 64); err == nil {
			afterSeq = seq
		}
	}
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var filtered []api.PolicyChangeItem
	toSeq := afterSeq
	for _, ch := range data.PolicyChanges {
		if ch.Seq > afterSeq {
			filtered = append(filtered, ch)
			if ch.Seq > toSeq {
				toSeq = ch.Seq
			}
			if len(filtered) >= limit {
				break
			}
		}
	}

	response := api.ChangesResponse{
		TenantID: data.TenantID,
		FromSeq:  afterSeq,
		ToSeq:    toSeq,
		Changes:  filtered,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListVersions returns mock versions
func ListVersions(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	query := r.URL.Query().Get("q")
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	cursor := r.URL.Query().Get("cursor")

	// Create versions from policy changes (group by revision)
	revisionMap := make(map[int64]*api.Version)
	for _, ch := range data.PolicyChanges {
		rev := ch.Revision
		if _, exists := revisionMap[rev]; !exists {
			revisionMap[rev] = &api.Version{
				ID:          strconv.FormatInt(rev, 10),
				CreatedAt:   ch.CreatedAt,
				Author:      "system",
				Message:     "Policy update",
				ChangeCount: 0,
			}
			if rev > 1 {
				parentID := strconv.FormatInt(rev-1, 10)
				revisionMap[rev].ParentVersionID = &parentID
			}
		}
		revisionMap[rev].ChangeCount++
	}

	// Convert to slice and sort by revision (descending)
	versions := make([]api.Version, 0, len(revisionMap))
	for _, v := range revisionMap {
		versions = append(versions, *v)
	}

	// Simple sort by revision descending
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			revI, _ := strconv.ParseInt(versions[i].ID, 10, 64)
			revJ, _ := strconv.ParseInt(versions[j].ID, 10, 64)
			if revI < revJ {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	// Apply search filter
	if query != "" {
		filtered := make([]api.Version, 0)
		queryLower := strings.ToLower(query)
		for _, v := range versions {
			if strings.Contains(strings.ToLower(v.ID), queryLower) ||
				strings.Contains(strings.ToLower(v.Author), queryLower) ||
				strings.Contains(strings.ToLower(v.Message), queryLower) {
				filtered = append(filtered, v)
			}
		}
		versions = filtered
	}

	total := len(versions)

	// Apply pagination
	startIdx := 0
	if cursor != "" {
		if idx, err := strconv.Atoi(cursor); err == nil && idx > 0 && idx < len(versions) {
			startIdx = idx
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(versions) {
		endIdx = len(versions)
	}

	items := versions[startIdx:endIdx]
	var nextCursor *string
	if endIdx < len(versions) {
		cursor := strconv.Itoa(endIdx)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.Version]{
		Items:      items,
		NextCursor: nextCursor,
		Total:      &total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListSubjects returns mock subjects
func ListSubjects(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()

	// Ensure we have data
	if data == nil || len(data.Subjects) == 0 {
		response := api.SearchResponse[api.Subject]{
			Items:      []api.Subject{},
			NextCursor: nil,
			Total:      func() *int { v := 0; return &v }(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get query parameter and clean it up (handle malformed queries like "limit=10")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	// Ignore queries that look like parameter assignments (e.g., "limit=10")
	if strings.Contains(query, "=") {
		query = ""
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Always return all subjects if no query, or filter if query provided
	var filtered []api.Subject
	for _, s := range data.Subjects {
		if query == "" || matchesQuery(s.ExternalID, query) || matchesQuery(s.Email, query) || matchesQuery(s.Display, query) || matchesQuery(s.DisplayName, query) {
			filtered = append(filtered, s)
		}
	}

	// Apply limit after filtering
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	total := len(data.Subjects)
	if query != "" {
		// Count all matching items for total
		matchingCount := 0
		for _, s := range data.Subjects {
			if matchesQuery(s.ExternalID, query) || matchesQuery(s.Email, query) || matchesQuery(s.Display, query) || matchesQuery(s.DisplayName, query) {
				matchingCount++
			}
		}
		total = matchingCount
	}

	var nextCursor *string
	if len(filtered) == limit && total > limit {
		cursor := strconv.Itoa(limit)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.Subject]{
		Items:      filtered,
		NextCursor: nextCursor,
		Total:      &total,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSubject returns a mock subject by ID
func GetSubject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	for _, s := range data.Subjects {
		if s.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
}

// CreateSubject creates a mock subject (adds to mock data)
func CreateSubject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	displayName := req.Display
	if displayName == "" {
		displayName = req.ExternalID
	}
	newSubject := api.Subject{
		ID:          uuid.New(),
		ExternalID:  req.ExternalID,
		Email:       req.Email,
		Display:     req.Display,
		DisplayName: displayName,
		Kind:        "user",
		Attributes:  make(map[string]string),
		Tags:        []string{},
		CreatedAt:   time.Now(),
	}
	data.Subjects = append(data.Subjects, newSubject)
	data.Revision++

	response := api.SubjectResponse{
		Subject:  newSubject,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateSubject updates a mock subject
func UpdateSubject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	var req api.UpdateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, s := range data.Subjects {
		if s.ID == id {
			if req.Email != "" {
				data.Subjects[i].Email = req.Email
			}
			if req.Display != "" {
				data.Subjects[i].Display = req.Display
				data.Subjects[i].DisplayName = req.Display
			}
			// Ensure displayName is set
			if data.Subjects[i].DisplayName == "" {
				data.Subjects[i].DisplayName = data.Subjects[i].ExternalID
			}
			data.Revision++

			response := api.SubjectResponse{
				Subject:  data.Subjects[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
}

// DeleteSubject deletes a mock subject
func DeleteSubject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject ID")
		return
	}

	for i, s := range data.Subjects {
		if s.ID == id {
			data.Subjects = append(data.Subjects[:i], data.Subjects[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject not found")
}

// ListSubjectGroups returns mock subject groups
func ListSubjectGroups(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()

	// Get query parameter and clean it up
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if strings.Contains(query, "=") {
		query = ""
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var filtered []api.SubjectSet
	for _, sg := range data.SubjectGroups {
		if query == "" || matchesQuery(sg.Name, query) {
			filtered = append(filtered, sg)
		}
	}

	// Apply limit after filtering
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	total := len(data.SubjectGroups)
	if query != "" {
		matchingCount := 0
		for _, sg := range data.SubjectGroups {
			if matchesQuery(sg.Name, query) {
				matchingCount++
			}
		}
		total = matchingCount
	}

	var nextCursor *string
	if len(filtered) == limit && total > limit {
		cursor := strconv.Itoa(limit)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.SubjectGroup]{
		Items:      filtered,
		NextCursor: nextCursor,
		Total:      &total,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSubjectGroup returns a mock subject group by ID
func GetSubjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	for _, sg := range data.SubjectGroups {
		if sg.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sg)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
}

// CreateSubjectGroup creates a mock subject group
func CreateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateSubjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	newGroup := api.SubjectSet{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}
	data.SubjectGroups = append(data.SubjectGroups, newGroup)
	data.Revision++

	response := api.SubjectGroupResponse{
		Group:    newGroup,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateSubjectGroup updates a mock subject group
func UpdateSubjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	var req api.UpdateSubjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, sg := range data.SubjectGroups {
		if sg.ID == id {
			data.SubjectGroups[i].Name = req.Name
			data.Revision++

			response := api.SubjectGroupResponse{
				Group:    data.SubjectGroups[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
}

// DeleteSubjectGroup deletes a mock subject group
func DeleteSubjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	for i, sg := range data.SubjectGroups {
		if sg.ID == id {
			data.SubjectGroups = append(data.SubjectGroups[:i], data.SubjectGroups[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject group not found")
}

// Similar functions for Objects, ObjectGroups, Relationships, Rules, Denies, etc.
// For brevity, I'll create a few key ones and you can extend them

// ListObjects returns mock objects
func ListObjects(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()

	// Ensure we have data
	if data == nil || len(data.Objects) == 0 {
		response := api.SearchResponse[api.Object]{
			Items:      []api.Object{},
			NextCursor: nil,
			Total:      func() *int { v := 0; return &v }(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get query parameter and clean it up (handle malformed queries like "limit=10")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	// Ignore queries that look like parameter assignments (e.g., "limit=10")
	if strings.Contains(query, "=") {
		query = ""
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var filtered []api.Object
	for _, obj := range data.Objects {
		// If query is empty, include all items. Otherwise, filter by query.
		if query == "" || matchesQuery(obj.ExternalID, query) || matchesQuery(obj.Type, query) || matchesQuery(obj.DisplayName, query) {
			filtered = append(filtered, obj)
		}
	}

	// Apply limit after filtering
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	total := len(data.Objects) // Total count of all objects
	if query != "" {
		// Count all matching items for total
		matchingCount := 0
		for _, obj := range data.Objects {
			if matchesQuery(obj.ExternalID, query) || matchesQuery(obj.Type, query) || matchesQuery(obj.DisplayName, query) {
				matchingCount++
			}
		}
		total = matchingCount
	}

	var nextCursor *string
	if len(filtered) == limit && total > limit {
		cursor := strconv.Itoa(limit)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.Object]{
		Items:      filtered,
		NextCursor: nextCursor,
		Total:      &total,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetObject returns a mock object by ID
func GetObject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	for _, obj := range data.Objects {
		if obj.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(obj)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
}

// CreateObject creates a mock object
func CreateObject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	displayName := req.ExternalID
	if req.Type != "" {
		displayName = req.Type + ": " + req.ExternalID
	}
	newObject := api.Object{
		ID:          uuid.New(),
		ExternalID:  req.ExternalID,
		Type:        req.Type,
		DisplayName: displayName,
		Kind:        req.Type,
		Attributes:  make(map[string]string),
		Tags:        []string{},
		CreatedAt:   time.Now(),
	}
	data.Objects = append(data.Objects, newObject)
	data.Revision++

	response := api.ObjectResponse{
		Object:   newObject,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ListObjectGroups returns mock object groups
func ListObjectGroups(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()

	// Get query parameter and clean it up
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if strings.Contains(query, "=") {
		query = ""
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var filtered []api.ObjectSet
	for _, og := range data.ObjectGroups {
		if query == "" || matchesQuery(og.Name, query) {
			filtered = append(filtered, og)
		}
	}

	// Apply limit after filtering
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	total := len(data.ObjectGroups)
	if query != "" {
		matchingCount := 0
		for _, og := range data.ObjectGroups {
			if matchesQuery(og.Name, query) {
				matchingCount++
			}
		}
		total = matchingCount
	}

	var nextCursor *string
	if len(filtered) == limit && total > limit {
		cursor := strconv.Itoa(limit)
		nextCursor = &cursor
	}

	response := api.SearchResponse[api.ObjectSet]{
		Items:      filtered,
		NextCursor: nextCursor,
		Total:      &total,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetObjectGroup returns a mock object group by ID
func GetObjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	for _, og := range data.ObjectGroups {
		if og.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(og)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
}

// ListRelationships returns mock relationships
func ListRelationships(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	filtered := data.Relationships
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	response := api.ListRelationshipsResponse{
		Relationships: filtered,
		HasMore:       false,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListRules returns mock rules
func ListRules(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	filtered := data.Rules
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	total := len(filtered)
	response := api.SearchResponse[api.Rule]{
		Items:      filtered,
		NextCursor: nil,
		Total:      &total,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetRule returns a mock rule by ID
func GetRule(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	for _, rule := range data.Rules {
		if rule.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rule)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
}

// ListDenies returns mock denies
func ListDenies(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	filtered := data.Denies
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	response := api.ListDeniesResponse{
		Denies:  filtered,
		HasMore: false,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDeny returns a mock deny by ID
func GetDeny(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	for _, deny := range data.Denies {
		if deny.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(deny)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
}

// GetGraphSummary returns mock graph summary
func GetGraphSummary(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	response := api.GraphSummaryResponse{
		Subjects:      len(data.Subjects),
		Objects:       len(data.Objects),
		SubjectSets:   len(data.SubjectGroups),
		ObjectSets:    len(data.ObjectGroups),
		Relationships: len(data.Relationships),
		Rules:         len(data.Rules),
		Denies:        len(data.Denies),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GraphSearch returns mock search results
func GraphSearch(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	query := r.URL.Query().Get("q")
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	var nodes []api.GraphNode
	for _, s := range data.Subjects {
		if matchesQuery(s.ExternalID, query) || matchesQuery(s.Display, query) {
			nodes = append(nodes, api.GraphNode{
				ID:   s.ID,
				Type: "subject",
				Name: s.ExternalID,
			})
			if len(nodes) >= limit {
				break
			}
		}
	}

	for _, sg := range data.SubjectGroups {
		if matchesQuery(sg.Name, query) {
			nodes = append(nodes, api.GraphNode{
				ID:   sg.ID,
				Type: "subject-set",
				Name: sg.Name,
			})
			if len(nodes) >= limit {
				break
			}
		}
	}

	response := api.GraphSearchResponse{
		Nodes: nodes,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Authorize returns mock authorization decision
func Authorize(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Simple mock logic: allow if user exists in mock data
	allowed := false
	for _, s := range data.Subjects {
		if s.ID == req.UserID {
			allowed = true
			break
		}
	}

	response := api.AuthorizeResponse{
		Allowed:  allowed,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Evaluate returns mock evaluation decision (UI-compatible format)
func Evaluate(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Parse UUIDs
	subjectID, err := uuid.Parse(req.Subject.ID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_SUBJECT_ID", "Invalid subject ID format")
		return
	}

	// Simple mock logic: allow if subject exists in mock data
	allowed := false
	for _, s := range data.Subjects {
		if s.ID == subjectID {
			allowed = true
			break
		}
	}

	decision := "DENY"
	if allowed {
		decision = "ALLOW"
	}

	versionID := strconv.FormatInt(data.Revision, 10)

	response := api.EvaluateResponse{
		Decision:    decision,
		VersionID:   versionID,
		EvaluatedAt: time.Now(),
	}

	// Add trace if requested
	if req.Explain {
		trace := &api.ExplainTrace{}
		trace.Summary.MatchedRulesCount = 0
		trace.Summary.DenyRulesCount = 0
		trace.Summary.EffectiveDecision = decision
		trace.Summary.VersionID = versionID
		trace.MatchedRules = []api.MatchedRule{}
		trace.Denies = []api.DenyRule{}
		response.Trace = trace
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportPolicy returns mock policy bundle
func ExportPolicy(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	bundle := api.PolicyBundle{
		Revision:      data.Revision,
		Subjects:      data.Subjects,
		SubjectSets:   data.SubjectGroups,
		Objects:       data.Objects,
		ObjectSets:    data.ObjectGroups,
		Relationships: data.Relationships,
		Rules:         data.Rules,
		Denies:        data.Denies,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

// UpdateObject updates a mock object
func UpdateObject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	var req api.UpdateObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, obj := range data.Objects {
		if obj.ID == id {
			if req.Type != "" {
				data.Objects[i].Type = req.Type
				data.Objects[i].Kind = req.Type
				// Update displayName based on type
				if req.Type != "" {
					data.Objects[i].DisplayName = req.Type + ": " + data.Objects[i].ExternalID
				} else {
					data.Objects[i].DisplayName = data.Objects[i].ExternalID
				}
			}
			// Ensure displayName is set
			if data.Objects[i].DisplayName == "" {
				if data.Objects[i].Type != "" {
					data.Objects[i].DisplayName = data.Objects[i].Type + ": " + data.Objects[i].ExternalID
				} else {
					data.Objects[i].DisplayName = data.Objects[i].ExternalID
				}
			}
			data.Revision++

			response := api.ObjectResponse{
				Object:   data.Objects[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
}

// DeleteObject deletes a mock object
func DeleteObject(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	for i, obj := range data.Objects {
		if obj.ID == id {
			data.Objects = append(data.Objects[:i], data.Objects[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object not found")
}

// CreateObjectGroup creates a mock object group
func CreateObjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateObjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	newGroup := api.ObjectSet{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}
	data.ObjectGroups = append(data.ObjectGroups, newGroup)
	data.Revision++

	response := api.ObjectGroupResponse{
		Group:    newGroup,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateObjectGroup updates a mock object group
func UpdateObjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	var req api.UpdateObjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, og := range data.ObjectGroups {
		if og.ID == id {
			data.ObjectGroups[i].Name = req.Name
			data.Revision++

			response := api.ObjectGroupResponse{
				Group:    data.ObjectGroups[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
}

// DeleteObjectGroup deletes a mock object group
func DeleteObjectGroup(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid group ID")
		return
	}

	for i, og := range data.ObjectGroups {
		if og.ID == id {
			data.ObjectGroups = append(data.ObjectGroups[:i], data.ObjectGroups[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object group not found")
}

// CreateRelationship creates a mock relationship
func CreateRelationship(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	newRel := api.Relationship{
		ID:   uuid.New(),
		Kind: req.Kind,
		From: req.From,
		To:   req.To,
	}
	data.Relationships = append(data.Relationships, newRel)
	data.Revision++

	response := api.RelationshipResponse{
		Relationship: newRel,
		Revision:     data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// DeleteRelationship deletes a mock relationship
func DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.DeleteRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, rel := range data.Relationships {
		if rel.Kind == req.Kind && rel.From.ID == req.From.ID && rel.To.ID == req.To.ID {
			data.Relationships = append(data.Relationships[:i], data.Relationships[i+1:]...)
			data.Revision++

			response := api.RelationshipResponse{
				Relationship: rel,
				Revision:     data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Relationship not found")
}

// CreateRule creates a mock rule
func CreateRule(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	now := time.Now()
	newRule := api.Rule{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		ScopeID:         req.ScopeID,
		Actions:         req.Actions,
		SubjectSelector: req.SubjectSelector,
		ObjectSelector:  req.ObjectSelector,
		Condition:       req.Condition,
		Effect:          req.Effect,
		Priority:        req.Priority,
		Enabled:         req.Enabled,
		CreatedAt:       now,
	}
	data.Rules = append(data.Rules, newRule)
	data.Revision++

	response := api.RuleResponse{
		Rule:     newRule,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateRule updates a mock rule
func UpdateRule(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	var req api.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, rule := range data.Rules {
		if rule.ID == id {
			if req.Name != nil {
				data.Rules[i].Name = *req.Name
			}
			if req.Description != nil {
				data.Rules[i].Description = *req.Description
			}
			if len(req.Actions) > 0 {
				data.Rules[i].Actions = req.Actions
			}
			if req.Condition != nil {
				data.Rules[i].Condition = req.Condition
			}
			if req.Effect != nil {
				data.Rules[i].Effect = *req.Effect
			}
			if req.Priority != nil {
				data.Rules[i].Priority = req.Priority
			}
			if req.Enabled != nil {
				data.Rules[i].Enabled = *req.Enabled
			}
			now := time.Now()
			data.Rules[i].UpdatedAt = &now
			data.Revision++

			response := api.RuleResponse{
				Rule:     data.Rules[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
}

// DeleteRule deletes a mock rule
func DeleteRule(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid rule ID")
		return
	}

	for i, rule := range data.Rules {
		if rule.ID == id {
			data.Rules = append(data.Rules[:i], data.Rules[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Rule not found")
}

// CreateDeny creates a mock deny rule
func CreateDeny(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.CreateDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	newDeny := api.Deny{
		ID:          uuid.New(),
		Subject:     req.Subject,
		Operations:  req.Operations,
		Targets:     req.Targets,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}
	data.Denies = append(data.Denies, newDeny)
	data.Revision++

	response := api.DenyResponse{
		Deny:     newDeny,
		Revision: data.Revision,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateDeny updates a mock deny rule
func UpdateDeny(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	var req api.UpdateDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	for i, deny := range data.Denies {
		if deny.ID == id {
			if len(req.Operations) > 0 {
				data.Denies[i].Operations = req.Operations
			}
			if len(req.Targets) > 0 {
				data.Denies[i].Targets = req.Targets
			}
			if req.Description != "" {
				data.Denies[i].Description = req.Description
			}
			data.Revision++

			response := api.DenyResponse{
				Deny:     data.Denies[i],
				Revision: data.Revision,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
}

// DeleteDeny deletes a mock deny rule
func DeleteDeny(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid deny ID")
		return
	}

	for i, deny := range data.Denies {
		if deny.ID == id {
			data.Denies = append(data.Denies[:i], data.Denies[i+1:]...)
			data.Revision++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Deny rule not found")
}

// AuthorizeExplain returns mock explain response
func AuthorizeExplain(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Simple mock: return empty closures
	response := api.ExplainResponse{
		Allowed:  true,
		Revision: data.Revision,
	}
	response.Explain.SubjectClosure = []uuid.UUID{req.UserID}
	response.Explain.ObjectClosure = []uuid.UUID{req.ObjectID}
	response.Explain.AllowHits = []uuid.UUID{}
	response.Explain.DenyHits = []uuid.UUID{}
	response.Explain.EffectiveAllowSetSize = 1
	response.Explain.EffectiveDenySetSize = 0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGraphNeighborhood returns mock neighborhood
// Supports both GET (query params) and POST (request body) for UI compatibility
func GetGraphNeighborhood(w http.ResponseWriter, r *http.Request) {
	var nodeType, nodeIDStr string
	var depth int = 1

	// Handle POST request with JSON body (UI format)
	if r.Method == "POST" {
		var reqBody struct {
			Seed struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"seed"`
			Depth int `json:"depth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			nodeType = reqBody.Seed.Type
			nodeIDStr = reqBody.Seed.ID
			if reqBody.Depth > 0 {
				depth = reqBody.Depth
			}
		}
	}

	// Fallback to GET query parameters if POST body parsing failed or it's a GET request
	if nodeIDStr == "" {
		nodeIDStr = r.URL.Query().Get("node_id")
		nodeType = r.URL.Query().Get("node_type")
		if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
			if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
				depth = d
			}
		}
	}

	// depth is reserved for future implementation of neighborhood depth traversal
	_ = depth

	if nodeIDStr == "" {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "node_id is required")
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid node ID")
		return
	}

	// Return the node itself and empty edges
	// TODO: Build actual neighborhood graph from mock data
	response := api.GraphNeighborhoodResponse{
		Nodes: []api.GraphNode{
			{
				ID:   nodeID,
				Type: nodeType,
				Name: "Node " + nodeID.String()[:8], // Placeholder name
			},
		},
		Edges: []api.GraphEdge{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImportPolicy imports a mock policy bundle
func ImportPolicy(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	var req api.ImportPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	applied := 0

	// Add subjects
	data.Subjects = append(data.Subjects, req.Bundle.Subjects...)
	applied += len(req.Bundle.Subjects)

	// Add subject sets
	data.SubjectGroups = append(data.SubjectGroups, req.Bundle.SubjectSets...)
	applied += len(req.Bundle.SubjectSets)

	// Add objects
	data.Objects = append(data.Objects, req.Bundle.Objects...)
	applied += len(req.Bundle.Objects)

	// Add object sets
	data.ObjectGroups = append(data.ObjectGroups, req.Bundle.ObjectSets...)
	applied += len(req.Bundle.ObjectSets)

	// Add relationships
	data.Relationships = append(data.Relationships, req.Bundle.Relationships...)
	applied += len(req.Bundle.Relationships)

	// Add rules
	data.Rules = append(data.Rules, req.Bundle.Rules...)
	applied += len(req.Bundle.Rules)

	// Add denies
	data.Denies = append(data.Denies, req.Bundle.Denies...)
	applied += len(req.Bundle.Denies)

	data.Revision++

	response := api.ImportPolicyResponse{
		Revision: data.Revision,
		Applied:  applied,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AddSubjectSetMembers adds members to a subject set (mock)
func AddSubjectSetMembers(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject set ID")
		return
	}

	var req struct {
		SubjectIDs []string `json:"subjectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Find the subject set
	for i, sg := range data.SubjectGroups {
		if sg.ID == setID {
			// Add new member IDs
			for _, idStr := range req.SubjectIDs {
				if id, err := uuid.Parse(idStr); err == nil {
					data.SubjectGroups[i].MemberSubjectIDs = append(data.SubjectGroups[i].MemberSubjectIDs, id)
				}
			}
			data.Revision++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data.SubjectGroups[i])
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject set not found")
}

// RemoveSubjectSetMember removes a member from a subject set (mock)
func RemoveSubjectSetMember(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid subject set ID")
		return
	}

	var req struct {
		SubjectIDs []string `json:"subjectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Find the subject set
	for i, sg := range data.SubjectGroups {
		if sg.ID == setID {
			// Remove member IDs
			for _, idStr := range req.SubjectIDs {
				if id, err := uuid.Parse(idStr); err == nil {
					for j, memberID := range data.SubjectGroups[i].MemberSubjectIDs {
						if memberID == id {
							data.SubjectGroups[i].MemberSubjectIDs = append(
								data.SubjectGroups[i].MemberSubjectIDs[:j],
								data.SubjectGroups[i].MemberSubjectIDs[j+1:]...)
							break
						}
					}
				}
			}
			data.Revision++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data.SubjectGroups[i])
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Subject set not found")
}

// AddObjectSetMembers adds members to an object set (mock)
func AddObjectSetMembers(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object set ID")
		return
	}

	var req struct {
		ObjectIDs []string `json:"objectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Find the object set
	for i, og := range data.ObjectGroups {
		if og.ID == setID {
			// Add new member IDs
			for _, idStr := range req.ObjectIDs {
				if id, err := uuid.Parse(idStr); err == nil {
					data.ObjectGroups[i].MemberObjectIDs = append(data.ObjectGroups[i].MemberObjectIDs, id)
				}
			}
			data.Revision++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data.ObjectGroups[i])
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object set not found")
}

// RemoveObjectSetMember removes a member from an object set (mock)
func RemoveObjectSetMember(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")
	setID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid object set ID")
		return
	}

	var req struct {
		ObjectIDs []string `json:"objectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Find the object set
	for i, og := range data.ObjectGroups {
		if og.ID == setID {
			// Remove member IDs
			for _, idStr := range req.ObjectIDs {
				if id, err := uuid.Parse(idStr); err == nil {
					for j, memberID := range data.ObjectGroups[i].MemberObjectIDs {
						if memberID == id {
							data.ObjectGroups[i].MemberObjectIDs = append(
								data.ObjectGroups[i].MemberObjectIDs[:j],
								data.ObjectGroups[i].MemberObjectIDs[j+1:]...)
							break
						}
					}
				}
			}
			data.Revision++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data.ObjectGroups[i])
			return
		}
	}

	respondError(w, http.StatusNotFound, "NOT_FOUND", "Object set not found")
}

// GetVersionDiff returns version diff (mock)
func GetVersionDiff(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")

	// Find changes for this version
	var versionChanges []api.PolicyChangeItem
	for _, ch := range data.PolicyChanges {
		if strconv.FormatInt(ch.Revision, 10) == idStr {
			versionChanges = append(versionChanges, ch)
		}
	}

	response := map[string]interface{}{
		"versionId": idStr,
		"changes":   versionChanges,
		"added":     []interface{}{},
		"removed":   []interface{}{},
		"modified":  []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetVersionSnapshot returns version snapshot (mock)
func GetVersionSnapshot(w http.ResponseWriter, r *http.Request) {
	data := GetMockData()
	idStr := chi.URLParam(r, "id")

	response := map[string]interface{}{
		"versionId": idStr,
		"subjects":  data.Subjects,
		"objects":   data.Objects,
		"rules":     data.Rules,
		"denies":    data.Denies,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper function for error responses
func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
