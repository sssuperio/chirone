package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type projectSnapshot struct {
	Glyphs   json.RawMessage `json:"glyphs"`
	Syntaxes json.RawMessage `json:"syntaxes"`
	Metrics  json.RawMessage `json:"metrics"`
}

type projectDocument struct {
	Project   string `json:"project"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updatedAt"`
	projectSnapshot
}

type updateProjectRequest struct {
	ClientID    string `json:"clientId"`
	BaseVersion *int64 `json:"baseVersion,omitempty"`
	projectSnapshot
}

type updateGlyphRequest struct {
	ClientID    string          `json:"clientId"`
	BaseVersion *int64          `json:"baseVersion,omitempty"`
	Glyph       json.RawMessage `json:"glyph"`
}

type deleteGlyphRequest struct {
	ClientID    string `json:"clientId"`
	BaseVersion *int64 `json:"baseVersion,omitempty"`
	ID          string `json:"id"`
}

type updateSyntaxRequest struct {
	ClientID    string          `json:"clientId"`
	BaseVersion *int64          `json:"baseVersion,omitempty"`
	Syntax      json.RawMessage `json:"syntax"`
}

type deleteSyntaxRequest struct {
	ClientID    string `json:"clientId"`
	BaseVersion *int64 `json:"baseVersion,omitempty"`
	ID          string `json:"id"`
}

type updateMetricsRequest struct {
	ClientID    string          `json:"clientId"`
	BaseVersion *int64          `json:"baseVersion,omitempty"`
	Metrics     json.RawMessage `json:"metrics"`
}

type versionConflictError struct {
	ExpectedVersion int64
	Current         projectDocument
}

type entityConflictError struct {
	ExpectedVersion int64
	CurrentVersion  int64
	ProjectVersion  int64
	Entity          string
	EntityID        string
	EntityDeleted   bool
	UpdatedAt       string
	Payload         json.RawMessage
}

func (e *entityConflictError) Error() string {
	return fmt.Sprintf(
		"entity version conflict: entity=%s id=%s baseVersion=%d currentVersion=%d",
		e.Entity,
		e.EntityID,
		e.ExpectedVersion,
		e.CurrentVersion,
	)
}

type entityUpdateResponse struct {
	Project        string          `json:"project"`
	Entity         string          `json:"entity"`
	EntityID       string          `json:"entityId,omitempty"`
	Version        int64           `json:"version"`
	ProjectVersion int64           `json:"projectVersion,omitempty"`
	Deleted        bool            `json:"deleted,omitempty"`
	UpdatedAt      string          `json:"updatedAt"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type projectResponse struct {
	projectDocument
	GlyphVersions  map[string]int64 `json:"glyphVersions,omitempty"`
	SyntaxVersions map[string]int64 `json:"syntaxVersions,omitempty"`
	MetricsVersion int64            `json:"metricsVersion,omitempty"`
}

func (e *versionConflictError) Error() string {
	return fmt.Sprintf(
		"version conflict: baseVersion=%d currentVersion=%d",
		e.ExpectedVersion,
		e.Current.Version,
	)
}

type projectEvent struct {
	Type          string          `json:"type"`
	ClientID      string          `json:"clientId,omitempty"`
	Entity        string          `json:"entity,omitempty"`
	EntityID      string          `json:"entityId,omitempty"`
	EntityVersion int64           `json:"entityVersion,omitempty"`
	EntityDeleted bool            `json:"entityDeleted,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	projectDocument
}

type projectState struct {
	Doc            projectDocument
	Glyphs         map[string]json.RawMessage
	Syntaxes       map[string]json.RawMessage
	Metrics        json.RawMessage
	GlyphVersions  map[string]int64
	SyntaxVersions map[string]int64
	MetricsVersion int64
	Subs           map[chan projectEvent]struct{}
}

type hub struct {
	mu       sync.RWMutex
	projects map[string]*projectState
	dataDir  string
}

