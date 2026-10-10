package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupStore(t *testing.T) {
	t.Helper()
	jwtSecret = []byte("test-secret")
	t.Setenv("LEGACY_DATA_FILE", "")
	t.Cleanup(func() {
		if database != nil {
			database.Close()
			database = nil
			fixtureBaseline = diskStore{}
		}
	})
	if err := loadStore(filepath.Join(t.TempDir(), "crm.sqlite3")); err != nil {
		t.Fatal(err)
	}
	fixtureBaseline = cloneStore()
}

func TestPersistenceRoundTrip(t *testing.T) {
	setupStore(t)
	user, _ := findUserByEmail("demo@northwind.dev")
	user.SessionVersion = 3
	mu.Lock()
	users[user.Email] = user
	contacts[0].Name = "Persisted contact"
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	contacts = nil
	users = nil
	mu.Unlock()
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	restored, _ := findUserByEmail(user.Email)
	if restored.Password != user.Password || restored.SessionVersion != 3 || contacts[0].Name != "Persisted contact" {
		t.Fatal("store did not preserve accounts and records")
	}
	info, _ := os.Stat(dataPath)
	if info.Mode().Perm() != 0600 {
		t.Fatal("store must be private")
	}
}

func TestCorruptStoreIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crm.sqlite3")
	os.WriteFile(path, []byte("invalid"), 0600)
	if loadStore(path) == nil {
		t.Fatal("expected corrupt data error")
	}
	payload, _ := os.ReadFile(path)
	if string(payload) != "invalid" {
		t.Fatal("corrupt store was overwritten")
	}
}

func TestReportsUseAllRecords(t *testing.T) {
	setupStore(t)
	response := httptest.NewRecorder()
	user, _ := findUserByEmail("demo@northwind.dev")
	request := aiUserRequest(context.Background(), user)
	request.URL = &url.URL{Path: "/api/reports"}
	handleReports(response, request)
	var report reportSummary
	json.Unmarshal(response.Body.Bytes(), &report)
	if report.Contacts != len(contacts) || report.OpenTasks != 5 || report.OutstandingBalance != 2235 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestLogoutAllInvalidatesOtherSessionsAndSurvivesRestart(t *testing.T) {
	setupStore(t)
	user, _ := findUserByEmail("demo@northwind.dev")
	first, _ := issueToken(user)
	second, _ := issueToken(user)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/me/logout-all", authMiddleware(handleLogoutAll))
	mux.HandleFunc("GET /api/me", authMiddleware(handleMe))
	handler := persistMutations(mux)
	request := httptest.NewRequest("POST", "/api/me/logout-all", nil)
	request.Header.Set("Authorization", "Bearer "+first)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first, second} {
		request = httptest.NewRequest("GET", "/api/me", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 401 {
			t.Fatal("old session still valid")
		}
	}
}

func TestMutationPersistenceAndRollback(t *testing.T) {
	setupStore(t)
	user, _ := findUserByEmail("demo@northwind.dev")
	token, _ := issueToken(user)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/contacts", authMiddleware(createContact))
	handler := persistMutations(mux)
	email := "new@example.com"
	create := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/api/contacts", strings.NewReader(`{"name":"New contact","email":"`+email+`","stage":"Lead"}`))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	count := len(contacts)
	response := create()
	if response.Code < 200 || response.Code >= 300 {
		t.Fatal(response.Body.String())
	}
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	if len(contacts) != count+1 {
		t.Fatal("mutation was not saved")
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	email = "another@example.com"
	response = create()
	if response.Code != 503 || len(contacts) != count+1 {
		t.Fatal("failed write must rollback")
	}
}

func TestStaticRoutesDoNotExposeStore(t *testing.T) {
	mux := http.NewServeMux()
	mountStatic(mux)
	for _, path := range []string{"/data/crm.json", "/elm.json", "/roadmap.txt", "/golang-backend/go.mod"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 404 {
			t.Fatalf("exposed %s", path)
		}
	}
	manifest := httptest.NewRecorder()
	mux.ServeHTTP(manifest, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	if manifest.Code != http.StatusOK || !strings.Contains(manifest.Header().Get("Content-Type"), "manifest+json") || !json.Valid(manifest.Body.Bytes()) {
		t.Fatalf("PWA manifest route failed: status=%d type=%q", manifest.Code, manifest.Header().Get("Content-Type"))
	}
	worker := httptest.NewRecorder()
	mux.ServeHTTP(worker, httptest.NewRequest("GET", "/sw.js", nil))
	if worker.Code != http.StatusOK || !strings.Contains(worker.Header().Get("Content-Type"), "javascript") || !strings.Contains(worker.Body.String(), "url.pathname.startsWith('/api/')") {
		t.Fatalf("root-scoped PWA worker route failed or does not bypass API responses: status=%d", worker.Code)
	}
	for _, path := range []string{"/elm.js", "/public/styles.css", "/public/mobile.js", "/public/pwa-sw.js", "/public/icons/ecc-192.png", "/public/icons/ecc-512.png"} {
		asset := httptest.NewRecorder()
		mux.ServeHTTP(asset, httptest.NewRequest("GET", path, nil))
		if asset.Code != http.StatusOK {
			t.Fatalf("PWA shell asset %s returned %d", path, asset.Code)
		}
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", "/contacts/c_1", nil))
	if response.Code != 200 {
		t.Fatal("deep links must serve app")
	}
}
