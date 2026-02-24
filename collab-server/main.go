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
	"path/filepath"
	"regexp"
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
	ClientID string `json:"clientId"`
	projectSnapshot
}

type projectEvent struct {
	Type     string `json:"type"`
	ClientID string `json:"clientId,omitempty"`
	projectDocument
}

type projectState struct {
	Doc  projectDocument
	Subs map[chan projectEvent]struct{}
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

func (h *hub) getProject(projectID string) (projectDocument, bool, error) {
	projectID = sanitizeProjectID(projectID)

	h.mu.RLock()
	if state, ok := h.projects[projectID]; ok {
		doc := state.Doc
		h.mu.RUnlock()
		return doc, true, nil
	}
	h.mu.RUnlock()

	doc, err := h.loadProjectFromDisk(projectID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return projectDocument{}, false, nil
		}
		return projectDocument{}, false, err
	}

	h.mu.Lock()
	state, ok := h.projects[projectID]
	if !ok {
		state = &projectState{
			Doc:  *doc,
			Subs: map[chan projectEvent]struct{}{},
		}
		h.projects[projectID] = state
	}
	out := state.Doc
	h.mu.Unlock()

	return out, true, nil
}

func (h *hub) subscribe(projectID string, out chan projectEvent) (projectDocument, bool, error) {
	projectID = sanitizeProjectID(projectID)
	doc, exists, err := h.getProject(projectID)
	if err != nil {
		return projectDocument{}, false, err
	}

	h.mu.Lock()
	state, ok := h.projects[projectID]
	if !ok {
		state = &projectState{
			Doc: projectDocument{
				Project: sanitizeProjectID(projectID),
			},
			Subs: map[chan projectEvent]struct{}{},
		}
		h.projects[projectID] = state
	}
	state.Subs[out] = struct{}{}
	h.mu.Unlock()

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

	var (
		doc      projectDocument
		channels []chan projectEvent
	)

	h.mu.Lock()
	state, ok := h.projects[projectID]
	if !ok {
		loaded, err := h.loadProjectFromDisk(projectID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			h.mu.Unlock()
			return projectDocument{}, err
		}
		if loaded != nil {
			state = &projectState{
				Doc:  *loaded,
				Subs: map[chan projectEvent]struct{}{},
			}
		} else {
			state = &projectState{
				Doc: projectDocument{
					Project: sanitizeProjectID(projectID),
					Version: 0,
				},
				Subs: map[chan projectEvent]struct{}{},
			}
		}
		h.projects[projectID] = state
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	state.Doc.Project = projectID
	state.Doc.Version++
	state.Doc.UpdatedAt = now
	state.Doc.projectSnapshot = snapshot
	doc = state.Doc

	channels = make([]chan projectEvent, 0, len(state.Subs))
	for ch := range state.Subs {
		channels = append(channels, ch)
	}
	h.mu.Unlock()

	if err := h.saveProjectToDisk(doc); err != nil {
		return projectDocument{}, err
	}

	event := projectEvent{
		Type:            "snapshot",
		ClientID:        req.ClientID,
		projectDocument: doc,
	}

	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			// Slow consumers are skipped; latest snapshot always wins.
		}
	}

	return doc, nil
}

type server struct {
	hub         *hub
	allowOrigin string
}

func (s *server) writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if s.allowOrigin == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if origin == s.allowOrigin {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,OPTIONS")
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
		doc, ok, err := s.hub.getProject(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	case http.MethodPut:
		defer r.Body.Close()

		var req updateProjectRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		doc, err := s.hub.updateProject(projectID, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
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

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
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
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/project", s.handleProject)
	mux.HandleFunc("/api/events", s.handleEvents)
	return requestLogger(mux)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func run(ctx context.Context, addr, dataDir, allowOrigin string) error {
	srv := &server{
		hub:         newHub(dataDir),
		allowOrigin: allowOrigin,
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

	log.Printf("collab server listening on %s (data dir: %s)", addr, dataDir)
	return httpServer.ListenAndServe()
}

func main() {
	addr := flag.String("addr", ":8090", "address to listen on")
	dataDir := flag.String("data-dir", "./data", "directory where project snapshots are stored")
	allowOrigin := flag.String("allow-origin", "*", "CORS allowed origin (or * for all)")
	flag.Parse()

	ctx := context.Background()
	if err := run(ctx, *addr, *dataDir, *allowOrigin); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
