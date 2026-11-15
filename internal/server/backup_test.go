package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "bothub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open store: %v", err)
	}

	s := &Server{
		store:   st,
		agent:   agent.NewEngine(st, nil),
		version: "v1.6.0",
		commit:  "test-commit",
	}

	cleanup := func() {
		_ = st.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return s, cleanup
}

func TestBackupExportAndImport(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Seed a test asset and task
	err := s.store.SaveAsset(store.Asset{
		ID:        "test_asset_vps",
		Name:      "Test VPS Machine",
		Category:  "compute",
		Spec:      "2C4G",
		Price:     "$10.00/mo",
		DueDate:   "2026-12-31T23:59:59+08:00",
		Status:    "running",
		AutoRenew: true,
		WarnDays:  "[7, 3, 1]",
	})
	if err != nil {
		t.Fatalf("failed to save test asset: %v", err)
	}

	// 2. Test Export
	reqExport := httptest.NewRequest(http.MethodGet, "/api/system/export", nil)
	recExport := httptest.NewRecorder()
	s.handleExportBackup(recExport, reqExport)

	if recExport.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from export, got %d", recExport.Code)
	}

	var exported BackupData
	if err := json.Unmarshal(recExport.Body.Bytes(), &exported); err != nil {
		t.Fatalf("failed to unmarshal exported backup: %v", err)
	}

	if exported.Version != "v1.6.0" {
		t.Errorf("expected version v1.6.0, got %s", exported.Version)
	}

	foundAsset := false
	for _, a := range exported.Assets {
		if a.ID == "test_asset_vps" {
			foundAsset = true
			break
		}
	}
	if !foundAsset {
		t.Errorf("expected test_asset_vps in exported assets")
	}

	// 3. Test Import with modified data
	newAsset := store.Asset{
		ID:        "restored_asset_domain",
		Name:      "Restored Domain",
		Category:  "domain",
		Spec:      "Cloudflare",
		Price:     "$9.00/yr",
		DueDate:   "2027-01-01T00:00:00+08:00",
		Status:    "active",
		AutoRenew: true,
		WarnDays:  "[30, 7, 1]",
	}
	exported.Assets = append(exported.Assets, newAsset)

	importBytes, _ := json.Marshal(exported)
	reqImport := httptest.NewRequest(http.MethodPost, "/api/system/import", bytes.NewReader(importBytes))
	recImport := httptest.NewRecorder()
	s.handleImportBackup(recImport, reqImport)

	if recImport.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from import, got %d", recImport.Code)
	}

	// Verify imported asset exists in store
	importedAsset, err := s.store.GetAsset("restored_asset_domain")
	if err != nil || importedAsset == nil {
		t.Fatalf("failed to retrieve restored asset: %v", err)
	}
	if importedAsset.Name != "Restored Domain" {
		t.Errorf("unexpected name: %s", importedAsset.Name)
	}
}

func TestAgentMemoryAndSessionHandlers(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	body := []byte(`{"category":"infra","fact_key":"dns_provider","content":"Cloudflare Zero Trust"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent/memories", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleSaveAgentMemory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK saving memory, got %d: %s", rec.Code, rec.Body.String())
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/agent/memories?q=Cloudflare", nil)
	recList := httptest.NewRecorder()
	s.handleListAgentMemories(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing memories, got %d", recList.Code)
	}

	reqSess := httptest.NewRequest(http.MethodGet, "/api/agent/sessions", nil)
	recSess := httptest.NewRecorder()
	s.handleListAgentSessions(recSess, reqSess)
	if recSess.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing sessions, got %d", recSess.Code)
	}
}