var (
	projectIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func newHub(dataDir string) *hub {
	return &hub{
		projects: map[string]*projectState{},
		dataDir:  dataDir,
	}
}

func normalizeSnapshot(snapshot projectSnapshot) (projectSnapshot, error) {
	var out projectSnapshot

	if len(snapshot.Glyphs) == 0 {
		out.Glyphs = json.RawMessage(`[]`)
	} else {
		if !json.Valid(snapshot.Glyphs) {
			return out, errors.New("glyphs is not valid JSON")
		}
		out.Glyphs = snapshot.Glyphs
	}

	if len(snapshot.Syntaxes) == 0 {
		out.Syntaxes = json.RawMessage(`[]`)
	} else {
		if !json.Valid(snapshot.Syntaxes) {
			return out, errors.New("syntaxes is not valid JSON")
		}
		out.Syntaxes = snapshot.Syntaxes
	}

	if len(snapshot.Metrics) == 0 {
		out.Metrics = json.RawMessage(`{}`)
	} else {
		if !json.Valid(snapshot.Metrics) {
			return out, errors.New("metrics is not valid JSON")
		}
		out.Metrics = snapshot.Metrics
	}

	return out, nil
}

type entityID struct {
	ID string `json:"id"`
}

func normalizedRawObject(raw json.RawMessage, errPrefix string) (json.RawMessage, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, fmt.Errorf("%s is not valid JSON", errPrefix)
	}
	var tmp map[string]any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object", errPrefix)
	}
	bytes, err := json.Marshal(tmp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}

func parseEntityArrayByID(raw json.RawMessage, entityName string) (map[string]json.RawMessage, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%s must be an array", entityName)
	}

	out := make(map[string]json.RawMessage, len(list))
	for _, item := range list {
		normalized, err := normalizedRawObject(item, entityName+" item")
		if err != nil {
			return nil, err
		}
		var id entityID
		if err := json.Unmarshal(normalized, &id); err != nil {
			return nil, fmt.Errorf("%s item has invalid id", entityName)
		}
		id.ID = strings.TrimSpace(id.ID)
		if id.ID == "" {
			return nil, fmt.Errorf("%s item missing id", entityName)
		}
		out[id.ID] = normalized
	}
	return out, nil
}

func parseEntityItem(raw json.RawMessage, entityName string) (string, json.RawMessage, error) {
	normalized, err := normalizedRawObject(raw, entityName)
	if err != nil {
		return "", nil, err
	}
	var id entityID
	if err := json.Unmarshal(normalized, &id); err != nil {
		return "", nil, fmt.Errorf("%s has invalid id", entityName)
	}
	id.ID = strings.TrimSpace(id.ID)
	if id.ID == "" {
		return "", nil, fmt.Errorf("%s missing id", entityName)
	}
	return id.ID, normalized, nil
}

func serializeEntityMap(items map[string]json.RawMessage) (json.RawMessage, error) {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	list := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		list = append(list, items[id])
	}

	bytes, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}

func rebuildProjectSnapshot(state *projectState) error {
	glyphs, err := serializeEntityMap(state.Glyphs)
	if err != nil {
		return err
	}
	syntaxes, err := serializeEntityMap(state.Syntaxes)
	if err != nil {
		return err
	}
	metrics := state.Metrics
	if len(metrics) == 0 {
		metrics = json.RawMessage(`{}`)
	}

	state.Doc.projectSnapshot = projectSnapshot{
		Glyphs:   glyphs,
		Syntaxes: syntaxes,
		Metrics:  metrics,
	}
	return nil
}

func newProjectStateFromDocument(doc projectDocument) (*projectState, error) {
	snapshot, err := normalizeSnapshot(doc.projectSnapshot)
	if err != nil {
		return nil, err
	}
	doc.projectSnapshot = snapshot

	glyphMap, err := parseEntityArrayByID(snapshot.Glyphs, "glyphs")
	if err != nil {
		return nil, err
	}
	syntaxMap, err := parseEntityArrayByID(snapshot.Syntaxes, "syntaxes")
	if err != nil {
		return nil, err
	}

	metrics, err := normalizedRawObject(snapshot.Metrics, "metrics")
	if err != nil {
		return nil, err
	}

	state := &projectState{
		Doc:            doc,
		Glyphs:         glyphMap,
		Syntaxes:       syntaxMap,
		Metrics:        metrics,
		GlyphVersions:  map[string]int64{},
		SyntaxVersions: map[string]int64{},
		MetricsVersion: 1,
		Subs:           map[chan projectEvent]struct{}{},
	}
	for id := range glyphMap {
		state.GlyphVersions[id] = 1
	}
	for id := range syntaxMap {
		state.SyntaxVersions[id] = 1
	}
	if err := rebuildProjectSnapshot(state); err != nil {
		return nil, err
	}
	return state, nil
}

func sanitizeProjectID(raw string) string {
	if projectIDPattern.MatchString(raw) {
		return raw
	}
	return "default"
}

func (h *hub) projectFile(projectID string) string {
	filename := fmt.Sprintf("%s.json", projectID)
	return filepath.Join(h.dataDir, filename)
}

func (h *hub) projectDir(projectID string) string {
	return filepath.Join(h.dataDir, projectID)
}

