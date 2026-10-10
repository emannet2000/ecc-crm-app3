package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// These routes read committed SQL data without replacing the shared CRM cache.
func independentRead(r *http.Request) bool {
	_, _, repository := repositoryRoute(r)
	return repository || r.Method == "GET" && (r.URL.Path == "/api/reports" || r.URL.Path == "/api/audit") || independentDailyRead(r) || independentAI(r) || r.Method == "GET" && (strings.HasPrefix(r.URL.Path, "/api/records/") || r.URL.Path == "/api/insights" || r.URL.Path == "/api/health")
}
func registerExpansionRoutes(m *http.ServeMux) {
	registerDailyRoutes(m)
	m.HandleFunc("GET /api/records/{entity}/{id}", authMiddleware(handleSQLRecord))
	m.HandleFunc("GET /api/insights", authMiddleware(handleInsights))
	m.HandleFunc("GET /api/trash", authMiddleware(handleTrash))
	m.HandleFunc("POST /api/trash/{batch}/restore", authMiddleware(handleRestoreTrash))
	registerPortalRoutes(m)
	registerConversationRoutes(m)
	registerAssistantRoutes(m)
	registerRecordPhotoRoutes(m)
}
func migrateExpansion(tx *sql.Tx) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS contacts_org_email ON contacts(json_extract(data,'$.orgId'),lower(json_extract(data,'$.email')))`,
		`CREATE TABLE IF NOT EXISTS trash_batches(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,actor TEXT NOT NULL,label TEXT NOT NULL,data TEXT NOT NULL,deleted_at TEXT NOT NULL,restored_at TEXT)`,
		`CREATE INDEX IF NOT EXISTS trash_org ON trash_batches(org_id,deleted_at)`,
		`CREATE TABLE IF NOT EXISTS portal_access(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,student_id TEXT NOT NULL,contact_id TEXT NOT NULL,email TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,created_at TEXT NOT NULL,expires_at TEXT NOT NULL,revoked_at TEXT,activated_at TEXT)`,
		`CREATE INDEX IF NOT EXISTS portal_student ON portal_access(org_id,student_id)`,
		`CREATE TABLE IF NOT EXISTS conversations(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,contact_id TEXT NOT NULL,case_id TEXT NOT NULL DEFAULT '',channel TEXT NOT NULL,direction TEXT NOT NULL,provider_id TEXT NOT NULL,subject TEXT NOT NULL,body TEXT NOT NULL,status TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(org_id,channel,provider_id))`,
		`CREATE INDEX IF NOT EXISTS conversation_contact ON conversations(org_id,contact_id,created_at)`,
		`CREATE TABLE IF NOT EXISTS inbound_events(provider TEXT NOT NULL,event_id TEXT NOT NULL,received_at TEXT NOT NULL,PRIMARY KEY(provider,event_id))`,
		`CREATE TABLE IF NOT EXISTS case_stage_events(id TEXT PRIMARY KEY,org_id TEXT NOT NULL,case_id TEXT NOT NULL,stage TEXT NOT NULL,entered_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS case_stage_case ON case_stage_events(org_id,case_id,entered_at)`,
		`CREATE TABLE IF NOT EXISTS record_photos(org_id TEXT NOT NULL,entity TEXT NOT NULL,record_id TEXT NOT NULL,mime TEXT NOT NULL,data BLOB NOT NULL,updated_at TEXT NOT NULL,PRIMARY KEY(org_id,entity,record_id))`,
	}
	for _, table := range entityTables {
		statements = append(statements, `CREATE INDEX IF NOT EXISTS `+table+`_scope ON `+table+`(json_extract(data,'$.orgId'),json_extract(data,'$.ownerId'),json_extract(data,'$.visibility'),json_extract(data,'$.teamId'))`)
	}
	for _, q := range statements {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return migrateAI(tx)
}
func validEntity(entity string) bool {
	for _, e := range entityTables {
		if entity == e {
			return true
		}
	}
	return false
}
func versionOf(record map[string]any) string {
	copy := make(map[string]any, len(record))
	for k, v := range record {
		if k != "_revision" {
			copy[k] = v
		}
	}
	b, _ := json.Marshal(copy)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func withRecordVersions(payload any) any {
	b, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var value any
	if json.Unmarshal(b, &value) != nil {
		return payload
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, hasScope := x["visibility"]; hasScope && x["id"] != nil {
				x["_revision"] = versionOf(x)
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(value)
	return value
}
func checkRecordVersion(w http.ResponseWriter, r *http.Request, s diskStore) bool {
	if r.Method == "GET" || r.Method == "HEAD" {
		return true
	}
	expected := strings.Trim(r.Header.Get("If-Match"), "\"")
	if expected == "" {
		return true
	} // Compatibility for existing integrations.
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	entity, id := "", r.PathValue("id")
	if len(parts) > 1 && validEntity(parts[1]) {
		entity = parts[1]
	}
	if strings.HasPrefix(r.URL.Path, "/api/workflows/") {
		b, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			writeError(w, 400, "Invalid request")
			return false
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		var req struct{ ID string }
		json.Unmarshal(b, &req)
		id = req.ID
		for e, items := range recordsOf(s) {
			if validEntity(e) && items[id] != "" {
				entity = e
				break
			}
		}
	}
	if entity == "" || id == "" {
		return true
	}
	payload, found := recordsOf(s)[entity][id]
	if !found {
		writeError(w, 404, "Record not found")
		return false
	}
	var record map[string]any
	json.Unmarshal([]byte(payload), &record)
	if expected != versionOf(record) {
		writeError(w, 409, "This record changed since you opened it. Reload the record before saving.")
		return false
	}
	return true
}
func sqlRecord(r *http.Request, entity, id string) (map[string]any, error) {
	if !validEntity(entity) {
		return nil, sql.ErrNoRows
	}
	clause, args := scopeSQL(r, "data")
	args = append(args, id)
	var data string
	if err := storeDB(r).QueryRow("SELECT data FROM "+entity+" WHERE "+clause+" AND id=?", args...).Scan(&data); err != nil {
		return nil, err
	}
	var record map[string]any
	err := json.Unmarshal([]byte(data), &record)
	if err == nil {
		parents := map[string]string{}
		switch entity {
		case "documents":
			parents["cases"] = fmtString(record["caseId"])
		case "payments":
			parents["invoices"] = fmtString(record["invoiceId"])
		case "activities":
			if id := fmtString(record["contactId"]); id != "" {
				parents["contacts"] = id
			}
			if id := fmtString(record["dealId"]); id != "" {
				parents["deals"] = id
			}
		}
		for parentEntity, parentID := range parents {
			if _, err = sqlRecord(r, parentEntity, parentID); err != nil {
				return nil, err
			}
		}
	}
	return record, err
}
func handleSQLRecord(w http.ResponseWriter, r *http.Request) {
	record, err := sqlRecord(r, r.PathValue("entity"), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, 503, "Could not load the record. Retry shortly.")
			return
		}
		writeError(w, 404, "Record not found")
		return
	}
	w.Header().Set("ETag", "\""+versionOf(record)+"\"")
	var scope RecordScope
	b, _ := json.Marshal(record)
	json.Unmarshal(b, &scope)
	writeJSON(w, 200, map[string]any{"record": record, "editable": canWrite(scope, currentUser(r)), "features": integrationStatusFor(r)})
}

type trashChange struct {
	Entity string `json:"entity"`
	ID     string `json:"id"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func archiveDeleted(r *http.Request, before, after diskStore, actor string) error {
	a, b := recordsOf(before), recordsOf(after)
	changes := []trashChange{}
	deleted := false
	label := ""
	for e, records := range a {
		if !validEntity(e) {
			continue
		}
		for id, payload := range records {
			if b[e][id] == payload {
				continue
			}
			if b[e][id] == "" {
				deleted = true
				if label == "" {
					label = recordLabel(payload)
				}
			}
			if b[e][id] == "" && (e == "contacts" || e == "students") {
				column := "contact_id"
				if e == "students" {
					column = "student_id"
				}
				if _, err := storeDB(r).Exec("UPDATE portal_access SET revoked_at=? WHERE "+column+"=? AND revoked_at IS NULL", utcNow(), id); err != nil {
					return err
				}
			}
			if b[e][id] == "" && e == "cases" {
				if _, err := storeDB(r).Exec("INSERT INTO case_stage_events VALUES(?,?,?,?,?)", newID("stage"), recordOrg(payload), id, "Deleted", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					return err
				}
			}
			changes = append(changes, trashChange{e, id, payload, b[e][id]})
		}
	}
	if !deleted {
		return nil
	}
	org := metadata(r).OrgID
	if org == "" {
		return fmt.Errorf("missing deletion organization")
	}
	_, err := storeDB(r).Exec("INSERT INTO trash_batches VALUES(?,?,?,?,?,?,NULL)", newID("trash"), org, actor, label, mustJSON(changes), utcNow())
	return err
}
func handleTrash(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	rows, err := queryObjects(r, "SELECT id,actor,label,deleted_at AS deletedAt FROM trash_batches WHERE org_id=? AND restored_at IS NULL ORDER BY deleted_at DESC LIMIT ? OFFSET ?", currentUser(r).OrgID, limit, offset)
	if err != nil {
		writeError(w, 500, "Could not load trash")
		return
	}
	writeJSON(w, 200, map[string]any{"batches": rows})
}
func replaceSavedRecord(saved *diskStore, entity, id, payload string) error {
	fields := map[string]string{"contacts": "Contacts", "deals": "Deals", "activities": "Activities", "tasks": "Tasks", "schools": "Schools", "students": "Students", "agents": "Agents", "leads": "Leads", "cases": "Cases", "documents": "Documents", "invoices": "Invoices", "payments": "Payments", "partners": "Partners"}
	field := reflect.ValueOf(saved).Elem().FieldByName(fields[entity])
	if !field.IsValid() {
		return fmt.Errorf("invalid entity")
	}
	item := reflect.New(field.Type().Elem())
	if err := json.Unmarshal([]byte(payload), item.Interface()); err != nil {
		return err
	}
	for i := 0; i < field.Len(); i++ {
		if field.Index(i).FieldByName("ID").String() == id {
			field.Index(i).Set(item.Elem())
			return nil
		}
	}
	field.Set(reflect.Append(field, item.Elem()))
	return nil
}
func (state *runtimeStore) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	var data string
	if storeDB(r).QueryRow("SELECT data FROM trash_batches WHERE id=? AND org_id=? AND restored_at IS NULL", r.PathValue("batch"), currentUser(r).OrgID).Scan(&data) != nil {
		writeError(w, 404, "Deleted records not found")
		return
	}
	var changes []trashChange
	if json.Unmarshal([]byte(data), &changes) != nil {
		writeError(w, 500, "Invalid archive")
		return
	}
	saved := state.cloneStore()
	current := recordsOf(saved)
	for _, c := range changes {
		if current[c.Entity][c.ID] != c.After {
			writeError(w, 409, "A related record changed after deletion. Restore requires resolving that conflict first.")
			return
		}
		var scope RecordScope
		json.Unmarshal([]byte(c.Before), &scope)
		if !canWrite(scope, currentUser(r)) {
			writeError(w, 403, "Cannot restore records outside your workspace")
			return
		}
		if err := replaceSavedRecord(&saved, c.Entity, c.ID, c.Before); err != nil {
			writeError(w, 500, "Could not restore record")
			return
		}
	}
	// Related parents may have been deleted in a later batch. Do not resurrect dangling links.
	if !validRestoredRelations(saved, changes) {
		writeError(w, 409, "Restore the related parent records first")
		return
	}
	if _, err := storeDB(r).Exec("UPDATE trash_batches SET restored_at=? WHERE id=?", utcNow(), r.PathValue("batch")); err != nil {
		writeError(w, 500, "Could not restore archive")
		return
	}
	state.restoreStore(saved)
	writeJSON(w, 200, statusResponse{"restored"})
}

func handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRestoreTrash(w, r)
}
func validRestoredRelations(s diskStore, changes []trashChange) bool {
	records := recordsOf(s)
	refs := map[string]string{"contactId": "contacts", "clientId": "contacts", "caseId": "cases", "invoiceId": "invoices", "studentId": "students", "schoolId": "schools", "agentId": "agents", "partnerId": "partners", "parentCaseId": "cases", "dealId": "deals", "convertedContactId": "contacts"}
	for _, c := range changes {
		var record map[string]any
		json.Unmarshal([]byte(c.Before), &record)
		for field, entity := range refs {
			if id, ok := record[field].(string); ok && id != "" && records[entity][id] == "" {
				return false
			}
		}
	}
	return true
}

// Independent reads authorize against committed account roles, never a role that
// a legacy write has temporarily staged in the global cache.
func committedUser(r *http.Request, column, key string) (User, bool) {
	if column != "id" && column != "email" {
		return User{}, false
	}
	var u User
	var raw string
	err := storeDB(r).QueryRow("SELECT id,email,name,password_hash,session_version,org_id,team_id,role,disabled,two_factor_enabled,auth_data FROM users WHERE "+column+"=?", key).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.SessionVersion, &u.OrgID, &u.TeamID, &u.Role, &u.Disabled, &u.TwoFactorEnabled, &raw)
	var private struct {
		LastTOTPStep   int64
		TOTPSecret     string
		RecoveryHashes []string
	}
	if err == nil {
		err = json.Unmarshal([]byte(raw), &private)
		u.LastTOTPStep = private.LastTOTPStep
		u.TOTPSecret = private.TOTPSecret
		u.RecoveryHashes = private.RecoveryHashes
	}
	return u, err == nil
}
