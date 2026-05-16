package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestHub(t *testing.T) *hub {
	t.Helper()
	dir, err := os.MkdirTemp("", "chirone-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return newHub(dir)
}

func TestUpdateMetadata_CreatesAndUpdates(t *testing.T) {
	h := newTestHub(t)
	projectID := "testproj"

	// First update (create) — baseVersion 0
	meta := json.RawMessage(`{"familyName":"Test Font","designer":"Puria"}`)
	resp, err := h.updateMetadata(projectID, updateMetadataRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(0),
		Metadata:    meta,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Entity != "metadata" {
		t.Errorf("expected entity 'metadata', got %q", resp.Entity)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}

	// Second update — baseVersion must match
	meta2 := json.RawMessage(`{"familyName":"Updated","designer":"Puria"}`)
	resp2, err := h.updateMetadata(projectID, updateMetadataRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(1),
		Metadata:    meta2,
	})
	if err != nil {
		t.Fatalf("unexpected error on second update: %v", err)
	}
	if resp2.Version != 2 {
		t.Errorf("expected version 2, got %d", resp2.Version)
	}

	// Verify persistence by loading from disk
	doc, exists, err := h.getProject(projectID)
	if err != nil {
		t.Fatalf("getProject failed: %v", err)
	}
	if !exists {
		t.Fatal("project should exist after metadata updates")
	}
	var stored map[string]any
	if err := json.Unmarshal(doc.Metadata, &stored); err != nil {
		t.Fatalf("stored metadata is not valid JSON: %v", err)
	}
	if stored["familyName"] != "Updated" {
		t.Errorf("expected familyName 'Updated', got %v", stored["familyName"])
	}
}

func TestUpdateMetadata_VersionConflict(t *testing.T) {
	h := newTestHub(t)
	projectID := "testproj"

	meta := json.RawMessage(`{"familyName":"Test"}`)
	_, err := h.updateMetadata(projectID, updateMetadataRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(0),
		Metadata:    meta,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try with stale baseVersion
	_, err = h.updateMetadata(projectID, updateMetadataRequest{
		ClientID:    "client-2",
		BaseVersion: int64Ptr(0),
		Metadata:    meta,
	})
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	var conflictErr *entityConflictError
	if !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected conflict error, got: %v", err)
	}
	_ = conflictErr // silence unused
}

func TestUpdateMetadata_MissingBaseVersion(t *testing.T) {
	h := newTestHub(t)
	meta := json.RawMessage(`{"familyName":"Test"}`)
	_, err := h.updateMetadata("testproj", updateMetadataRequest{
		ClientID: "client-1",
		Metadata: meta,
	})
	if err == nil {
		t.Fatal("expected error for missing baseVersion")
	}
	if !strings.Contains(err.Error(), "missing baseVersion") {
		t.Errorf("expected 'missing baseVersion' error, got: %v", err)
	}
}

func TestUpdateFont_Upsert(t *testing.T) {
	h := newTestHub(t)
	projectID := "testproj"

	font := json.RawMessage(`{"id":"font-1","name":"Regular","syntaxId":"s1","metricsId":"m1","metadataId":"d1","outputName":"Test-Regular.otf","enabled":true}`)
	resp, err := h.updateFont(projectID, updateFontRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(0),
		Font:        font,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Entity != "font" {
		t.Errorf("expected entity 'font', got %q", resp.Entity)
	}
	if resp.EntityID != "font-1" {
		t.Errorf("expected entityID 'font-1', got %q", resp.EntityID)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}

	// Update existing font
	font2 := json.RawMessage(`{"id":"font-1","name":"Bold","syntaxId":"s1","metricsId":"m1","metadataId":"d1","outputName":"Test-Bold.otf","enabled":true}`)
	resp2, err := h.updateFont(projectID, updateFontRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(1),
		Font:        font2,
	})
	if err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}
	if resp2.Version != 2 {
		t.Errorf("expected version 2, got %d", resp2.Version)
	}

	// Verify stored (fonts serialized as array in project document)
	doc, exists, err := h.getProject(projectID)
	if err != nil {
		t.Fatalf("getProject failed: %v", err)
	}
	if !exists {
		t.Fatal("project should exist")
	}
	var stored []map[string]any
	if err := json.Unmarshal(doc.Fonts, &stored); err != nil {
		t.Fatalf("stored fonts not valid JSON array: %v", err)
	}
	var found bool
	for _, f := range stored {
		if f["id"] == "font-1" {
			if f["name"] != "Bold" {
				t.Errorf("expected name 'Bold', got %v", f["name"])
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("font-1 not found in stored fonts")
	}
}

func TestDeleteFont(t *testing.T) {
	h := newTestHub(t)
	projectID := "testproj"

	// Create a font first
	font := json.RawMessage(`{"id":"font-del","name":"Regular","syntaxId":"s1","metricsId":"m1","metadataId":"d1","outputName":"Test.otf","enabled":true}`)
	_, err := h.updateFont(projectID, updateFontRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(0),
		Font:        font,
	})
	if err != nil {
		t.Fatalf("unexpected error creating font: %v", err)
	}

	// Delete it
	resp, err := h.deleteFont(projectID, deleteFontRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(1),
		ID:          "font-del",
	})
	if err != nil {
		t.Fatalf("unexpected error deleting font: %v", err)
	}
	if !resp.Deleted {
		t.Error("expected Deleted=true")
	}

	// Verify gone
	doc, _, _ := h.getProject(projectID)
	var stored map[string]any
	json.Unmarshal(doc.Fonts, &stored)
	if _, ok := stored["font-del"]; ok {
		t.Error("font-del should be deleted")
	}
}