func (h *hub) projectGlyphDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "glyphs")
}

func (h *hub) projectSyntaxDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "syntaxes")
}

func (h *hub) projectMetricsFile(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "metrics.json")
}

func (h *hub) projectGlyphFile(projectID, filename string) string {
	return filepath.Join(h.projectGlyphDir(projectID), filename)
}

func (h *hub) projectSyntaxFile(projectID, filename string) string {
	return filepath.Join(h.projectSyntaxDir(projectID), filename)
}

func (h *hub) loadProjectFromDisk(projectID string) (*projectDocument, error) {
	bytes, err := os.ReadFile(h.projectFile(projectID))
	if err != nil {
		return nil, err
	}

	var doc projectDocument
	if err := json.Unmarshal(bytes, &doc); err == nil && len(doc.Glyphs) > 0 {
		doc.Project = sanitizeProjectID(projectID)
		if doc.Version < 1 {
			doc.Version = 1
		}
		if doc.UpdatedAt == "" {
			doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}

		snapshot, err := normalizeSnapshot(doc.projectSnapshot)
		if err != nil {
			return nil, err
		}
		doc.projectSnapshot = snapshot
		return &doc, nil
	}

	// Backward compatibility with plain exported GTL JSON:
	// {"glyphs":[...], "syntaxes":[...], "metrics":{...}}
	var snapshot projectSnapshot
	if err := json.Unmarshal(bytes, &snapshot); err != nil {
		return nil, err
	}
	normalized, err := normalizeSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	compatDoc := &projectDocument{
		Project:         projectID,
		Version:         1,
		UpdatedAt:       now,
		projectSnapshot: normalized,
	}

	return compatDoc, nil
}

func (h *hub) saveProjectToDisk(doc projectDocument) error {
	if err := os.MkdirAll(h.dataDir, 0o755); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	target := h.projectFile(doc.Project)
	temp := target + ".tmp"

	if err := os.WriteFile(temp, bytes, 0o644); err != nil {
		return err
	}

	return os.Rename(temp, target)
}

