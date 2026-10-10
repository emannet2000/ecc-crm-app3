package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

type RecordScope struct {
	OrgID      string `json:"orgId"`
	OwnerID    string `json:"ownerId"`
	TeamID     string `json:"teamId"`
	Visibility string `json:"visibility"`
}

type DBRunner interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

const relatedScopeKey contextKey = "relatedScope"

const requestTxKey contextKey = "transaction"
const requestUserKey contextKey = "user"
const requestMetaKey contextKey = "requestMeta"

type requestMeta struct {
	OwnershipTransfers map[string]bool
	SAMLRedirect       string
	Actor              string
	OrgID              string
	PersistFailure     bool
	Rollback           []func()
	AfterCommit        []func()
}

func storeDB(r *http.Request) DBRunner {
	if tx, ok := r.Context().Value(requestTxKey).(*sql.Tx); ok {
		return tx
	}
	return database
}
func currentUser(r *http.Request) User {
	user, _ := r.Context().Value(requestUserKey).(User)
	return user
}
func metadata(r *http.Request) *requestMeta {
	meta, _ := r.Context().Value(requestMetaKey).(*requestMeta)
	return meta
}
func validRole(role string) bool {
	return role == "admin" || role == "manager" || role == "member" || role == "viewer"
}
func isAdmin(user User) bool   { return user.Role == "admin" }
func isManager(user User) bool { return user.Role == "admin" || user.Role == "manager" }
func canRead(scope RecordScope, user User) bool {
	if scope.OrgID != user.OrgID || user.OrgID == "" {
		return false
	}
	return isManager(user) || scope.OwnerID == user.ID || scope.Visibility == "organization" || (scope.Visibility == "team" && scope.TeamID != "" && scope.TeamID == user.TeamID)
}
func canWrite(scope RecordScope, user User) bool {
	return scope.OrgID == user.OrgID && user.OrgID != "" && (isManager(user) || (user.Role == "member" && scope.OwnerID == user.ID))
}
func scopeOf[T any](item T) RecordScope {
	value := reflect.ValueOf(item).FieldByName("RecordScope")
	if value.IsValid() {
		return value.Interface().(RecordScope)
	}
	return RecordScope{}
}
func idOf[T any](item T) string { return reflect.ValueOf(item).FieldByName("ID").String() }
func withScope[T any](item T, scope RecordScope) T {
	reflect.ValueOf(&item).Elem().FieldByName("RecordScope").Set(reflect.ValueOf(scope))
	return item
}
func visibleRecords[T any](items []T, user User) []T {
	out := []T{}
	for _, item := range items {
		if canRead(scopeOf(item), user) {
			out = append(out, item)
		}
	}
	return out
}
func mergeRecords[T any](original, before, after []T, user User) []T {
	visible := map[string]bool{}
	updated := map[string]T{}
	for _, item := range before {
		visible[idOf(item)] = true
	}
	for _, item := range after {
		scope := scopeOf(item)
		if !visible[idOf(item)] && scope.OrgID != user.OrgID {
			scope = RecordScope{OrgID: user.OrgID, OwnerID: user.ID, TeamID: user.TeamID, Visibility: "organization"}
			item = withScope(item, scope)
		}
		updated[idOf(item)] = item
	}
	out := []T{}
	for _, item := range original {
		id := idOf(item)
		if !visible[id] {
			out = append(out, item)
			continue
		}
		if update, ok := updated[id]; ok {
			out = append(out, update)
			delete(updated, id)
		}
	}
	for _, item := range after {
		if _, ok := updated[idOf(item)]; ok {
			out = append(out, item)
			delete(updated, idOf(item))
		}
	}
	return out
}
func projectedStore(saved diskStore, user User) diskStore {
	saved.Contacts = visibleRecords(saved.Contacts, user)
	saved.Deals = visibleRecords(saved.Deals, user)
	saved.Activities = visibleRecords(saved.Activities, user)
	saved.Tasks = visibleRecords(saved.Tasks, user)
	saved.Schools = visibleRecords(saved.Schools, user)
	saved.Students = visibleRecords(saved.Students, user)
	saved.Agents = visibleRecords(saved.Agents, user)
	saved.Leads = visibleRecords(saved.Leads, user)
	saved.Cases = visibleRecords(saved.Cases, user)
	saved.Documents = visibleRecords(saved.Documents, user)
	saved.Invoices = visibleRecords(saved.Invoices, user)
	saved.Payments = visibleRecords(saved.Payments, user)
	saved.Partners = visibleRecords(saved.Partners, user)
	// A related record cannot reveal a parent hidden from the same user.
	allowed := map[string]bool{}
	for _, items := range recordsOf(saved) {
		for id := range items {
			allowed[id] = true
		}
	}
	ds := []Document{}
	for _, d := range saved.Documents {
		if allowed[d.CaseID] {
			ds = append(ds, d)
		}
	}
	saved.Documents = ds
	ps := []Payment{}
	for _, p := range saved.Payments {
		if allowed[p.InvoiceID] {
			ps = append(ps, p)
		}
	}
	saved.Payments = ps
	as := []Activity{}
	for _, a := range saved.Activities {
		if (a.ContactID == "" || allowed[a.ContactID]) && (a.DealID == "" || allowed[a.DealID]) {
			as = append(as, a)
		}
	}
	saved.Activities = as
	return saved
}
func mergeStores(original, before, after diskStore, user User) diskStore {
	after.Contacts = mergeRecords(original.Contacts, before.Contacts, after.Contacts, user)
	after.Deals = mergeRecords(original.Deals, before.Deals, after.Deals, user)
	after.Activities = mergeRecords(original.Activities, before.Activities, after.Activities, user)
	after.Tasks = mergeRecords(original.Tasks, before.Tasks, after.Tasks, user)
	after.Schools = mergeRecords(original.Schools, before.Schools, after.Schools, user)
	after.Students = mergeRecords(original.Students, before.Students, after.Students, user)
	after.Agents = mergeRecords(original.Agents, before.Agents, after.Agents, user)
	after.Leads = mergeRecords(original.Leads, before.Leads, after.Leads, user)
	after.Cases = mergeRecords(original.Cases, before.Cases, after.Cases, user)
	after.Documents = mergeRecords(original.Documents, before.Documents, after.Documents, user)
	after.Invoices = mergeRecords(original.Invoices, before.Invoices, after.Invoices, user)
	after.Payments = mergeRecords(original.Payments, before.Payments, after.Payments, user)
	after.Partners = mergeRecords(original.Partners, before.Partners, after.Partners, user)
	return after
}
func (state *runtimeStore) cloneStore() diskStore {
	return cloneStoreValue(reflect.ValueOf(state.snapshot())).Interface().(diskStore)
}

