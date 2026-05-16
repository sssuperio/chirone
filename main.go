package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

var version = "dev"

const defaultRuntimeSyncAPIBase = "https://chirone.sssuper.io"

func adminPassword() string {
	if pw := os.Getenv("CHIRONE_ADMIN_PASSWORD"); pw != "" {
		return pw
	}
	return "ch1r0ne"
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

//go:embed all:web/dist
var embeddedAssets embed.FS

var embeddedUI fs.FS

func init() {
	sub, err := fs.Sub(embeddedAssets, "web/dist")
	if err == nil {
		embeddedUI = sub
	}
}

type projectSnapshot struct {
	Glyphs          json.RawMessage `json:"glyphs"`
	Syntaxes        json.RawMessage `json:"syntaxes"`
	Metrics         json.RawMessage `json:"metrics"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	MetricsPresets  json.RawMessage `json:"metricsPresets,omitempty"`
	MetadataPresets json.RawMessage `json:"metadataPresets,omitempty"`
	Fonts           json.RawMessage `json:"fonts,omitempty"`
}

type projectDocument struct {
	Project      string `json:"project"`
	Version      int64  `json:"version"`
	UpdatedAt    string `json:"updatedAt"`
	PasswordHash string `json:"passwordHash,omitempty"`
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
	FontID      string          `json:"fontId,omitempty"`
	Glyph       json.RawMessage `json:"glyph"`
}

type deleteGlyphRequest struct {
	ClientID    string `json:"clientId"`
	BaseVersion *int64 `json:"baseVersion,omitempty"`
	FontID      string `json:"fontId,omitempty"`
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

type updateMetadataRequest struct {
	ClientID    string          `json:"clientId"`
	BaseVersion *int64          `json:"baseVersion,omitempty"`
	Metadata    json.RawMessage `json:"metadata"`
}

type updateFontRequest struct {
	ClientID    string          `json:"clientId"`
	BaseVersion *int64          `json:"baseVersion,omitempty"`
	Font        json.RawMessage `json:"font"`
}

type deleteFontRequest struct {
	ClientID    string `json:"clientId"`
	BaseVersion *int64 `json:"baseVersion,omitempty"`
	ID          string `json:"id"`
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
	GlyphVersions           map[string]int64 `json:"glyphVersions,omitempty"`
	SyntaxVersions          map[string]int64 `json:"syntaxVersions,omitempty"`
	MetricsVersion          int64            `json:"metricsVersion,omitempty"`
	MetricsPresetsVersions  map[string]int64 `json:"metricsPresetsVersions,omitempty"`
	MetadataPresetsVersions map[string]int64 `json:"metadataPresetsVersions,omitempty"`
	FontsVersions           map[string]int64 `json:"fontsVersions,omitempty"`
}

type projectVersionResponse struct {
	Project   string `json:"project"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

type revisionDocument struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
	projectSnapshot
}

type revisionMeta struct {
	ID        string `json:"id"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
}

type revisionsResponse struct {
	Project          string         `json:"project"`
	CurrentVersion   int64          `json:"currentVersion"`
	SuggestedMessage string         `json:"suggestedMessage"`
	Revisions        []revisionMeta `json:"revisions"`
}

type createRevisionRequest struct {
	ClientID string `json:"clientId,omitempty"`
	Message  string `json:"message"`
}

type createRevisionResponse struct {
	Project          string       `json:"project"`
	SuggestedMessage string       `json:"suggestedMessage"`
	Revision         revisionMeta `json:"revision"`
}

type revertRevisionRequest struct {
	ClientID string `json:"clientId,omitempty"`
	ID       string `json:"id"`
}

type appVersionResponse struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
}

type projectMeta struct {
	Project     string `json:"project"`
	Version     int64  `json:"version"`
	UpdatedAt   string `json:"updatedAt"`
	HasPassword bool   `json:"hasPassword"`
}

type projectListResponse struct {
	Projects []projectMeta `json:"projects"`
}

type projectAuthRequest struct {
	Password string `json:"password"`
}

type projectSetPasswordRequest struct {
	Password    string `json:"password"`
	OldPassword string `json:"oldPassword,omitempty"`
}

func hashPassword(password string) string {
	if password == "" {
		return ""
	}
	h := sha256.Sum256([]byte("chirone:" + password))
	return hex.EncodeToString(h[:])
}

func verifyPassword(password, hash string) bool {
	if hash == "" {
		return true
	}
	return hashPassword(password) == hash
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
	Doc                    projectDocument
	Glyphs                 map[string]json.RawMessage
	Syntaxes               map[string]json.RawMessage
	Metrics                json.RawMessage
	Metadata               json.RawMessage
	MetricsPresets         map[string]json.RawMessage
	MetadataPresets        map[string]json.RawMessage
	Fonts                  map[string]json.RawMessage
	GlyphVersions          map[string]int64
	SyntaxVersions         map[string]int64
	MetricsVersion         int64
	MetadataVersion        int64
	MetricsPresetsVersion  map[string]int64
	MetadataPresetsVersion map[string]int64
	FontsVersion           map[string]int64
	Subs                   map[chan projectEvent]struct{}
}

type hub struct {
	mu       sync.RWMutex
	projects map[string]*projectState
	dataDir  string
}

var (
	projectIDPattern  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	revisionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
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

	if len(snapshot.Metadata) == 0 {
		out.Metadata = json.RawMessage(`{}`)
	} else {
		if !json.Valid(snapshot.Metadata) {
			return out, errors.New("metadata is not valid JSON")
		}
		out.Metadata = snapshot.Metadata
	}

	if len(snapshot.MetricsPresets) == 0 {
		out.MetricsPresets = json.RawMessage(`[]`)
	} else {
		if !json.Valid(snapshot.MetricsPresets) {
			return out, errors.New("metricsPresets is not valid JSON")
		}
		out.MetricsPresets = snapshot.MetricsPresets
	}

	if len(snapshot.MetadataPresets) == 0 {
		out.MetadataPresets = json.RawMessage(`[]`)
	} else {
		if !json.Valid(snapshot.MetadataPresets) {
			return out, errors.New("metadataPresets is not valid JSON")
		}
		out.MetadataPresets = snapshot.MetadataPresets
	}

	if len(snapshot.Fonts) == 0 {
		out.Fonts = json.RawMessage(`[]`)
	} else {
		if !json.Valid(snapshot.Fonts) {
			return out, errors.New("fonts is not valid JSON")
		}
		out.Fonts = snapshot.Fonts
	}

	return out, nil
}

type entityID struct {
	ID string `json:"id"`
}

func jsonField(raw json.RawMessage, key string) (string, bool) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
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
	metadata := state.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	metricsPresets, err := serializeEntityMap(state.MetricsPresets)
	if err != nil {
		return err
	}
	metadataPresets, err := serializeEntityMap(state.MetadataPresets)
	if err != nil {
		return err
	}
	fonts, err := serializeEntityMap(state.Fonts)
	if err != nil {
		return err
	}

	state.Doc.projectSnapshot = projectSnapshot{
		Glyphs:          glyphs,
		Syntaxes:        syntaxes,
		Metrics:         metrics,
		Metadata:        metadata,
		MetricsPresets:  metricsPresets,
		MetadataPresets: metadataPresets,
		Fonts:           fonts,
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

	metadata, err := normalizedRawObject(snapshot.Metadata, "metadata")
	if err != nil {
		return nil, err
	}

	metricsPresetsMap, err := parseEntityArrayByID(snapshot.MetricsPresets, "metricsPresets")
	if err != nil {
		return nil, err
	}
	metadataPresetsMap, err := parseEntityArrayByID(snapshot.MetadataPresets, "metadataPresets")
	if err != nil {
		return nil, err
	}
	fontsMap, err := parseEntityArrayByID(snapshot.Fonts, "fonts")
	if err != nil {
		return nil, err
	}

	state := &projectState{
		Doc:                    doc,
		Glyphs:                 glyphMap,
		Syntaxes:               syntaxMap,
		Metrics:                metrics,
		Metadata:               metadata,
		MetricsPresets:         metricsPresetsMap,
		MetadataPresets:        metadataPresetsMap,
		Fonts:                  fontsMap,
		GlyphVersions:          map[string]int64{},
		SyntaxVersions:         map[string]int64{},
		MetricsVersion:         1,
		MetricsPresetsVersion:  map[string]int64{},
		MetadataPresetsVersion: map[string]int64{},
		FontsVersion:           map[string]int64{},
		Subs:                   map[chan projectEvent]struct{}{},
	}
	for id := range glyphMap {
		state.GlyphVersions[id] = 1
	}
	for id := range syntaxMap {
		state.SyntaxVersions[id] = 1
	}
	for id := range metricsPresetsMap {
		state.MetricsPresetsVersion[id] = 1
	}
	for id := range metadataPresetsMap {
		state.MetadataPresetsVersion[id] = 1
	}
	for id := range fontsMap {
		state.FontsVersion[id] = 1
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

func (h *hub) projectMetadataFile(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "metadata.json")
}

func (h *hub) projectMetricsPresetsDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "metrics")
}

func (h *hub) projectMetricsPresetsFile(projectID, filename string) string {
	return filepath.Join(h.projectMetricsPresetsDir(projectID), filename)
}

func (h *hub) projectMetadataPresetsDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "metadata")
}

func (h *hub) projectMetadataPresetsFile(projectID, filename string) string {
	return filepath.Join(h.projectMetadataPresetsDir(projectID), filename)
}

func (h *hub) projectFontsDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "fonts")
}

func (h *hub) projectFontsFile(projectID, filename string) string {
	return filepath.Join(h.projectFontsDir(projectID), filename)
}

func (h *hub) projectFontGlyphDir(projectID, fontID string) string {
	return filepath.Join(h.projectDir(projectID), "font-glyphs", fontID)
}

func (h *hub) projectFontGlyphFile(projectID, fontID, filename string) string {
	return filepath.Join(h.projectFontGlyphDir(projectID, fontID), filename)
}

func (h *hub) projectGlyphFile(projectID, filename string) string {
	return filepath.Join(h.projectGlyphDir(projectID), filename)
}

func (h *hub) projectSyntaxFile(projectID, filename string) string {
	return filepath.Join(h.projectSyntaxDir(projectID), filename)
}

func (h *hub) projectRevisionDir(projectID string) string {
	return filepath.Join(h.projectDir(projectID), "revisions")
}

func (h *hub) projectRevisionFile(projectID, revisionID string) string {
	return filepath.Join(h.projectRevisionDir(projectID), fmt.Sprintf("%s.json", revisionID))
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

	metadataBytes, err := json.MarshalIndent(json.RawMessage(state.Metadata), "", "  ")
	if err != nil {
		return err
	}
	if err := writeJSONAtomic(h.projectMetadataFile(projectID), metadataBytes); err != nil {
		return err
	}

	metricsPresetsFilesByID := entityFileNamesByID(state.MetricsPresets)
	metricsPresetsExpectedFiles := make(map[string]struct{}, len(metricsPresetsFilesByID))
	for id, raw := range state.MetricsPresets {
		entryBytes, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return err
		}
		filename := metricsPresetsFilesByID[id]
		metricsPresetsExpectedFiles[filename] = struct{}{}
		if err := writeJSONAtomic(h.projectMetricsPresetsFile(projectID, filename), entryBytes); err != nil {
			return err
		}
	}
	if err := removeStaleEntityFiles(h.projectMetricsPresetsDir(projectID), metricsPresetsExpectedFiles); err != nil {
		return err
	}

	metadataPresetsFilesByID := entityFileNamesByID(state.MetadataPresets)
	metadataPresetsExpectedFiles := make(map[string]struct{}, len(metadataPresetsFilesByID))
	for id, raw := range state.MetadataPresets {
		entryBytes, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return err
		}
		filename := metadataPresetsFilesByID[id]
		metadataPresetsExpectedFiles[filename] = struct{}{}
		if err := writeJSONAtomic(h.projectMetadataPresetsFile(projectID, filename), entryBytes); err != nil {
			return err
		}
	}
	if err := removeStaleEntityFiles(h.projectMetadataPresetsDir(projectID), metadataPresetsExpectedFiles); err != nil {
		return err
	}

	fontsFilesByID := entityFileNamesByID(state.Fonts)
	fontsExpectedFiles := make(map[string]struct{}, len(fontsFilesByID))
	for id, raw := range state.Fonts {
		entryBytes, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return err
		}
		filename := fontsFilesByID[id]
		fontsExpectedFiles[filename] = struct{}{}
		if err := writeJSONAtomic(h.projectFontsFile(projectID, filename), entryBytes); err != nil {
			return err
		}
	}
	if err := removeStaleEntityFiles(h.projectFontsDir(projectID), fontsExpectedFiles); err != nil {
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
		Doc:                    state.Doc,
		Glyphs:                 cloneRawMap(state.Glyphs),
		Syntaxes:               cloneRawMap(state.Syntaxes),
		Metrics:                cloneRawMessage(state.Metrics),
		Metadata:               cloneRawMessage(state.Metadata),
		MetricsPresets:         cloneRawMap(state.MetricsPresets),
		MetadataPresets:        cloneRawMap(state.MetadataPresets),
		Fonts:                  cloneRawMap(state.Fonts),
		GlyphVersions:          cloneInt64Map(state.GlyphVersions),
		SyntaxVersions:         cloneInt64Map(state.SyntaxVersions),
		MetricsVersion:         state.MetricsVersion,
		MetricsPresetsVersion:  cloneInt64Map(state.MetricsPresetsVersion),
		MetadataPresetsVersion: cloneInt64Map(state.MetadataPresetsVersion),
		FontsVersion:           cloneInt64Map(state.FontsVersion),
		Subs:                   nil,
	}
}

func cloneProjectSnapshot(snapshot projectSnapshot) projectSnapshot {
	return projectSnapshot{
		Glyphs:          cloneRawMessage(snapshot.Glyphs),
		Syntaxes:        cloneRawMessage(snapshot.Syntaxes),
		Metrics:         cloneRawMessage(snapshot.Metrics),
		Metadata:        cloneRawMessage(snapshot.Metadata),
		MetricsPresets:  cloneRawMessage(snapshot.MetricsPresets),
		MetadataPresets: cloneRawMessage(snapshot.MetadataPresets),
		Fonts:           cloneRawMessage(snapshot.Fonts),
	}
}

func projectResponseFromState(state *projectState) projectResponse {
	return projectResponse{
		projectDocument:         state.Doc,
		GlyphVersions:           cloneInt64Map(state.GlyphVersions),
		SyntaxVersions:          cloneInt64Map(state.SyntaxVersions),
		MetricsVersion:          state.MetricsVersion,
		MetricsPresetsVersions:  cloneInt64Map(state.MetricsPresetsVersion),
		MetadataPresetsVersions: cloneInt64Map(state.MetadataPresetsVersion),
		FontsVersions:           cloneInt64Map(state.FontsVersion),
	}
}

func entityRawMapsEqual(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for key, aValue := range a {
		bValue, ok := b[key]
		if !ok {
			return false
		}
		if string(aValue) != string(bValue) {
			return false
		}
	}
	return true
}

func listNameSummary(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sort.Strings(values)
	const maxNames = 8
	if len(values) <= maxNames {
		return strings.Join(values, ", ")
	}
	return fmt.Sprintf("%s (+%d)", strings.Join(values[:maxNames], ", "), len(values)-maxNames)
}

func entityDisplayName(raw json.RawMessage, id string) string {
	name := strings.TrimSpace(entityNameFromRaw(raw))
	if name == "" {
		return id
	}
	return name
}

func buildEntityDiffSegment(label string, currentRaw, previousRaw json.RawMessage) string {
	currentMap, err := parseEntityArrayByID(currentRaw, label)
	if err != nil {
		return ""
	}
	previousMap, err := parseEntityArrayByID(previousRaw, label)
	if err != nil {
		return ""
	}

	added := make([]string, 0)
	changed := make([]string, 0)
	removed := make([]string, 0)

	for id, currentItem := range currentMap {
		previousItem, hadPrevious := previousMap[id]
		if !hadPrevious {
			added = append(added, entityDisplayName(currentItem, id))
			continue
		}
		if string(currentItem) != string(previousItem) {
			changed = append(changed, entityDisplayName(currentItem, id))
		}
	}
	for id, previousItem := range previousMap {
		if _, stillPresent := currentMap[id]; stillPresent {
			continue
		}
		removed = append(removed, entityDisplayName(previousItem, id))
	}

	parts := make([]string, 0, 3)
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("+%d [%s]", len(added), listNameSummary(added)))
	}
	if len(changed) > 0 {
		parts = append(parts, fmt.Sprintf("~%d [%s]", len(changed), listNameSummary(changed)))
	}
	if len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("-%d [%s]", len(removed), listNameSummary(removed)))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s", label, strings.Join(parts, " "))
}

func buildSuggestedRevisionMessage(current projectSnapshot, previous *projectSnapshot) string {
	if previous == nil {
		glyphCount := 0
		syntaxCount := 0
		if parsedGlyphs, err := parseEntityArrayByID(current.Glyphs, "glyphs"); err == nil {
			glyphCount = len(parsedGlyphs)
		}
		if parsedSyntaxes, err := parseEntityArrayByID(current.Syntaxes, "syntaxes"); err == nil {
			syntaxCount = len(parsedSyntaxes)
		}
		return fmt.Sprintf("Init progetto: %d glifi, %d sintassi", glyphCount, syntaxCount)
	}

	segments := make([]string, 0, 3)
	if glyphSegment := buildEntityDiffSegment("glifi", current.Glyphs, previous.Glyphs); glyphSegment != "" {
		segments = append(segments, glyphSegment)
	}
	if syntaxSegment := buildEntityDiffSegment("sintassi", current.Syntaxes, previous.Syntaxes); syntaxSegment != "" {
		segments = append(segments, syntaxSegment)
	}
	if string(current.Metrics) != string(previous.Metrics) {
		segments = append(segments, "metriche aggiornate")
	}

	if len(segments) == 0 {
		return "Nessuna modifica rispetto all'ultima revisione"
	}
	return strings.Join(segments, " | ")
}

func revisionMetaFromDocument(doc revisionDocument) revisionMeta {
	return revisionMeta{
		ID:        doc.ID,
		Version:   doc.Version,
		CreatedAt: doc.CreatedAt,
		Message:   doc.Message,
	}
}

func (h *hub) listRevisionDocuments(projectID string) ([]revisionDocument, error) {
	entries, err := os.ReadDir(h.projectRevisionDir(projectID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []revisionDocument{}, nil
		}
		return nil, err
	}

	revisions := make([]revisionDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if pathpkg.Ext(name) != ".json" {
			continue
		}

		revisionID := strings.TrimSuffix(name, ".json")
		if !revisionIDPattern.MatchString(revisionID) {
			continue
		}

		raw, err := os.ReadFile(h.projectRevisionFile(projectID, revisionID))
		if err != nil {
			return nil, err
		}

		var doc revisionDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		if doc.ID == "" {
			doc.ID = revisionID
		}
		if doc.CreatedAt == "" {
			doc.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		doc.Message = strings.TrimSpace(doc.Message)
		if doc.Message == "" {
			doc.Message = "Revisione senza messaggio"
		}
		doc.Project = projectID

		snapshot, err := normalizeSnapshot(doc.projectSnapshot)
		if err != nil {
			return nil, err
		}
		doc.projectSnapshot = snapshot
		revisions = append(revisions, doc)
	}

	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].CreatedAt == revisions[j].CreatedAt {
			return revisions[i].ID > revisions[j].ID
		}
		return revisions[i].CreatedAt > revisions[j].CreatedAt
	})

	return revisions, nil
}

func (h *hub) loadRevisionDocument(projectID, revisionID string) (*revisionDocument, error) {
	revisionID = strings.TrimSpace(revisionID)
	if !revisionIDPattern.MatchString(revisionID) {
		return nil, errors.New("invalid revision id")
	}

	raw, err := os.ReadFile(h.projectRevisionFile(projectID, revisionID))
	if err != nil {
		return nil, err
	}

	var doc revisionDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.ID == "" {
		doc.ID = revisionID
	}
	doc.Project = projectID
	doc.Message = strings.TrimSpace(doc.Message)
	if doc.Message == "" {
		doc.Message = "Revisione senza messaggio"
	}
	if doc.CreatedAt == "" {
		doc.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	snapshot, err := normalizeSnapshot(doc.projectSnapshot)
	if err != nil {
		return nil, err
	}
	doc.projectSnapshot = snapshot

	return &doc, nil
}

func (h *hub) getRevisions(projectID string) (revisionsResponse, error) {
	projectID = sanitizeProjectID(projectID)

	project, err := h.getOrCreateProjectResponse(projectID)
	if err != nil {
		return revisionsResponse{}, err
	}

	revisions, err := h.listRevisionDocuments(projectID)
	if err != nil {
		return revisionsResponse{}, err
	}

	metas := make([]revisionMeta, 0, len(revisions))
	for _, revision := range revisions {
		metas = append(metas, revisionMetaFromDocument(revision))
	}

	var previousSnapshot *projectSnapshot
	if len(revisions) > 0 {
		previousSnapshot = &revisions[0].projectSnapshot
	}

	return revisionsResponse{
		Project:          projectID,
		CurrentVersion:   project.Version,
		SuggestedMessage: buildSuggestedRevisionMessage(project.projectSnapshot, previousSnapshot),
		Revisions:        metas,
	}, nil
}

func (h *hub) createRevision(projectID string, req createRevisionRequest) (createRevisionResponse, error) {
	projectID = sanitizeProjectID(projectID)

	project, err := h.getOrCreateProjectResponse(projectID)
	if err != nil {
		return createRevisionResponse{}, err
	}

	revisions, err := h.listRevisionDocuments(projectID)
	if err != nil {
		return createRevisionResponse{}, err
	}

	var previousSnapshot *projectSnapshot
	if len(revisions) > 0 {
		previousSnapshot = &revisions[0].projectSnapshot
	}
	suggested := buildSuggestedRevisionMessage(project.projectSnapshot, previousSnapshot)

	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = suggested
	}

	baseID := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	revisionID := baseID
	for attempt := 1; ; attempt++ {
		if _, err := os.Stat(h.projectRevisionFile(projectID, revisionID)); errors.Is(err, os.ErrNotExist) {
			break
		}
		revisionID = fmt.Sprintf("%s-%d", baseID, attempt)
	}

	revision := revisionDocument{
		ID:              revisionID,
		Project:         projectID,
		Version:         project.Version,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		Message:         message,
		projectSnapshot: cloneProjectSnapshot(project.projectSnapshot),
	}

	bytes, err := json.MarshalIndent(revision, "", "  ")
	if err != nil {
		return createRevisionResponse{}, err
	}
	if err := writeJSONAtomic(h.projectRevisionFile(projectID, revision.ID), bytes); err != nil {
		return createRevisionResponse{}, err
	}

	return createRevisionResponse{
		Project:          projectID,
		SuggestedMessage: buildSuggestedRevisionMessage(project.projectSnapshot, &revision.projectSnapshot),
		Revision:         revisionMetaFromDocument(revision),
	}, nil
}

func (h *hub) revertRevision(projectID, revisionID, clientID string) (projectResponse, error) {
	projectID = sanitizeProjectID(projectID)
	revision, err := h.loadRevisionDocument(projectID, revisionID)
	if err != nil {
		return projectResponse{}, err
	}

	nextGlyphs, err := parseEntityArrayByID(revision.Glyphs, "glyphs")
	if err != nil {
		return projectResponse{}, err
	}
	nextSyntaxes, err := parseEntityArrayByID(revision.Syntaxes, "syntaxes")
	if err != nil {
		return projectResponse{}, err
	}
	nextMetrics, err := normalizedRawObject(revision.Metrics, "metrics")
	if err != nil {
		return projectResponse{}, err
	}

	var (
		response    projectResponse
		persistCopy *projectState
		channels    []chan projectEvent
	)

	h.mu.Lock()
	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		h.mu.Unlock()
		return projectResponse{}, err
	}

	sameGlyphs := entityRawMapsEqual(state.Glyphs, nextGlyphs)
	sameSyntaxes := entityRawMapsEqual(state.Syntaxes, nextSyntaxes)
	sameMetrics := string(state.Metrics) == string(nextMetrics)

	if sameGlyphs && sameSyntaxes && sameMetrics {
		response = projectResponseFromState(state)
		h.mu.Unlock()
		return response, nil
	}

	state.GlyphVersions = mergeVersionMap(state.GlyphVersions, nextGlyphs, state.Glyphs)
	state.SyntaxVersions = mergeVersionMap(state.SyntaxVersions, nextSyntaxes, state.Syntaxes)
	if !sameMetrics {
		if state.MetricsVersion < 1 {
			state.MetricsVersion = 1
		} else {
			state.MetricsVersion++
		}
	}

	state.Glyphs = nextGlyphs
	state.Syntaxes = nextSyntaxes
	state.Metrics = nextMetrics
	if err := applyProjectMutation(state, projectID); err != nil {
		h.mu.Unlock()
		return projectResponse{}, err
	}

	response = projectResponseFromState(state)
	persistCopy = cloneProjectStateForPersist(state)
	channels = collectSubscriberChannels(state)
	h.mu.Unlock()

	if persistCopy != nil {
		if err := h.saveProjectStateToDisk(projectID, persistCopy); err != nil {
			return projectResponse{}, err
		}
	}
	publishProjectEvent(channels, projectEvent{
		Type:            "snapshot",
		ClientID:        clientID,
		projectDocument: state.Doc,
	})

	return response, nil
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
		Glyphs:                 map[string]json.RawMessage{},
		Syntaxes:               map[string]json.RawMessage{},
		Metrics:                json.RawMessage(`{}`),
		Metadata:               json.RawMessage(`{}`),
		MetricsPresets:         map[string]json.RawMessage{},
		MetadataPresets:        map[string]json.RawMessage{},
		Fonts:                  map[string]json.RawMessage{},
		GlyphVersions:          map[string]int64{},
		SyntaxVersions:         map[string]int64{},
		MetricsVersion:         0,
		MetricsPresetsVersion:  map[string]int64{},
		MetadataPresetsVersion: map[string]int64{},
		FontsVersion:           map[string]int64{},
		Subs:                   map[chan projectEvent]struct{}{},
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

func (h *hub) getOrCreateProjectResponse(projectID string) (projectResponse, error) {
	projectID = sanitizeProjectID(projectID)

	h.mu.Lock()
	defer h.mu.Unlock()

	state, err := h.getOrCreateProjectStateLocked(projectID)
	if err != nil {
		return projectResponse{}, err
	}

	return projectResponseFromState(state), nil
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

func (h *hub) updateMetadata(projectID string, req updateMetadataRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	metadataRaw, err := normalizedRawObject(req.Metadata, "metadata")
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

	currentVersion := state.MetadataVersion
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "metadata",
			EntityID:        "",
			EntityDeleted:   false,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         cloneRawMessage(state.Metadata),
		}
	}

	nextVersion := currentVersion
	if string(state.Metadata) != string(metadataRaw) {
		if nextVersion < 1 {
			nextVersion = 1
		} else {
			nextVersion++
		}
		state.Metadata = metadataRaw
		state.MetadataVersion = nextVersion
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "metadata_update",
			ClientID:        req.ClientID,
			Entity:          "metadata",
			EntityVersion:   nextVersion,
			Payload:         cloneRawMessage(metadataRaw),
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "metadata",
		Version:        nextVersion,
		ProjectVersion: state.Doc.Version,
		UpdatedAt:      state.Doc.UpdatedAt,
		Payload:        cloneRawMessage(state.Metadata),
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

func (h *hub) updateFont(projectID string, req updateFontRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)
	fontRaw, err := normalizedRawObject(req.Font, "font")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	var fontID string
	if id, ok := jsonField(fontRaw, "id"); ok {
		fontID = id
	} else {
		return entityUpdateResponse{}, errors.New("font must have an id field")
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

	currentVersion := state.FontsVersion[fontID]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "font",
			EntityID:        fontID,
			EntityDeleted:   false,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         state.Fonts[fontID],
		}
	}

	nextVersion := currentVersion
	existing, exists := state.Fonts[fontID]
	if !exists || string(existing) != string(fontRaw) {
		if nextVersion < 1 {
			nextVersion = 1
		} else {
			nextVersion++
		}
		state.Fonts[fontID] = fontRaw
		state.FontsVersion[fontID] = nextVersion
		if err := applyProjectMutation(state, projectID); err != nil {
			h.mu.Unlock()
			return entityUpdateResponse{}, err
		}
		persistCopy = cloneProjectStateForPersist(state)
		channels = collectSubscriberChannels(state)
		event = &projectEvent{
			Type:            "font_upsert",
			ClientID:        req.ClientID,
			Entity:          "font",
			EntityID:        fontID,
			EntityVersion:   nextVersion,
			Payload:         cloneRawMessage(fontRaw),
			projectDocument: state.Doc,
		}
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "font",
		EntityID:       fontID,
		Version:        nextVersion,
		ProjectVersion: state.Doc.Version,
		UpdatedAt:      state.Doc.UpdatedAt,
		Payload:        cloneRawMessage(state.Fonts[fontID]),
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

func (h *hub) deleteFont(projectID string, req deleteFontRequest) (entityUpdateResponse, error) {
	projectID = sanitizeProjectID(projectID)

	if req.ID == "" {
		return entityUpdateResponse{}, errors.New("missing font id")
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

	fontRaw, exists := state.Fonts[req.ID]
	if !exists {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("font not found")
	}

	if req.BaseVersion == nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, errors.New("missing baseVersion")
	}

	currentVersion := state.FontsVersion[req.ID]
	if *req.BaseVersion != currentVersion {
		h.mu.Unlock()
		return entityUpdateResponse{}, &entityConflictError{
			ExpectedVersion: *req.BaseVersion,
			CurrentVersion:  currentVersion,
			ProjectVersion:  state.Doc.Version,
			Entity:          "font",
			EntityID:        req.ID,
			EntityDeleted:   false,
			UpdatedAt:       state.Doc.UpdatedAt,
			Payload:         fontRaw,
		}
	}

	delete(state.Fonts, req.ID)
	delete(state.FontsVersion, req.ID)
	if err := applyProjectMutation(state, projectID); err != nil {
		h.mu.Unlock()
		return entityUpdateResponse{}, err
	}
	persistCopy = cloneProjectStateForPersist(state)
	channels = collectSubscriberChannels(state)
	event = &projectEvent{
		Type:            "font_delete",
		ClientID:        req.ClientID,
		Entity:          "font",
		EntityID:        req.ID,
		EntityDeleted:   true,
		EntityVersion:   currentVersion + 1,
		projectDocument: state.Doc,
	}

	response = entityUpdateResponse{
		Project:        projectID,
		Entity:         "font",
		EntityID:       req.ID,
		Deleted:        true,
		Version:        currentVersion + 1,
		ProjectVersion: state.Doc.Version,
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
	uiFS        fs.FS
	appVersion  string
	appSHA      string
}

func resolveGitSHA() string {
	if sha := resolveGitSHAFromBuildInfo(); sha != "" {
		return sha
	}
	if sha := resolveGitSHAFromGit(); sha != "" {
		return sha
	}
	return "unknown"
}

func resolveGitSHAFromBuildInfo() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	sha := ""
	dirty := false
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			sha = strings.TrimSpace(setting.Value)
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if sha == "" {
		return ""
	}
	if len(sha) > 12 {
		sha = sha[:12]
	}
	if dirty {
		sha += "-dirty"
	}
	return sha
}

func resolveGitSHAFromGit() string {
	shaBytes, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(shaBytes))
	if sha == "" {
		return ""
	}

	statusBytes, err := exec.Command("git", "status", "--porcelain").Output()
	if err == nil && strings.TrimSpace(string(statusBytes)) != "" {
		sha += "-dirty"
	}

	return sha
}

func (s *server) writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if s.allowOrigin == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if origin == s.allowOrigin {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Last-Event-ID")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": s.appVersion,
		"sha":     s.appSHA,
	})
}

func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
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
	_ = json.NewEncoder(w).Encode(appVersionResponse{
		Version: s.appVersion,
		SHA:     s.appSHA,
	})
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

func (s *server) projectNeedsPassword(projectID string) bool {
	doc, exists, err := s.hub.getProject(projectID)
	if err != nil || !exists {
		return false
	}
	return doc.PasswordHash != ""
}

func (s *server) checkProjectPassword(r *http.Request, projectID string) bool {
	doc, exists, err := s.hub.getProject(projectID)
	if err != nil || !exists {
		return true
	}
	if doc.PasswordHash == "" {
		return true
	}
	password := r.Header.Get("X-Chirone-Password")
	return verifyPassword(password, doc.PasswordHash)
}

type createProjectRequest struct {
	Project  string `json:"project"`
	Password string `json:"password,omitempty"`
}

func (s *server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	adminPw := r.Header.Get("X-Chirone-Admin-Password")
	if adminPw != adminPassword() {
		http.Error(w, "admin password required", http.StatusForbidden)
		return
	}

	defer func() { _ = r.Body.Close() }()
	var req createProjectRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	projectID := sanitizeProjectID(req.Project)
	if projectID == "" || projectID == "default" {
		http.Error(w, "invalid project name", http.StatusBadRequest)
		return
	}

	// Check if project already exists
	if _, exists, _ := s.hub.getProject(projectID); exists {
		http.Error(w, "project already exists", http.StatusConflict)
		return
	}

	state, err := newEmptyProjectState(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Password != "" {
		state.Doc.PasswordHash = hashPassword(req.Password)
	}

	if err := s.hub.saveProjectToDisk(state.Doc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := projectMeta{
		Project:     state.Doc.Project,
		Version:     state.Doc.Version,
		UpdatedAt:   state.Doc.UpdatedAt,
		HasPassword: state.Doc.PasswordHash != "",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleProjectAuth(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	defer func() { _ = r.Body.Close() }()
	var req projectAuthRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	doc, exists, err := s.hub.getProject(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	if !verifyPassword(req.Password, doc.PasswordHash) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) handleProjectPassword(w http.ResponseWriter, r *http.Request) {
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

	defer func() { _ = r.Body.Close() }()
	var req projectSetPasswordRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	doc, exists, err := s.hub.getProject(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	if !verifyPassword(req.OldPassword, doc.PasswordHash) {
		http.Error(w, "invalid current password", http.StatusForbidden)
		return
	}

	doc.PasswordHash = hashPassword(req.Password)
	if err := s.hub.saveProjectToDisk(doc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) handleProjectList(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h := s.hub
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries, err := os.ReadDir(h.dataDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projectListResponse{Projects: []projectMeta{}})
		return
	}

	projects := make([]projectMeta, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if pathpkg.Ext(name) != ".json" {
			continue
		}
		projectID := strings.TrimSuffix(name, ".json")
		if !projectIDPattern.MatchString(projectID) {
			continue
		}

		doc, err := h.loadProjectFromDisk(projectID)
		if err != nil {
			continue
		}

		projects = append(projects, projectMeta{
			Project:     doc.Project,
			Version:     doc.Version,
			UpdatedAt:   doc.UpdatedAt,
			HasPassword: doc.PasswordHash != "",
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Project < projects[j].Project
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(projectListResponse{Projects: projects})
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
		defer func() {
			_ = r.Body.Close()
		}()

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

func (s *server) handleProjectVersion(w http.ResponseWriter, r *http.Request) {
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

	doc, ok, err := s.hub.getProject(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	resp := projectVersionResponse{
		Project:   doc.Project,
		Version:   doc.Version,
		UpdatedAt: doc.UpdatedAt,
	}
	if resp.Project == "" {
		resp.Project = projectID
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleRevisions(w http.ResponseWriter, r *http.Request) {
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
		resp, err := s.hub.getRevisions(projectID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "project not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodPost:
		defer func() {
			_ = r.Body.Close()
		}()
		var req createRevisionRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.createRevision(projectID, req)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "project not found", http.StatusNotFound)
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

func (s *server) handleRevisionRevert(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req revertRevisionRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.hub.revertRevision(projectID, req.ID, req.ClientID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "revision not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()
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
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()
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

// Font-scoped glyph storage

func (h *hub) saveFontGlyph(projectID, fontID string, glyphRaw json.RawMessage) (entityUpdateResponse, error) {
	id, normalized, err := parseEntityItem(glyphRaw, "glyph")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	bytes, err := json.MarshalIndent(json.RawMessage(normalized), "", "  ")
	if err != nil {
		return entityUpdateResponse{}, err
	}

	filename := sanitizeEntityFilenameBase(entityNameFromRaw(normalized))
	if filename == "" {
		filename = sanitizeEntityFilenameBase(id)
	}
	if filename == "" {
		filename = "unnamed"
	}

	if err := writeJSONAtomic(h.projectFontGlyphFile(projectID, fontID, filename+".json"), bytes); err != nil {
		return entityUpdateResponse{}, err
	}

	return entityUpdateResponse{
		Project:   projectID,
		Entity:    "fontGlyph",
		EntityID:  id,
		Version:   1,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   cloneRawMessage(normalized),
	}, nil
}

func (h *hub) deleteFontGlyph(projectID, fontID, glyphID string) (entityUpdateResponse, error) {
	glyphID = strings.TrimSpace(glyphID)
	if glyphID == "" {
		return entityUpdateResponse{}, errors.New("missing id")
	}

	dir := h.projectFontGlyphDir(projectID, fontID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entityUpdateResponse{Project: projectID, Entity: "fontGlyph", EntityID: glyphID, Deleted: true}, nil
		}
		return entityUpdateResponse{}, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if pathpkg.Ext(name) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var parsed entityID
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		if parsed.ID == glyphID {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return entityUpdateResponse{}, err
			}
			return entityUpdateResponse{
				Project:   projectID,
				Entity:    "fontGlyph",
				EntityID:  glyphID,
				Deleted:   true,
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}, nil
		}
	}

	return entityUpdateResponse{Project: projectID, Entity: "fontGlyph", EntityID: glyphID, Deleted: true}, nil
}

func (h *hub) listFontGlyphs(projectID, fontID string) ([]json.RawMessage, error) {
	dir := h.projectFontGlyphDir(projectID, fontID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []json.RawMessage{}, nil
		}
		return nil, err
	}

	var result []json.RawMessage
	for _, entry := range entries {
		if entry.IsDir() || pathpkg.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		result = append(result, json.RawMessage(raw))
	}
	return result, nil
}

func (s *server) handleFontGlyph(w http.ResponseWriter, r *http.Request) {
	s.writeCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	projectID := sanitizeProjectID(r.URL.Query().Get("project"))
	if projectID == "" {
		projectID = "default"
	}
	fontID := strings.TrimSpace(r.URL.Query().Get("font"))
	if fontID == "" {
		http.Error(w, "missing font id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() { _ = r.Body.Close() }()
		var req updateGlyphRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.saveFontGlyph(projectID, fontID, req.Glyph)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodDelete:
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() { _ = r.Body.Close() }()
		var req deleteGlyphRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.deleteFontGlyph(projectID, fontID, req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleFontGlyphList(w http.ResponseWriter, r *http.Request) {
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
	fontID := strings.TrimSpace(r.URL.Query().Get("font"))
	if fontID == "" {
		http.Error(w, "missing font id", http.StatusBadRequest)
		return
	}

	glyphs, err := s.hub.listFontGlyphs(projectID, fontID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(glyphs)
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
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()
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
		defer func() {
			_ = r.Body.Close()
		}()
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

	if !s.checkProjectPassword(r, projectID) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
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
func (s *server) handleMetadata(w http.ResponseWriter, r *http.Request) {
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

	if !s.checkProjectPassword(r, projectID) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()
	var req updateMetadataRequest
	if err := decodeRequestBody(w, r, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	resp, err := s.hub.updateMetadata(projectID, req)
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

func (s *server) handleFont(w http.ResponseWriter, r *http.Request) {
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
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()
		var req updateFontRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.updateFont(projectID, req)
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
		if !s.checkProjectPassword(r, projectID) {
			http.Error(w, "invalid password", http.StatusForbidden)
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()
		var req deleteFontRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, err := s.hub.deleteFont(projectID, req)
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
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

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

	if _, err := fmt.Fprintf(w, ": connected %d\n\n", time.Now().UnixNano()); err != nil {
		return
	}
	flusher.Flush()

	if exists {
		if err := sendEvent(projectEvent{
			Type:            "snapshot",
			projectDocument: doc,
		}); err != nil {
			return
		}
	}

	ticker := time.NewTicker(5 * time.Second)
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

func fileExistsFS(fsys fs.FS, path string) bool {
	if fsys == nil {
		return false
	}
	info, err := fs.Stat(fsys, path)
	return err == nil && !info.IsDir()
}

func embeddedUIAvailable() bool {
	return fileExistsFS(embeddedUI, "index.html")
}

func resolveUIFS(uiDir string) fs.FS {
	uiDir = strings.TrimSpace(uiDir)
	switch {
	case uiDir != "":
		return os.DirFS(uiDir)
	case embeddedUIAvailable():
		return embeddedUI
	default:
		return nil
	}
}

func runtimePublicEnv() map[string]string {
	values := map[string]string{
		"PUBLIC_CHIRONE_SYNC_API_BASE": defaultRuntimeSyncAPIBase,
	}
	for _, key := range []string{
		"PUBLIC_CHIRONE_SYNC_API_BASE",
		"PUBLIC_CHIRONE_ALLOW_SYNC_API_BASE_OVERRIDE",
		"PUBLIC_CHIRONE_SYNC_PROJECT",
	} {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	return values
}

func injectRuntimePublicEnv(indexHTML []byte) []byte {
	envJSON, err := json.Marshal(runtimePublicEnv())
	if err != nil {
		return indexHTML
	}

	script := []byte("<script>window.__CHIRONE_PUBLIC_ENV__=" + string(envJSON) + ";</script>")
	headClose := []byte("</head>")
	if index := strings.Index(string(indexHTML), string(headClose)); index >= 0 {
		output := make([]byte, 0, len(indexHTML)+len(script))
		output = append(output, indexHTML[:index]...)
		output = append(output, script...)
		output = append(output, indexHTML[index:]...)
		return output
	}

	output := make([]byte, 0, len(indexHTML)+len(script))
	output = append(output, script...)
	output = append(output, indexHTML...)
	return output
}

func (s *server) serveHTML(w http.ResponseWriter, r *http.Request, path string) {
	html, err := fs.ReadFile(s.uiFS, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write(injectRuntimePublicEnv(html))
}

func (s *server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.uiFS == nil {
		http.NotFound(w, r)
		return
	}

	cleanPath := pathpkg.Clean("/" + r.URL.Path)
	relativePath := strings.TrimPrefix(cleanPath, "/")
	if relativePath == "" || relativePath == "." {
		relativePath = "index.html"
	}

	serveIfExists := func(path string) bool {
		if !fileExistsFS(s.uiFS, path) {
			return false
		}
		if pathpkg.Ext(path) == ".html" {
			s.serveHTML(w, r, path)
			return true
		}
		http.ServeFileFS(w, r, s.uiFS, path)
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
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/project", s.handleProject)
	mux.HandleFunc("/api/projects", s.handleProjectList)
	mux.HandleFunc("/api/project/create", s.handleProjectCreate)
	mux.HandleFunc("/api/project/auth", s.handleProjectAuth)
	mux.HandleFunc("/api/project/password", s.handleProjectPassword)
	mux.HandleFunc("/api/project-version", s.handleProjectVersion)
	mux.HandleFunc("/api/revisions", s.handleRevisions)
	mux.HandleFunc("/api/revisions/revert", s.handleRevisionRevert)
	mux.HandleFunc("/api/glyph", s.handleGlyph)
	mux.HandleFunc("/api/font-glyph", s.handleFontGlyph)
	mux.HandleFunc("/api/font-glyphs", s.handleFontGlyphList)
	mux.HandleFunc("/api/syntax", s.handleSyntax)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/metadata", s.handleMetadata)
	mux.HandleFunc("/api/font", s.handleFont)
	mux.HandleFunc("/api/events", s.handleEvents)

	if s.uiFS != nil {
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
			_, _ = fmt.Fprintf(w, "chirone %s (%s)\n", s.appVersion, s.appSHA)
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
	loadEnvFile(".env")
	resolvedUIDir := strings.TrimSpace(uiDir)
	srv := &server{
		hub:         newHub(dataDir),
		allowOrigin: allowOrigin,
		uiDir:       resolvedUIDir,
		uiFS:        resolveUIFS(resolvedUIDir),
		appVersion:  version,
		appSHA:      resolveGitSHA(),
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
		log.Printf("chirone listening on %s (data dir: %s, ui dir override: %s)", addr, dataDir, srv.uiDir)
	} else if srv.uiFS != nil {
		log.Printf("chirone listening on %s (data dir: %s, ui: embedded)", addr, dataDir)
	} else {
		log.Printf("chirone listening on %s (data dir: %s, ui: unavailable)", addr, dataDir)
	}
	return httpServer.ListenAndServe()
}

func serveCommand(args []string) error {
	defaultAddr := ":8090"
	if envAddr := os.Getenv("CHIRONE_ADDR"); envAddr != "" {
		defaultAddr = envAddr
	}

	flags := flag.NewFlagSet("chirone", flag.ContinueOnError)
	flags.Usage = printUsage

	addr := flags.String("addr", defaultAddr, "address to listen on")
	defaultDataDir := "./data"
	if envDataDir := os.Getenv("CHIRONE_DATA_DIR"); envDataDir != "" {
		defaultDataDir = envDataDir
	}

	dataDir := flags.String("data-dir", defaultDataDir, "directory where project snapshots are stored")
	allowOrigin := flags.String("allow-origin", "*", "CORS allowed origin (or * for all)")
	uiDir := flags.String("ui-dir", "", "optional directory to serve static UI files from instead of embedded assets")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	ctx := context.Background()
	if err := run(ctx, *addr, *dataDir, *allowOrigin, *uiDir); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runCLI(args []string) error {
	switch {
	case len(args) == 0:
		return serveCommand(nil)
	case args[0] == "help" || args[0] == "-h" || args[0] == "--help":
		printUsage()
		return nil
	case args[0] == "version" || args[0] == "--version":
		fmt.Println(version)
		return nil
	case args[0] == "serve":
		return serveCommand(args[1:])
	default:
		return serveCommand(args)
	}
}

func printUsage() {
	fmt.Print(`chirone serves the embedded Chirone web app and collaboration API.

Usage:
  chirone
  chirone serve [flags]
  chirone version

Flags:
  --addr string
        address to listen on (default ":8090")
  --data-dir string
        directory where project snapshots are stored (default "./data")
  --allow-origin string
        CORS allowed origin (or * for all) (default "*")
  --ui-dir string
        optional directory to serve static UI files from instead of embedded assets
`)
}

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