func writeJSONAtomic(target string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temp := target + ".tmp"
	if err := os.WriteFile(temp, bytes, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func removeStaleEntityFiles(dir string, expectedFiles map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if pathpkg.Ext(name) != ".json" {
			continue
		}
		if _, ok := expectedFiles[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func sanitizeEntityFilenameBase(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var out strings.Builder
	for _, r := range trimmed {
		switch r {
		case '/', '\\':
			out.WriteRune('_')
		default:
			if r < 32 || r == 127 {
				out.WriteRune('_')
				continue
			}
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}

func entityNameFromRaw(raw json.RawMessage) string {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Name)
}

func entityFileNamesByID(items map[string]json.RawMessage) map[string]string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make(map[string]string, len(items))
	used := map[string]struct{}{}
	for _, id := range ids {
		base := sanitizeEntityFilenameBase(entityNameFromRaw(items[id]))
		if base == "" {
			base = sanitizeEntityFilenameBase(id)
		}
		if base == "" {
			base = "unnamed"
		}

		filename := fmt.Sprintf("%s.json", base)
		if _, exists := used[filename]; exists {
			suffix := sanitizeEntityFilenameBase(id)
			if suffix == "" {
				suffix = fmt.Sprintf("id-%d", len(used)+1)
			}
			filename = fmt.Sprintf("%s--%s.json", base, suffix)
			for {
				if _, stillExists := used[filename]; !stillExists {
					break
				}
				filename = fmt.Sprintf("%s--%s-%d.json", base, suffix, len(used)+1)
			}
		}

		out[id] = filename
		used[filename] = struct{}{}
	}
	return out
}

func (h *hub) saveProjectStateToDisk(projectID string, state *projectState) error {
	if err := h.saveProjectToDisk(state.Doc); err != nil {
		return err
	}

	glyphFilesByID := entityFileNamesByID(state.Glyphs)
	glyphExpectedFiles := make(map[string]struct{}, len(glyphFilesByID))
	for id, glyphRaw := range state.Glyphs {
		glyphBytes, err := json.MarshalIndent(json.RawMessage(glyphRaw), "", "  ")
		if err != nil {
			return err
		}
		filename := glyphFilesByID[id]
		glyphExpectedFiles[filename] = struct{}{}
		if err := writeJSONAtomic(h.projectGlyphFile(projectID, filename), glyphBytes); err != nil {
			return err
		}
	}
	if err := removeStaleEntityFiles(h.projectGlyphDir(projectID), glyphExpectedFiles); err != nil {
		return err
	}

	syntaxFilesByID := entityFileNamesByID(state.Syntaxes)
	syntaxExpectedFiles := make(map[string]struct{}, len(syntaxFilesByID))
	for id, syntaxRaw := range state.Syntaxes {
		syntaxBytes, err := json.MarshalIndent(json.RawMessage(syntaxRaw), "", "  ")
		if err != nil {
			return err
		}
		filename := syntaxFilesByID[id]
		syntaxExpectedFiles[filename] = struct{}{}
		if err := writeJSONAtomic(h.projectSyntaxFile(projectID, filename), syntaxBytes); err != nil {
			return err
		}
	}
	if err := removeStaleEntityFiles(h.projectSyntaxDir(projectID), syntaxExpectedFiles); err != nil {
		return err
	}

	metricsBytes, err := json.MarshalIndent(json.RawMessage(state.Metrics), "", "  ")
	if err != nil {
		return err
	}
	if err := writeJSONAtomic(h.projectMetricsFile(projectID), metricsBytes); err != nil {
		return err
	}

	return nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneRawMap(input map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(input))
	for key, value := range input {
		out[key] = cloneRawMessage(value)
	}
	return out
}

func cloneInt64Map(input map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneProjectStateForPersist(state *projectState) *projectState {
	return &projectState{
		Doc:            state.Doc,
		Glyphs:         cloneRawMap(state.Glyphs),
		Syntaxes:       cloneRawMap(state.Syntaxes),
		Metrics:        cloneRawMessage(state.Metrics),
		GlyphVersions:  cloneInt64Map(state.GlyphVersions),
		SyntaxVersions: cloneInt64Map(state.SyntaxVersions),
		MetricsVersion: state.MetricsVersion,
		Subs:           nil,
	}
}

func collectSubscriberChannels(state *projectState) []chan projectEvent {
	channels := make([]chan projectEvent, 0, len(state.Subs))
	for ch := range state.Subs {
		channels = append(channels, ch)
	}
	return channels
}

func publishProjectEvent(channels []chan projectEvent, event projectEvent) {
	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			// Channel is full; drop one stale queued event and try to enqueue the latest.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- event:
			default:
				// Still not writable (e.g. unbuffered/no receiver); skip this subscriber.
			}
		}
	}
}

func (h *hub) loadStateFromDisk(projectID string) (*projectState, bool, error) {
	doc, err := h.loadProjectFromDisk(projectID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	state, err := newProjectStateFromDocument(*doc)
	if err != nil {
		return nil, false, err
	}
	return state, true, nil
}

func newEmptyProjectState(projectID string) (*projectState, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	state := &projectState{
		Doc: projectDocument{
			Project:   projectID,
			Version:   0,
			UpdatedAt: now,
		},
		Glyphs:         map[string]json.RawMessage{},
		Syntaxes:       map[string]json.RawMessage{},
		Metrics:        json.RawMessage(`{}`),
		GlyphVersions:  map[string]int64{},
		SyntaxVersions: map[string]int64{},
		MetricsVersion: 0,
		Subs:           map[chan projectEvent]struct{}{},
	}
	if err := rebuildProjectSnapshot(state); err != nil {
		return nil, err
	}
	return state, nil
}

func mergeVersionMap(existing map[string]int64, next map[string]json.RawMessage, current map[string]json.RawMessage) map[string]int64 {
	out := make(map[string]int64, len(next))
	for id, raw := range next {
		prevRaw, hadPrev := current[id]
		prevVersion := existing[id]
		if !hadPrev {
			out[id] = 1
			continue
		}
		if string(prevRaw) == string(raw) {
			if prevVersion < 1 {
				out[id] = 1
			} else {
				out[id] = prevVersion
			}
			continue
		}
		if prevVersion < 1 {
			out[id] = 1
		} else {
			out[id] = prevVersion + 1
		}
	}
	return out
}

func (h *hub) getOrCreateProjectStateLocked(projectID string) (*projectState, error) {
	if state, ok := h.projects[projectID]; ok {
		return state, nil
	}

	loadedState, loaded, err := h.loadStateFromDisk(projectID)
	if err != nil {
		return nil, err
	}
	if loaded && loadedState != nil {
		h.projects[projectID] = loadedState
		return loadedState, nil
	}

	state, err := newEmptyProjectState(projectID)
	if err != nil {
		return nil, err
	}
	h.projects[projectID] = state
	return state, nil
}

func (h *hub) getProject(projectID string) (projectDocument, bool, error) {
	projectID = sanitizeProjectID(projectID)

	h.mu.RLock()
	if state, ok := h.projects[projectID]; ok {
		doc := state.Doc
		h.mu.RUnlock()
		return doc, true, nil
	}
	h.mu.RUnlock()

	loadedState, exists, err := h.loadStateFromDisk(projectID)
	if err != nil {
		return projectDocument{}, false, err
	}
	if !exists || loadedState == nil {
		return projectDocument{}, false, nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if state, ok := h.projects[projectID]; ok {
		return state.Doc, true, nil
	}
	h.projects[projectID] = loadedState
	return loadedState.Doc, true, nil
}

func (h *hub) getProjectResponse(projectID string) (projectResponse, bool, error) {
	projectID = sanitizeProjectID(projectID)

	h.mu.RLock()
	if state, ok := h.projects[projectID]; ok {
		resp := projectResponse{
			projectDocument: state.Doc,
			GlyphVersions:   cloneInt64Map(state.GlyphVersions),
			SyntaxVersions:  cloneInt64Map(state.SyntaxVersions),
			MetricsVersion:  state.MetricsVersion,
		}
		h.mu.RUnlock()
		return resp, true, nil
	}
	h.mu.RUnlock()

	loadedState, exists, err := h.loadStateFromDisk(projectID)
	if err != nil {
		return projectResponse{}, false, err
	}
	if !exists || loadedState == nil {
		return projectResponse{}, false, nil
	}

	h.mu.Lock()
	if state, ok := h.projects[projectID]; ok {
		resp := projectResponse{
			projectDocument: state.Doc,
			GlyphVersions:   cloneInt64Map(state.GlyphVersions),
			SyntaxVersions:  cloneInt64Map(state.SyntaxVersions),
			MetricsVersion:  state.MetricsVersion,
		}
		h.mu.Unlock()
		return resp, true, nil
	}
	h.projects[projectID] = loadedState
	resp := projectResponse{
		projectDocument: loadedState.Doc,
		GlyphVersions:   cloneInt64Map(loadedState.GlyphVersions),
		SyntaxVersions:  cloneInt64Map(loadedState.SyntaxVersions),
		MetricsVersion:  loadedState.MetricsVersion,
	}
	h.mu.Unlock()
	return resp, true, nil
}

func (h *hub) subscribe(projectID string, out chan projectEvent) (projectDocument, bool, error) {
	projectID = sanitizeProjectID(projectID)

	doc, exists, err := h.getProject(projectID)
	if err != nil {
		return projectDocument{}, false, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.projects[projectID]
	if !ok {
		state, err = newEmptyProjectState(projectID)
		if err != nil {
			return projectDocument{}, false, err
		}
		h.projects[projectID] = state
		doc = state.Doc
		exists = false
	}
	state.Subs[out] = struct{}{}

	return doc, exists, nil
}

func (h *hub) unsubscribe(projectID string, out chan projectEvent) {
	projectID = sanitizeProjectID(projectID)

	h.mu.Lock()
	defer h.mu.Unlock()

	if state, ok := h.projects[projectID]; ok {
		delete(state.Subs, out)
	}
}

func (h *hub) updateProject(projectID string, req updateProjectRequest) (projectDocument, error) {
	projectID = sanitizeProjectID(projectID)
	snapshot, err := normalizeSnapshot(req.projectSnapshot)
	if err != nil {
		return projectDocument{}, err
	}

	nextGlyphs, err := parseEntityArrayByID(snapshot.Glyphs, "glyphs")
	if err != nil {
		return projectDocument{}, err
	}
	nextSyntaxes, err := parseEntityArrayByID(snapshot.Syntaxes, "syntaxes")
	if err != nil {
		return projectDocument{}, err
	}
	nextMetrics, err := normalizedRawObject(snapshot.Metrics, "metrics")
	if err != nil {
		return projectDocument{}, err
	}

	var (
		doc         projectDocument
		persistCopy *projectState
		channels    []chan projectEvent
	)

	h.mu.Lock()
	state, ok := h.projects[projectID]
	if !ok {
		loadedState, loaded, err := h.loadStateFromDisk(projectID)
		if err != nil {
			h.mu.Unlock()
			return projectDocument{}, err
		}
		if loaded && loadedState != nil {
			state = loadedState
		} else {
			state, err = newEmptyProjectState(projectID)
			if err != nil {
				h.mu.Unlock()
				return projectDocument{}, err
			}
		}
		h.projects[projectID] = state
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return projectDocument{}, errors.New("missing baseVersion")
	}
	if *req.BaseVersion != state.Doc.Version {
		conflictDoc := state.Doc
		h.mu.Unlock()
		return projectDocument{}, &versionConflictError{
			ExpectedVersion: *req.BaseVersion,
			Current:         conflictDoc,
		}
	}

	state.GlyphVersions = mergeVersionMap(state.GlyphVersions, nextGlyphs, state.Glyphs)
	state.SyntaxVersions = mergeVersionMap(state.SyntaxVersions, nextSyntaxes, state.Syntaxes)
	if string(state.Metrics) != string(nextMetrics) {
		if state.MetricsVersion < 1 {
			state.MetricsVersion = 1
		} else {
			state.MetricsVersion++
		}
	}

	state.Glyphs = nextGlyphs
	state.Syntaxes = nextSyntaxes
	state.Metrics = nextMetrics

	state.Doc.Project = projectID
	state.Doc.Version++
	state.Doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := rebuildProjectSnapshot(state); err != nil {
		h.mu.Unlock()
		return projectDocument{}, err
	}

	doc = state.Doc
	persistCopy = cloneProjectStateForPersist(state)
	channels = collectSubscriberChannels(state)
	h.mu.Unlock()

	if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
		return projectDocument{}, err
	}

	publishProjectEvent(channels, projectEvent{
		Type:            "snapshot",
		ClientID:        req.ClientID,
		projectDocument: doc,
	})

	return doc, nil
}

func applyProjectMutation(state *projectState, projectID string) error {
	state.Doc.Project = projectID
	state.Doc.Version++
	state.Doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return rebuildProjectSnapshot(state)
}

func (h *hub) updateGlyph(projectID string, req updateGlyphRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	id, glyphRaw, err := parseEntityItem(req.Glyph, "glyph")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	var (
		response    entityUpdateResponse
		persistCopy *projectState
		channels    []chan projectEvent
		event       *projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.GlyphVersions[id]
	currentGlyph, hasGlyph := state.Glyphs[id]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "glyph",
			EntityID:        id,
			EntityDeleted:   !hasGlyph,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(currentGlyph),
		}
	}

	nextVersion := currentVersion
	if !hasGlyph {
		nextVersion = 1
	} else if string(currentGlyph) != string(glyphRaw) {
		if nextVersion < 1 {
			nextVersion = 1
		} else {
			nextVersion++
		}
	}

	if !hasGlyph || string(currentGlyph) != string(glyphRaw) {
		state.Glyphs[id] = glyphRaw
		state.GlyphVersions[id] = nextVersion
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "glyph_upsert",
			ClientID:        req.ClientID,
			Entity:          "glyph",
			EntityID:        id,
			EntityVersion:   nextVersion,
			Payload:         cloneRawMessage(glyphRaw),
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "glyph",
		EntityID:       id,
		Version:        nextVersion,
		ProjectVersion: state.Doc.Version,
		UpdatedAt:      state.Doc.UpdatedAt,
		Payload:        cloneRawMessage(glyphRaw),
	}
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return entityUpdateResponse{}, err
		}
	}
	if event != nil {
		publishProjectEvent(channels, *event)
	}

	return response, nil
}

func (h *hub) deleteGlyph(projectID string, req deleteGlyphRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return entityUpdateResponse{}, errors.New("missing id")
	}

	var (
		response    entityUpdateResponse
		persistCopy *projectState
		channels    []chan projectEvent
		event       *projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.GlyphVersions[id]
	currentGlyph, hasGlyph := state.Glyphs[id]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "glyph",
			EntityID:        id,
			EntityDeleted:   !hasGlyph,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(currentGlyph),
		}
	}

	if hasGlyph {
		delete(state.Glyphs, id)
		delete(state.GlyphVersions, id)
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "glyph_delete",
			ClientID:        req.ClientID,
			Entity:          "glyph",
			EntityID:        id,
			EntityVersion:   currentVersion,
			EntityDeleted:   true,
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "glyph",
		EntityID:       id,
		Version:        currentVersion,
		ProjectVersion: state.Doc.Version,
		Deleted:        true,
		UpdatedAt:      state.Doc.UpdatedAt,
	}
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return entityUpdateResponse{}, err
		}
	}
	if event != nil {
		publishProjectEvent(channels, *event)
	}

	return response, nil
}

