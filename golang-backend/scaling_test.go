package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// Compare the new SQL read path with the legacy projected-cache handler at the
// same dataset size. Run: go test -run '^$' -bench BenchmarkRecordReads -benchmem
func BenchmarkRecordReads(b *testing.B) {
	for _, mode := range []string{"sql", "legacy"} {
		b.Run(mode, func(b *testing.B) {
			jwtSecret = []byte("benchmark-only-secret")
			b.Setenv("LEGACY_DATA_FILE", "")
			if err := loadStore(filepath.Join(b.TempDir(), "crm.sqlite3")); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { database.Close(); database = nil })
			u, _ := findUserByEmail("demo@northwind.dev")
			scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
			mu.Lock()
			for i := 0; i < 5000; i++ {
				contacts = append(contacts, Contact{RecordScope: scope, ID: newID("benchmark"), Name: "Benchmark client", Stage: "Lead"})
			}
			id := contacts[len(contacts)-1].ID
			mu.Unlock()
			if err := saveStore(); err != nil {
				b.Fatal(err)
			}
			token, err := issueToken(u)
			if err != nil {
				b.Fatal(err)
			}
			m := http.NewServeMux()
			registerExpansionRoutes(m)
			m.HandleFunc("GET /api/legacy-contacts/{id}", authMiddleware(getContact))
			h := persistMutations(m)
			path := "/api/legacy-contacts/" + id
			if mode == "sql" {
				path = "/api/records/contacts/" + id
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					r := httptest.NewRequest("GET", path, nil)
					r.Header.Set("Authorization", "Bearer "+token)
					w := httptest.NewRecorder()
					h.ServeHTTP(w, r)
					if w.Code != 200 {
						b.Errorf("read failed: %d", w.Code)
					}
				}
			})
		})
	}
}

// Exercise scoped list paging, daily work and a real validated write against
// 5,000 clients/applications plus another workspace. This is a local baseline,
// not a declaration of production throughput or multi-process safety.
func BenchmarkRepositoryWorkload(b *testing.B) {
	for _, mode := range []string{"paged-list", "daily-work", "validated-write"} {
		b.Run(mode, func(b *testing.B) {
			jwtSecret = []byte("benchmark-only-secret")
			b.Setenv("LEGACY_DATA_FILE", "")
			if err := loadStore(filepath.Join(b.TempDir(), "crm.sqlite3")); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { database.Close(); database = nil })
			u, _ := findUserByEmail("demo@northwind.dev")
			scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
			foreign := scope
			foreign.OrgID = "benchmark-other-org"
			mu.Lock()
			for i := 0; i < 5000; i++ {
				s := scope
				if i%5 == 0 {
					s = foreign
				}
				id := fmt.Sprintf("workload-%05d", i)
				contacts = append(contacts, Contact{RecordScope: s, ID: id, Name: "Workload client", Email: fmt.Sprintf("client-%d@example.com", i), Stage: "Lead"})
				cases = append(cases, Case{RecordScope: s, ID: "case-" + id, ClientID: id, CaseNumber: "APP-" + id, AssignedOfficer: u.Name, CurrentStage: "Documents Pending", NextAction: "Review passport", NextDeadline: "2026-10-15"})
				documents = append(documents, Document{RecordScope: s, ID: "doc-" + id, CaseID: "case-" + id, DocName: "Passport", Required: true, Status: "Requested"})
			}
			mu.Unlock()
			if err := saveStore(); err != nil {
				b.Fatal(err)
			}
			token, _ := issueToken(u)
			h := expansionTestMux()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				method, path, body := "GET", "/api/contacts?limit=25&offset=500&sort=name", ""
				if mode == "daily-work" {
					path = "/api/my-work?limit=25"
				}
				if mode == "validated-write" {
					method = "PUT"
					path = "/api/contacts/workload-00001"
					body = fmt.Sprintf(`{"name":"Updated client %d","email":"client-1@example.com","stage":"Lead"}`, i)
				}
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != 200 {
					b.Fatalf("%s failed: %d %s", mode, w.Code, w.Body.String())
				}
			}
		})
	}
}
