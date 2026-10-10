package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuditEvent struct {
	Scope      RecordScope `json:"-"`
	OrgID      string      `json:"orgId"`
	ID         int64       `json:"id"`
	Actor      string      `json:"actor"`
	Action     string      `json:"action"`
	Entity     string      `json:"entity"`
	RecordID   string      `json:"recordId"`
	Label      string      `json:"label"`
	OccurredAt string      `json:"occurredAt"`
}

func requestActor(r *http.Request) string {
	token, err := jwt.Parse(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), func(t *jwt.Token) (any, error) { return jwtSecret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err == nil && token.Valid {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if email, ok := claims["email"].(string); ok {
				return email
			}
		}
	}
	return "Account registration"
}

func recordsOf(saved diskStore) map[string]map[string]string {
	result := map[string]map[string]string{}
	groups := reflect.ValueOf(saved)
	for _, table := range entityTables {
		items := groups.FieldByName(strings.ToUpper(table[:1]) + table[1:])
		records := make(map[string]string, items.Len())
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i)
			payload, _ := json.Marshal(item.Interface())
			records[item.FieldByName("ID").String()] = string(payload)
		}
		result[table] = records
	}
	// Account history never includes passwords, hashes, or session tokens.
	result["users"] = map[string]string{}
	for _, stored := range saved.Users {
		payload, _ := json.Marshal(stored.User)
		result["users"][stored.User.ID] = string(payload)
	}
	return result
}
func recordLabel(payload string) string {
	var data map[string]any
	json.Unmarshal([]byte(payload), &data)
	for _, key := range []string{"name", "title", "legalCompanyName", "caseNumber", "invoiceNumber", "docName", "id"} {
		if label, ok := data[key].(string); ok && label != "" {
			return label
		}
	}
	return "Record"
}
func changesBetween(before, after diskStore, actor string) []AuditEvent {
	old, new := recordsOf(before), recordsOf(after)
	events := []AuditEvent{}
	for table, records := range new {
		for id, payload := range records {
			action := "updated"
			previous, exists := old[table][id]
			if previous == payload {
				continue
			}
			if !exists {
				action = "created"
			}
			events = append(events, AuditEvent{Scope: auditScope(payload, table), OrgID: recordOrg(payload), Actor: actor, Action: action, Entity: table, RecordID: id, Label: recordLabel(payload), OccurredAt: time.Now().UTC().Format(time.RFC3339)})
		}
		for id, payload := range old[table] {
			if _, exists := records[id]; !exists {
				events = append(events, AuditEvent{Scope: auditScope(payload, table), OrgID: recordOrg(payload), Actor: actor, Action: "deleted", Entity: table, RecordID: id, Label: recordLabel(payload), OccurredAt: time.Now().UTC().Format(time.RFC3339)})
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Entity+events[i].RecordID < events[j].Entity+events[j].RecordID })
	return events
}

func handleAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseLimitOffset(r, 25, 0)
	actor := strings.TrimSpace(r.URL.Query().Get("actor"))
	predicate, args := scopeSQL(r, "scope_data")
	where := " WHERE " + predicate
	for _, table := range entityTables {
		scope, scopeArgs := scopeSQL(r, "data")
		where += " AND (entity<>? OR record_id NOT IN (SELECT id FROM " + table + ") OR record_id IN (SELECT id FROM " + table + " WHERE " + scope + "))"
		args = append(args, table)
		args = append(args, scopeArgs...)
	}
	if actor != "" {
		where += " AND actor=?"
		args = append(args, actor)
	}
	var total int
	if err := storeDB(r).QueryRow("SELECT count(*) FROM audit_events"+where, args...).Scan(&total); err != nil {
		writeError(w, 500, "Could not load history")
		return
	}
	args = append(args, limit, offset)
	rows, err := storeDB(r).Query("SELECT id,actor,action,entity,record_id,label,occurred_at FROM audit_events"+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		writeError(w, 500, "Could not load history")
		return
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		if err = rows.Scan(&event.ID, &event.Actor, &event.Action, &event.Entity, &event.RecordID, &event.Label, &event.OccurredAt); err != nil {
			writeError(w, 500, "Could not load history")
			return
		}
		events = append(events, event)
	}
	if rows.Err() != nil {
		writeError(w, 500, "Could not load history")
		return
	}
	writeJSON(w, 200, map[string]any{"events": events, "total": total})
}

type searchEntity struct{ table, title, subtitle string }

var searchableEntities = []searchEntity{
	{"contacts", "name", "email"}, {"deals", "title", "contactName"}, {"tasks", "title", "status"},
	{"schools", "name", "countryCode"}, {"students", "name", "studentCode"}, {"agents", "name", "agentCode"},
	{"documents", "docName", "caseNumber"}, {"payments", "reference", "method"}, {"activities", "title", "body"}, {"leads", "name", "email"}, {"cases", "caseNumber", "clientName"}, {"invoices", "invoiceNumber", "clientName"}, {"partners", "legalCompanyName", "contactEmail"},
}