func (h *hub) updateSyntax(projectID string, req updateSyntaxRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	id, syntaxRaw, err := parseEntityItem(req.Syntax, "syntax")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	var (
		response    entityUpdateResponse
		persistCopy *projectState
		channels    []chan projectEvent
		event       *projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.SyntaxVersions[id]
	currentSyntax, hasSyntax := state.Syntaxes[id]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "syntax",
			EntityID:        id,
			EntityDeleted:   !hasSyntax,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(currentSyntax),
		}
	}

	nextVersion := currentVersion
	if !hasSyntax {
		nextVersion = 1
	} else if string(currentSyntax) != string(syntaxRaw) {
		if nextVersion < 1 {
			nextVersion = 1
		} else {
			nextVersion++
		}
	}

	if !hasSyntax || string(currentSyntax) != string(syntaxRaw) {
		state.Syntaxes[id] = syntaxRaw
		state.SyntaxVersions[id] = nextVersion
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "syntax_upsert",
			ClientID:        req.ClientID,
			Entity:          "syntax",
			EntityID:        id,
			EntityVersion:   nextVersion,
			Payload:         cloneRawMessage(syntaxRaw),
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "syntax",
		EntityID:       id,
		Version:        nextVersion,
		ProjectVersion: state.Doc.Version,
		UpdatedAt:      state.Doc.UpdatedAt,
		Payload:        cloneRawMessage(syntaxRaw),
	}
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return entityUpdateResponse{}, err
		}
	}
	if event != nil {
		publishProjectEvent(channels, *event)
	}

	return response, nil
}

