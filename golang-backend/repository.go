package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Legacy API URLs retain their response shape while their list/detail reads use
// scoped SQL, paging and ordering without replacing the process-wide cache.
func repositoryRoute(r *http.Request) (string, string, bool) {
	if r.Method != "GET" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "api" || !validEntity(parts[1]) {
		return "", "", false
	}
	id := ""
	if len(parts) == 3 {
		id = parts[2]
	}
	return parts[1], id, true
}
func repositoryFields(entity string) map[string]reflect.Kind {
	types := map[string]any{"contacts": Contact{}, "deals": Deal{}, "activities": Activity{}, "tasks": Task{}, "schools": School{}, "students": Student{}, "agents": Agent{}, "leads": Lead{}, "cases": Case{}, "documents": Document{}, "invoices": Invoice{}, "payments": Payment{}, "partners": Partner{}}
	out := map[string]reflect.Kind{}
	var collect func(reflect.Type)
	collect = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				collect(f.Type)
				continue
			}
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			if key != "" && key != "-" {
				out[key] = f.Type.Kind()
			}
		}
	}
	collect(reflect.TypeOf(types[entity]))
	return out
}
func repositoryPredicate(r *http.Request, entity string) (string, []any) {
	predicate, args := scopeSQL(r, "record.data")
	// Related documents/payments/activity retain parent visibility checks in SQL.
	parents := map[string]string{}
	switch entity {
	case "documents":
		parents["cases"] = "caseId"
	case "payments":
		parents["invoices"] = "invoiceId"
	case "activities":
		parents["contacts"] = "contactId"
		parents["deals"] = "dealId"
	}
	for parent, field := range parents {
		scope, scopeArgs := scopeSQL(r, "parent.data")
		optional := entity == "activities"
		exists := "EXISTS(SELECT 1 FROM " + parent + " parent WHERE parent.id=json_extract(record.data,'$." + field + "') AND " + scope + ")"
		if optional {
			exists = "(COALESCE(json_extract(record.data,'$." + field + "'),'')='' OR " + exists + ")"
		}
		predicate += " AND " + exists
		args = append(args, scopeArgs...)
	}
	fields := repositoryFields(entity)
	filters := map[string]string{"agentId": "agentId", "schoolId": "schoolId", "clientId": "clientId", "caseId": "caseId", "invoiceId": "invoiceId", "status": "status", "stage": "currentStage", "milestone": "paymentMilestone"}
	for query, field := range filters {
		if value := strings.TrimSpace(r.URL.Query().Get(query)); value != "" {
			if _, ok := fields[field]; ok {
				predicate += " AND json_extract(record.data,'$." + field + "')=?"
				args = append(args, value)
			}
		}
	}
	statusField := map[string]string{"contacts": "stage", "deals": "stage", "tasks": "status", "schools": "contractStatus", "students": "acceptanceStatus", "agents": "agentStatus", "leads": "status", "cases": "currentStage", "documents": "status", "invoices": "paymentMilestone", "partners": "type"}[entity]
	if value := r.URL.Query().Get("filterStatus"); value != "" && statusField != "" {
		predicate += " AND lower(json_extract(record.data,'$." + statusField + "'))=lower(?)"
		args = append(args, value)
	}
	if r.URL.Query().Get("mine") == "true" {
		predicate += " AND json_extract(record.data,'$.ownerId')=?"
		args = append(args, currentUser(r).ID)
	}
	searchFields := map[string][]string{
		"contacts": {"name", "email", "company"}, "deals": {"title", "contactName", "owner"}, "tasks": {"title", "description", "contactName", "owner"}, "schools": {"name", "countryCode", "contactPerson"}, "students": {"name", "studentCode", "schoolName", "agentName", "program"}, "agents": {"name", "agentCode", "countryCode"}, "leads": {"name", "leadNumber", "email", "interestedCountry", "assignedTo"}, "cases": {"caseNumber", "clientName", "serviceCategory", "destinationCountry", "assignedOfficer"}, "documents": {"docName", "caseNumber"}, "invoices": {"invoiceNumber", "clientName", "caseNumber"}, "partners": {"legalCompanyName", "contactPerson", "country"}, "activities": {"title", "body"},
	}
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))); q != "" {
		clauses := []string{}
		for _, field := range searchFields[entity] {
			clauses = append(clauses, "instr(lower(COALESCE(json_extract(record.data,'$."+field+"'),'')),?)>0")
			args = append(args, q)
		}
		if len(clauses) > 0 {
			predicate += " AND (" + strings.Join(clauses, " OR ") + ")"
		}
	}
	return predicate, args
}
func enrichRepositoryRecord(r *http.Request, entity string, record map[string]any) error {
	if entity != "schools" && entity != "agents" {
		return nil
	}
	field, countField := "schoolId", "studentsEnrolled"
	if entity == "agents" {
		field, countField = "agentId", "studentsReferred"
	}
	scope, args := scopeSQL(r, "data")
	args = append(args, fmtString(record["id"]))
	var count int
	if err := storeDB(r).QueryRow("SELECT count(*) FROM students WHERE "+scope+" AND json_extract(data,'$."+field+"')=?", args...).Scan(&count); err != nil {
		return err
	}
	record[countField] = count
	return nil
}
func handleRepositoryRead(w http.ResponseWriter, r *http.Request, entity, id string) {
	tx, err := database.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		writeError(w, 503, "Database unavailable")
		return
	}
	defer tx.Rollback()
	r = withDBTransaction(r, tx)
	if id != "" {
		record, err := sqlRecord(r, entity, id)
		if err == sql.ErrNoRows {
			writeError(w, 404, "Record not found")
			return
		}
		if err != nil {
			writeError(w, 503, "Could not load record")
			return
		}
		if err = enrichRepositoryRecord(r, entity, record); err != nil {
			writeError(w, 500, "Could not load record totals")
			return
		}
		singular := map[string]string{"contacts": "contact", "deals": "deal", "activities": "activity", "tasks": "task", "schools": "school", "students": "student", "agents": "agent", "leads": "lead", "cases": "case", "documents": "document", "invoices": "invoice", "payments": "payment", "partners": "partner"}[entity]
		writeJSON(w, 200, map[string]any{singular: record})
		return
	}
	where, args := repositoryPredicate(r, entity)
	var total int
	if err = storeDB(r).QueryRow("SELECT count(*) FROM "+entity+" record WHERE "+where, args...).Scan(&total); err != nil {
		writeError(w, 500, "Could not load record totals")
		return
	}
	order := "record.sort_order,record.id"
	field := r.URL.Query().Get("sort")
	if kind, valid := repositoryFields(entity)[field]; valid {
		expression := "json_extract(record.data,'$." + field + "')"
		if kind == reflect.String {
			expression = "lower(COALESCE(" + expression + ",''))"
		}
		direction := " ASC"
		if r.URL.Query().Get("direction") == "desc" {
			direction = " DESC"
		}
		order = expression + direction + ",record.sort_order,record.id"
	}
	limit, offset := parseLimitOffset(r, 50, 0)
	args = append(args, limit, offset)
	rows, err := storeDB(r).Query("SELECT record.data FROM "+entity+" record WHERE "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		writeError(w, 500, "Could not load records")
		return
	}
	records := []map[string]any{}
	for rows.Next() {
		var data string
		var record map[string]any
		if err = rows.Scan(&data); err == nil {
			err = json.Unmarshal([]byte(data), &record)
		}
		if err != nil {
			break
		}
		records = append(records, record)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeError(w, 500, "Could not load records")
		return
	}
	for _, record := range records {
		if err = enrichRepositoryRecord(r, entity, record); err != nil {
			writeError(w, 500, "Could not load linked totals")
			return
		}
	}
	writeJSON(w, 200, map[string]any{entity: records, "total": total, "limit": limit, "offset": offset})
}