type SearchResult struct {
	Entity   string `json:"entity"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

func (state *runtimeStore) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 2 {
		writeJSON(w, 200, map[string]any{"results": []SearchResult{}, "truncated": false})
		return
	}
	if len(query) > 200 {
		writeError(w, 400, "Search is limited to 200 characters")
		return
	}
	escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query)
	terms := []string{}
	args := []any{}
	for _, entity := range searchableEntities {
		title := "coalesce(json_extract(data,'$." + entity.title + "'),'')"
		subtitle := "coalesce(json_extract(data,'$." + entity.subtitle + "'),'')"
		fields := []string{title, subtitle, "coalesce(json_extract(data,'$.company'),'')", "coalesce(json_extract(data,'$.phone'),'')", "coalesce(json_extract(data,'$.contactName'),'')"}
		scope, scopeArgs := scopeSQL(r, "data")
		terms = append(terms, "SELECT '"+entity.table+"' AS entity,id,"+title+" AS title,"+subtitle+" AS subtitle FROM "+entity.table+" WHERE ("+strings.Join(fields, " || ' ' || ")+") LIKE ? ESCAPE '\\'")
		terms[len(terms)-1] += " AND (" + scope + ")"
		args = append(args, "%"+escaped+"%")
		args = append(args, scopeArgs...)
	}
	rows, err := storeDB(r).Query("SELECT * FROM ("+strings.Join(terms, " UNION ALL ")+") ORDER BY title COLLATE NOCASE,entity,id LIMIT 31", args...)
	if err != nil {
		writeError(w, 500, "Could not search records")
		return
	}
	defer rows.Close()
	results := []SearchResult{}
	for rows.Next() {
		var result SearchResult
		if err = rows.Scan(&result.Entity, &result.ID, &result.Title, &result.Subtitle); err != nil {
			writeError(w, 500, "Could not search records")
			return
		}
		if _, visible := recordsOf(state.snapshot())[result.Entity][result.ID]; visible {
			results = append(results, result)
		}
	}
	if rows.Err() != nil {
		writeError(w, 500, "Could not search records")
		return
	}
	truncated := len(results) > 30
	if truncated {
		results = results[:30]
	}
	writeJSON(w, 200, map[string]any{"results": results, "truncated": truncated})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSearch(w, r)
}

func csvValue(value any) string {
	var result string
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		result = v
	case float64:
		result = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		result = strconv.FormatBool(v)
	default:
		payload, _ := json.Marshal(value)
		result = string(payload)
	}
	// Neutralize spreadsheet formulas, including leading whitespace/control characters.
	trimmed := strings.TrimLeft(result, " \t\r\n")
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		if _, ok := value.(string); ok {
			result = "'" + result
		}
	}
	return result
}
func (state *runtimeStore) handleExport(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("entity")
	allowed := false
	for _, name := range entityTables {
		if name == table {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, 404, "Unknown export type")
		return
	}
	scope, args := scopeSQL(r, "data")
	rows, err := storeDB(r).Query("SELECT data FROM "+table+" WHERE "+scope+" ORDER BY sort_order,id", args...)
	if err != nil {
		writeError(w, 500, "Could not export records")
		return
	}
	defer rows.Close()
	records := []map[string]any{}
	columns := map[string]bool{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			writeError(w, 500, "Could not export records")
			return
		}
		var data map[string]any
		if err = json.Unmarshal(payload, &data); err != nil {
			writeError(w, 500, "Could not export records")
			return
		}
		id, _ := data["id"].(string)
		if _, visible := recordsOf(state.snapshot())[table][id]; !visible {
			continue
		}
		records = append(records, data)
		for key := range data {
			columns[key] = true
		}
	}
	if rows.Err() != nil {
		writeError(w, 500, "Could not export records")
		return
	}
	header := []string{}
	for key := range columns {
		header = append(header, key)
	}
	sort.Strings(header)
	if len(header) == 0 {
		header = []string{"id"}
	}
	switch r.URL.Query().Get("format") {
	case "xlsx", "pdf":
		format := r.URL.Query().Get("format")
		var data []byte
		var exportErr error
		if format == "xlsx" {
			data, exportErr = xlsxExport(header, records)
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		} else {
			data, exportErr = pdfExport(table, header, records)
			w.Header().Set("Content-Type", "application/pdf")
		}
		if exportErr != nil {
			writeError(w, 500, "Could not generate export")
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, table, format))
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, table))
	w.Header().Set("Cache-Control", "no-store")
	writer := csv.NewWriter(w)
	writer.Write(header)
	for _, record := range records {
		line := []string{}
		for _, key := range header {
			line = append(line, csvValue(record[key]))
		}
		writer.Write(line)
	}
	writer.Flush()
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleExport(w, r)
}

func auditScope(payload, entity string) RecordScope {
	var scope RecordScope
	json.Unmarshal([]byte(payload), &scope)
	if scope.OrgID == "" {
		scope.OrgID = recordOrg(payload)
	}
	if entity == "users" {
		scope.Visibility = "private"
		scope.OwnerID = ""
	}
	return scope
}