func (h *hub) deleteSyntax(projectID string, req deleteSyntaxRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return entityUpdateResponse{}, errors.New("missing id")
	}

	var (
		response    entityUpdateResponse
		persistCopy *projectState
		channels    []chan projectEvent
		event       *projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.SyntaxVersions[id]
	currentSyntax, hasSyntax := state.Syntaxes[id]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "syntax",
			EntityID:        id,
			EntityDeleted:   !hasSyntax,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(currentSyntax),
		}
	}

	if hasSyntax {
		delete(state.Syntaxes, id)
		delete(state.SyntaxVersions, id)
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "syntax_delete",
			ClientID:        req.ClientID,
			Entity:          "syntax",
			EntityID:        id,
			EntityVersion:   currentVersion,
			EntityDeleted:   true,
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "syntax",
		EntityID:       id,
		Version:        currentVersion,
		ProjectVersion: state.Doc.Version,
		Deleted:        true,
		UpdatedAt:      state.Doc.UpdatedAt,
	}
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return entityUpdateResponse{}, err
		}
	}
	if event != nil {
		publishProjectEvent(channels, *event)
	}

	return response, nil
}

func (h *hub) updateMetrics(projectID string, req updateMetricsRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	metricsRaw, err := normalizedRawObject(req.Metrics, "metrics")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	var (
		response    entityUpdateResponse
		persistCopy *projectState
		channels    []chan projectEvent
		event       *projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.MetricsVersion
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "metrics",
			EntityID:        "",
			EntityDeleted:   false,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(state.Metrics),
		}
	}

	nextVersion := currentVersion
	if string(state.Metrics) != string(metricsRaw) {
		if nextVersion < 1 {
			nextVersion = 1
		} else {
			nextVersion++
		}
		state.Metrics = metricsRaw
		state.MetricsVersion = nextVersion
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "metrics_update",
			ClientID:        req.ClientID,
			Entity:          "metrics",
			EntityVersion:   nextVersion,
			Payload:         cloneRawMessage(metricsRaw),
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "metrics",
		Version:        nextVersion,
		ProjectVersion: state.Doc.Version,
		UpdatedAt:      state.Doc.UpdatedAt,
		Payload:        cloneRawMessage(state.Metrics),
	}
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return entityUpdateResponse{}, err
		}
	}
	if event != nil {
		publishProjectEvent(channels, *event)
	}

	return response, nil
}