func TestDeleteFont_NotFound(t *testing.T) {
	h := newTestHub(t)
	_, err := h.deleteFont("testproj", deleteFontRequest{
		ClientID:    "client-1",
		BaseVersion: int64Ptr(0),
		ID:          "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent font")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestHandleMetadata_Put(t *testing.T) {
	h := newTestHub(t)
	s := &server{hub: h}

	body := strings.NewReader(`{"clientId":"c1","baseVersion":0,"metadata":{"familyName":"Via API"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/metadata?project=testproj", body)
	// project has no password, checkProjectPassword passes
	rec := httptest.NewRecorder()

	s.handleMetadata(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp entityUpdateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Entity != "metadata" {
		t.Errorf("expected entity 'metadata', got %q", resp.Entity)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
}

func TestHandleFont_PutAndDelete(t *testing.T) {
	h := newTestHub(t)
	s := &server{hub: h}
	project := "testproj"

	// PUT
	body := strings.NewReader(`{"clientId":"c1","baseVersion":0,"font":{"id":"f1","name":"Regular","syntaxId":"s1","metricsId":"m1","metadataId":"d1","outputName":"R.otf","enabled":true}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/font?project="+project, body)
	// project has no password, checkProjectPassword passes
	rec := httptest.NewRecorder()
	s.handleFont(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// DELETE
	body2 := strings.NewReader(`{"clientId":"c1","baseVersion":1,"id":"f1"}`)
	req2 := httptest.NewRequest(http.MethodDelete, "/api/font?project="+project, body2)
	req2.Header.Set("X-Chirone-Admin-Password", "test")
	rec2 := httptest.NewRecorder()
	s.handleFont(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("DELETE expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp entityUpdateResponse
	json.NewDecoder(rec2.Body).Decode(&resp)
	if !resp.Deleted {
		t.Error("expected Deleted=true in response")
	}
}

func TestHandleFont_MethodNotAllowed(t *testing.T) {
	h := newTestHub(t)
	s := &server{hub: h}

	req := httptest.NewRequest(http.MethodGet, "/api/font?project=testproj", nil)
	rec := httptest.NewRecorder()
	s.handleFont(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