// Normal CRM mutations persist only changed rows with organization and previous
// payload guards. Full synchronization remains available for bootstrap/import.
func saveStoreDeltaTx(tx *sql.Tx, before, after diskStore, events []AuditEvent) error {
	oldRecords, newRecords := recordsOf(before), recordsOf(after)
	changedCases := []Case{}
	for _, entity := range entityTables {
		for id, old := range oldRecords[entity] {
			next := newRecords[entity][id]
			if old == next {
				continue
			}
			org := recordOrg(old)
			var result sql.Result
			var err error
			if next == "" {
				result, err = tx.Exec("DELETE FROM "+entity+" WHERE id=? AND json_extract(data,'$.orgId')=? AND data=?", id, org, old)
			} else {
				if recordOrg(next) != org {
					return fmt.Errorf("record cannot change organization")
				}
				result, err = tx.Exec("UPDATE "+entity+" SET data=? WHERE id=? AND json_extract(data,'$.orgId')=? AND data=?", next, id, org, old)
			}
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("record changed outside this process; reload before editing")
			}
		}
		list := reflect.ValueOf(after).FieldByName(strings.ToUpper(entity[:1]) + entity[1:])
		for i := 0; i < list.Len(); i++ {
			id := idOfValue(list.Index(i))
			next := newRecords[entity][id]
			if oldRecords[entity][id] != "" {
				continue
			}
			if _, err := tx.Exec("INSERT INTO "+entity+"(id,sort_order,data) SELECT ?,COALESCE(MAX(sort_order),-1)+1,? FROM "+entity, id, next); err != nil {
				return err
			}
		}
	}
	for _, c := range after.Cases {
		if oldRecords["cases"][c.ID] != newRecords["cases"][c.ID] {
			changedCases = append(changedCases, c)
		}
	}
	if err := trackCaseStages(tx, diskStore{Cases: changedCases}); err != nil {
		return err
	}
	for key, stored := range after.Users {
		if mustJSON(stored) == mustJSON(before.Users[key]) {
			continue
		}
		u := stored.User
		private := mustJSON(struct {
			LastTOTPStep   int64
			TOTPSecret     string
			RecoveryHashes []string
		}{stored.LastTOTPStep, stored.TOTPSecret, stored.RecoveryHashes})
		if _, err := tx.Exec(`INSERT INTO users(id,email,name,password_hash,session_version,org_id,team_id,role,disabled,two_factor_enabled,auth_data) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET email=excluded.email,name=excluded.name,password_hash=excluded.password_hash,session_version=excluded.session_version,org_id=excluded.org_id,team_id=excluded.team_id,role=excluded.role,disabled=excluded.disabled,two_factor_enabled=excluded.two_factor_enabled,auth_data=excluded.auth_data WHERE users.org_id=excluded.org_id`, u.ID, u.Email, u.Name, stored.Password, stored.SessionVersion, u.OrgID, u.TeamID, u.Role, u.Disabled, u.TwoFactorEnabled, private); err != nil {
			return err
		}
	}
	for id, expiry := range before.Revoked {
		if _, ok := after.Revoked[id]; !ok || expiry.Before(time.Now()) {
			if _, err := tx.Exec("DELETE FROM revoked_tokens WHERE id=?", id); err != nil {
				return err
			}
		}
	}
	for id, expiry := range after.Revoked {
		if expiry.After(time.Now()) && expiry != before.Revoked[id] {
			if _, err := tx.Exec("INSERT OR REPLACE INTO revoked_tokens VALUES(?,?)", id, expiry.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		}
	}
	for _, event := range events {
		if _, err := tx.Exec("INSERT INTO audit_events(actor,action,entity,record_id,label,occurred_at,org_id,scope_data) VALUES(?,?,?,?,?,?,?,?)", event.Actor, event.Action, event.Entity, event.RecordID, event.Label, event.OccurredAt, event.OrgID, mustJSON(event.Scope)); err != nil {
			return err
		}
	}
	return nil
}
func idOfValue(v reflect.Value) string { return v.FieldByName("ID").String() }

func withDBTransaction(r *http.Request, tx *sql.Tx) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestTxKey, tx))
}