type server struct {
	hub         *hub
	allowOrigin string
	uiDir       string
}

func (s *server) writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if s.allowOrigin == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if origin == s.allowOrigin {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Last-Event-ID")
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func decodeRequestBody(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeEntityConflict(w http.ResponseWriter, projectID string, conflictErr *entityConflictError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(entityUpdateResponse{
		Project:        projectID,
		Entity:         conflictErr.Entity,
		EntityID:       conflictErr.EntityID,
		Version:        conflictErr.CurrentVersion,
		ProjectVersion: conflictErr.ProjectVersion,
		Deleted:        conflictErr.EntityDeleted,
		UpdatedAt:      conflictErr.UpdatedAt,
		Payload:        conflictErr.Payload,
	})
}

func (s *server) handleProject(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	switch r.Method {
	case http.MethodGet:
		resp, ok, err := s.hub.getProjectResponse(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodPut:
		defer r.Body.Close()

		var req updateProjectRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		doc, err := s.hub.updateProject(projectID, req)
		if err != nil {
			var conflictErr *versionConflictError
			if errors.As(err, &conflictErr) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(conflictErr.Current)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleGlyph(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	switch r.Method {
	case http.MethodPut:
		defer r.Body.Close()
		var req updateGlyphRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.updateGlyph(projectID, req)
		if err != nil {
			var conflictErr *entityConflictError
			if errors.As(err, &conflictErr) {
				writeEntityConflict(w, projectID, conflictErr)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodDelete:
		defer r.Body.Close()
		var req deleteGlyphRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.deleteGlyph(projectID, req)
		if err != nil {
			var conflictErr *entityConflictError
			if errors.As(err, &conflictErr) {
				writeEntityConflict(w, projectID, conflictErr)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleSyntax(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	switch r.Method {
	case http.MethodPut:
		defer r.Body.Close()
		var req updateSyntaxRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.updateSyntax(projectID, req)
		if err != nil {
			var conflictErr *entityConflictError
			if errors.As(err, &conflictErr) {
				writeEntityConflict(w, projectID, conflictErr)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodDelete:
		defer r.Body.Close()
		var req deleteSyntaxRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.deleteSyntax(projectID, req)
		if err != nil {
			var conflictErr *entityConflictError
			if errors.As(err, &conflictErr) {
				writeEntityConflict(w, projectID, conflictErr)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	defer r.Body.Close()
	var req updateMetricsRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	resp, err := s.hub.updateMetrics(projectID, req)
	if err != nil {
		var conflictErr *entityConflictError
		if errors.As(err, &conflictErr) {
			writeEntityConflict(w, projectID, conflictErr)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events := make(chan projectEvent, 32)
	doc, exists, err := s.hub.subscribe(projectID, events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		s.hub.unsubscribe(projectID, events)
	}()

	sendEvent := func(evt projectEvent) error {
		payload, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", evt.Type); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if exists {
		if err := sendEvent(projectEvent{
			Type:            "snapshot",
			projectDocument: doc,
		}); err != nil {
			return
		}
	}

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping %d\n\n", time.Now().UnixNano()); err != nil {
				return
			}
			flusher.Flush()
		case evt := <-events:
			if err := sendEvent(evt); err != nil {
				return
			}
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.uiDir == "" {
		http.NotFound(w, r)
		return
	}

	cleanPath := pathpkg.Clean("/" + r.URL.Path)
	relativePath := strings.TrimPrefix(cleanPath, "/")
	if relativePath == "" || relativePath == "." {
		relativePath = "index.html"
	}

	serveIfExists := func(path string) bool {
		target := filepath.Join(s.uiDir, filepath.FromSlash(path))
		if !fileExists(target) {
			return false
		}
		http.ServeFile(w, r, target)
		return true
	}

	if pathpkg.Ext(relativePath) != "" {
		if serveIfExists(relativePath) {
			return
		}
		http.NotFound(w, r)
		return
	}

	if serveIfExists(relativePath + ".html") {
		return
	}

	if serveIfExists(pathpkg.Join(relativePath, "index.html")) {
		return
	}

	if serveIfExists("index.html") {
		return
	}

	http.NotFound(w, r)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/project", s.handleProject)
	mux.HandleFunc("/api/glyph", s.handleGlyph)
	mux.HandleFunc("/api/syntax", s.handleSyntax)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/events", s.handleEvents)

	if s.uiDir != "" {
		mux.HandleFunc("/", s.handleUI)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			s.writeCORS(w, r)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("chirone collab server\n"))
		})
	}

	return requestLogger(mux)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func run(ctx context.Context, addr, dataDir, allowOrigin, uiDir string) error {
	srv := &server{
		hub:         newHub(dataDir),
		allowOrigin: allowOrigin,
		uiDir:       strings.TrimSpace(uiDir),
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.routes(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if srv.uiDir != "" {
		log.Printf("collab server listening on %s (data dir: %s, ui dir: %s)", addr, dataDir, srv.uiDir)
	} else {
		log.Printf("collab server listening on %s (data dir: %s)", addr, dataDir)
	}
	return httpServer.ListenAndServe()
}

func main() {
	addr := flag.String("addr", ":8090", "address to listen on")
	dataDir := flag.String("data-dir", "./data", "directory where project snapshots are stored")
	allowOrigin := flag.String("allow-origin", "*", "CORS allowed origin (or * for all)")
	uiDir := flag.String("ui-dir", "", "optional directory to serve static UI files from")
	flag.Parse()

	ctx := context.Background()
	if err := run(ctx, *addr, *dataDir, *allowOrigin, *uiDir); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