func cloneStore() diskStore {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.cloneStore()
}
func recordOrg(payload string) string {
	var scope RecordScope
	json.Unmarshal([]byte(payload), &scope)
	if scope.OrgID == "" {
		return "org_default"
	}
	return scope.OrgID
}

// Every API request is serialized by persistMutations. Existing CRM handlers
// receive a private projection; only that projection's changes can be merged.
func scopedHandler(next http.HandlerFunc, w http.ResponseWriter, r *http.Request, user User) {
	if independentRead(r) {
		if entity, id, ok := repositoryRoute(r); ok {
			handleRepositoryRead(w, r, entity, id)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/photo") || strings.HasSuffix(r.URL.Path, "/qr") {
			record, err := sqlRecord(r, r.PathValue("entity"), r.PathValue("id"))
			if err != nil {
				writeError(w, 404, "Record not found")
				return
			}
			var saved diskStore
			if err = replaceSavedRecord(&saved, r.PathValue("entity"), r.PathValue("id"), mustJSON(record)); err != nil {
				writeError(w, 500, "Could not load record")
				return
			}
			r, _ = attachRuntime(r, saved)
		}

		next(w, r)
		return
	}
	nested, _ := r.Context().Value(contextKey("scopedRequest")).(bool)
	r = r.WithContext(context.WithValue(r.Context(), contextKey("scopedRequest"), true))
	if !nested && targetedContactWrite(next, w, r, user) {
		return
	}
	state, release := runtimeForCall(r)
	defer release()
	identity := state.snapshot()
	orgUser := user
	orgUser.Role = "admin"
	orgRequest := r.WithContext(context.WithValue(r.Context(), requestUserKey, orgUser))
	original, err := sqlWorkspace(orgRequest)
	if err != nil {
		writeError(w, 503, "Could not load committed workspace records")
		return
	}
	original.Users, original.Revoked = identity.Users, identity.Revoked
	state.restoreStore(original)
	state.before = state.cloneStore()
	original = state.cloneStore()
	projected := projectedStore(original, user)
	if r.Method == "GET" && r.URL.Query().Get("filterStatus") != "" {
		filterProjectedStatus(&projected, r)
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		accountAction := strings.HasPrefix(r.URL.Path, "/api/me/") || r.URL.Path == "/api/me" || strings.HasPrefix(r.URL.Path, "/api/notifications") || strings.HasPrefix(r.URL.Path, "/api/preferences") || strings.HasPrefix(r.URL.Path, "/api/views") || strings.HasPrefix(r.URL.Path, "/api/push/")
		if user.Role == "viewer" && !accountAction {
			writeError(w, 403, "Your role has read-only access")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		id := r.PathValue("id")
		if len(parts) > 2 && id != "" {
			for _, table := range entityTables {
				if parts[1] == table {
					items := recordsOf(original)[table]
					payload, exists := items[id]
					if !exists {
						writeError(w, 404, "Record not found")
						return
					}
					var scope RecordScope
					json.Unmarshal([]byte(payload), &scope)
					if !canRead(scope, user) {
						writeError(w, 404, "Record not found")
						return
					}
					if !canWrite(scope, user) {
						writeError(w, 403, "You can only edit records you own")
						return
					}
				}
			}
		}
	}
	r = r.WithContext(context.WithValue(r.Context(), relatedScopeKey, scopeForRelatedRequest(r, projected)))
	if !normalizeRequest(w, r) {
		return
	}
	if !checkRecordVersion(w, r, projected) {
		return
	}
	state.restoreStore(projected)
	state.mu.Lock()
	visiblePayments := map[string]bool{}
	for _, p := range projected.Payments {
		visiblePayments[p.ID] = true
	}
	state.hiddenLedgerPayments = nil
	for _, p := range original.Payments {
		if !visiblePayments[p.ID] && p.OrgID == user.OrgID {
			state.hiddenLedgerPayments = append(state.hiddenLedgerPayments, p)
		}
	}
	state.mu.Unlock()
	defer func() { state.mu.Lock(); state.hiddenLedgerPayments = nil; state.mu.Unlock() }()
	response := &bufferedResponse{header: make(http.Header)}
	next(response, r)
	if response.status == 0 {
		response.status = 200
	}

	after := state.snapshot()
	if r.Method != "GET" && r.Method != "HEAD" && response.status >= 200 && response.status < 300 {
		changes := changesBetween(projected, after, user.Email)
		beforeRecords := recordsOf(projected)
		afterRecords := recordsOf(after)
		for _, change := range changes {
			if change.Entity == "users" {
				continue
			}
			if payload, exists := beforeRecords[change.Entity][change.RecordID]; exists {
				var scope RecordScope
				json.Unmarshal([]byte(payload), &scope)
				if !canWrite(scope, user) && !(metadata(r) != nil && metadata(r).OwnershipTransfers[change.Entity+":"+change.RecordID]) && !computedOnly(change.Entity, payload, afterRecords[change.Entity][change.RecordID]) {
					state.restoreStore(original)
					writeError(w, 403, "A related record is outside your edit permissions")
					return
				}
			}
		}
		merged := mergeStores(original, projected, after, user)
		if danglingDeletedRelations(original, merged) {
			state.restoreStore(original)
			writeError(w, 409, "Reassign related records before deleting or merging this record")
			return
		}
		state.restoreStore(merged)
		state.mu.Lock()
		state.hiddenLedgerPayments = nil
		state.mu.Unlock()
		state.applyComputedData()
	} else {
		state.restoreStore(original)
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	w.WriteHeader(response.status)
	w.Write(response.body.Bytes())
}

func scopeSQL(r *http.Request, column string) (string, []any) {
	user := currentUser(r)
	return `json_extract(` + column + `,'$.orgId')=? AND (? IN ('admin','manager') OR json_extract(` + column + `,'$.ownerId')=? OR json_extract(` + column + `,'$.visibility')='organization' OR (json_extract(` + column + `,'$.visibility')='team' AND json_extract(` + column + `,'$.teamId')=? AND ?<>''))`, []any{user.OrgID, user.Role, user.ID, user.TeamID, user.TeamID}
}

func normalizeWorkspaceData() {
	mu.Lock()
	defer mu.Unlock()
	keys := []string{}
	for key := range users {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	adminID := "u_1"
	if _, ok := users["demo@northwind.dev"]; !ok && len(keys) > 0 {
		adminID = users[keys[0]].ID
	}
	for key, user := range users {
		if user.OrgID == "" {
			user.OrgID = "org_default"
			user.TeamID = "team_default"
			user.Role = "member"
			if user.ID == adminID {
				user.Role = "admin"
			}
			users[key] = user
		}
	}
	normalize := func(items any) {
		value := reflect.ValueOf(items)
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i)
			scope := item.FieldByName("RecordScope").Interface().(RecordScope)
			if scope.OrgID == "" {
				scope = RecordScope{"org_default", adminID, "team_default", "organization"}
				owner := ""
				for _, field := range []string{"Owner", "CreatedBy", "AssignedTo"} {
					v := item.FieldByName(field)
					if v.IsValid() && v.Kind() == reflect.String && v.String() != "" {
						owner = v.String()
						break
					}
				}
				for _, user := range users {
					if user.OrgID == "org_default" && user.Name == owner {
						scope.OwnerID = user.ID
						scope.TeamID = user.TeamID
						break
					}
				}
				item.FieldByName("RecordScope").Set(reflect.ValueOf(scope))
			}
		}
	}
	for _, items := range []any{contacts, deals, activities, tasks, schools, students, agents, leads, cases, documents, invoices, payments, partners} {
		normalize(items)
	}
}

func migrateWorkspace(tx *sql.Tx, version int) error {
	if version < 2 {
		for _, statement := range []string{
			`ALTER TABLE users ADD COLUMN org_id TEXT NOT NULL DEFAULT 'org_default'`, `ALTER TABLE users ADD COLUMN team_id TEXT NOT NULL DEFAULT 'team_default'`, `ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT ''`, `ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0`, `ALTER TABLE users ADD COLUMN two_factor_enabled INTEGER NOT NULL DEFAULT 0`, `ALTER TABLE users ADD COLUMN auth_data TEXT NOT NULL DEFAULT '{}'`, `ALTER TABLE audit_events ADD COLUMN org_id TEXT NOT NULL DEFAULT 'org_default'`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
	}
	if version < 3 {
		if _, err := tx.Exec("ALTER TABLE audit_events ADD COLUMN scope_data TEXT NOT NULL DEFAULT '{}' "); err != nil {
			return err
		}
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS organizations(id TEXT PRIMARY KEY,name TEXT NOT NULL,timezone TEXT NOT NULL DEFAULT 'Asia/Manila',currency TEXT NOT NULL DEFAULT 'USD',ip_allowlist TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS teams(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,name TEXT NOT NULL,UNIQUE(org_id,name))`,
		`CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,org_id TEXT NOT NULL,access_hash TEXT NOT NULL UNIQUE,refresh_hash TEXT NOT NULL UNIQUE,created_at TEXT NOT NULL,last_seen TEXT NOT NULL,expires_at TEXT NOT NULL,refresh_expires_at TEXT NOT NULL,ip TEXT NOT NULL,user_agent TEXT NOT NULL,version INTEGER NOT NULL,persistent INTEGER NOT NULL DEFAULT 0,revoked_at TEXT)`,
		`CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id,revoked_at)`,
		`CREATE TABLE IF NOT EXISTS auth_tokens(hash TEXT PRIMARY KEY,user_id TEXT NOT NULL,purpose TEXT NOT NULL,expires_at TEXT NOT NULL,used_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS security_events(id INTEGER PRIMARY KEY AUTOINCREMENT,org_id TEXT NOT NULL,user_id TEXT NOT NULL,event TEXT NOT NULL,ip TEXT NOT NULL,user_agent TEXT NOT NULL,occurred_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS login_attempts(key TEXT PRIMARY KEY,failures INTEGER NOT NULL,window_start TEXT NOT NULL,locked_until TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workspace_entries(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,category TEXT NOT NULL,user_id TEXT NOT NULL DEFAULT '',record_id TEXT NOT NULL DEFAULT '',data TEXT NOT NULL CHECK(json_valid(data)),created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS workspace_entries_lookup ON workspace_entries(org_id,category,user_id,record_id)`,
		`CREATE TABLE IF NOT EXISTS file_versions(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,document_id TEXT NOT NULL,version INTEGER NOT NULL,filename TEXT NOT NULL,storage_path TEXT NOT NULL,mime TEXT NOT NULL,size INTEGER NOT NULL,sha256 TEXT NOT NULL,uploaded_by TEXT NOT NULL,uploaded_at TEXT NOT NULL,UNIQUE(document_id,version))`,
		`CREATE TABLE IF NOT EXISTS mail_outbox(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,user_id TEXT NOT NULL,contact_id TEXT NOT NULL DEFAULT '',invoice_id TEXT NOT NULL DEFAULT '',kind TEXT NOT NULL,recipient TEXT NOT NULL,subject TEXT NOT NULL,body TEXT NOT NULL,status TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,sent_at TEXT, next_attempt TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS notifications(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,user_id TEXT NOT NULL,kind TEXT NOT NULL,title TEXT NOT NULL,body TEXT NOT NULL,path TEXT NOT NULL,dedupe_key TEXT NOT NULL,created_at TEXT NOT NULL,read_at TEXT,UNIQUE(user_id,dedupe_key))`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,webhook_id TEXT NOT NULL,payload TEXT NOT NULL,status TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,next_attempt TEXT NOT NULL)`,
		`INSERT OR IGNORE INTO organizations(id,name,created_at) VALUES('org_default','ECC workspace',datetime('now'))`,
		`INSERT OR IGNORE INTO teams VALUES('team_default','org_default','General')`,
		`CREATE INDEX IF NOT EXISTS audit_org_recent ON audit_events(org_id,id DESC)`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	{
		if _, err := tx.Exec("UPDATE audit_events SET scope_data=json_object('orgId',org_id,'ownerId','','teamId','','visibility','private') WHERE scope_data='{}'"); err != nil {
			return err
		}
	}
	// Existing v1 users are assigned safely once; explicit roles are preserved.
	if _, err := tx.Exec(`UPDATE users SET role=CASE WHEN id=COALESCE((SELECT id FROM users WHERE id='u_1'),(SELECT id FROM users ORDER BY email LIMIT 1)) THEN 'admin' ELSE 'member' END WHERE role=''`); err != nil {
		return err
	}
	return nil
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !isAdmin(currentUser(r)) {
		writeError(w, 403, "Administrator access is required")
		return false
	}
	return true
}
func decodeRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	if err := decoder.Decode(value); err != nil {
		writeError(w, 400, "Invalid request body")
		return false
	}
	return true
}
func publicError(err error) string { return fmt.Sprintf("Operation failed: %s", err) }

func newRecordScope(r *http.Request) RecordScope {
	if scope, ok := r.Context().Value(relatedScopeKey).(RecordScope); ok {
		return scope
	}
	user := currentUser(r)
	return RecordScope{user.OrgID, user.ID, user.TeamID, "organization"}
}

func computedOnly(entity, before, after string) bool {
	if after == "" {
		return false
	}
	var a, b map[string]any
	json.Unmarshal([]byte(before), &a)
	json.Unmarshal([]byte(after), &b)
	keys := map[string][]string{"schools": {"studentsEnrolled"}, "agents": {"studentsReferred"}, "partners": {"casesReferred", "casesConverted"}}[entity]
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		delete(a, key)
		delete(b, key)
	}
	return mustJSON(a) == mustJSON(b)
}

func danglingDeletedRelations(before, after diskStore) bool {
	old, next := recordsOf(before), recordsOf(after)
	deleted := map[string]bool{}
	for entity, items := range old {
		if entity == "users" {
			continue
		}
		for id := range items {
			if _, ok := next[entity][id]; !ok {
				deleted[id] = true
			}
		}
	}
	if len(deleted) == 0 {
		return false
	}
	for entity, items := range next {
		if entity == "users" {
			continue
		}
		for _, payload := range items {
			var values map[string]any
			json.Unmarshal([]byte(payload), &values)
			for _, key := range []string{"contactId", "clientId", "caseId", "invoiceId", "schoolId", "agentId", "partnerId", "parentCaseId", "studentId", "dealId"} {
				id, _ := values[key].(string)
				if deleted[id] {
					return true
				}
			}
		}
	}
	return false
}

func filterProjectedStatus(s *diskStore, r *http.Request) {
	value := r.URL.Query().Get("filterStatus")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		return
	}
	entity := parts[1]
	field := map[string]string{"contacts": "Stage", "deals": "Stage", "tasks": "Status", "schools": "ContractStatus", "students": "AcceptanceStatus", "agents": "AgentStatus", "leads": "Status", "cases": "CurrentStage", "documents": "Status", "invoices": "PaymentMilestone", "partners": "Type"}[entity]
	if field == "" {
		return
	}
	list := reflect.ValueOf(s).Elem().FieldByName(strings.ToUpper(entity[:1]) + entity[1:])
	if !list.IsValid() {
		return
	}
	out := reflect.MakeSlice(list.Type(), 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		item := list.Index(i)
		if strings.EqualFold(item.FieldByName(field).String(), value) {
			out = reflect.Append(out, item)
		}
	}
	list.Set(out)
}
