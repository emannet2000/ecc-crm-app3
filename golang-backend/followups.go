package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var followupAutomationFailures atomic.Int64

type followupRule struct {
	Name             string `json:"name"`
	Entity           string `json:"entity"`
	Stage            string `json:"stage"`
	InactiveDays     int    `json:"inactiveDays"`
	TaskTitle        string `json:"taskTitle"`
	Description      string `json:"description"`
	Enabled          bool   `json:"enabled"`
	PreviewToken     string `json:"previewToken,omitempty"`
	PreviewExpiresAt string `json:"previewExpiresAt,omitempty"`
}

type followupMatch struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Entity    string      `json:"entity"`
	LastTouch string      `json:"lastTouch"`
	Scope     RecordScope `json:"-"`
	ContactID string      `json:"-"`
	Owner     string      `json:"-"`
}

func registerFollowupRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/follow-up-rules", authMiddleware(listFollowupRules))
	m.HandleFunc("POST /api/follow-up-rules/preview", authMiddleware(previewFollowupRule))
	m.HandleFunc("POST /api/follow-up-rules", authMiddleware(saveFollowupRule))
	m.HandleFunc("POST /api/follow-up-rules/{id}/preview", authMiddleware(refreshFollowupPreview))
	m.HandleFunc("PATCH /api/follow-up-rules/{id}", authMiddleware(toggleFollowupRule))
	m.HandleFunc("PUT /api/follow-up-rules/{id}", authMiddleware(editFollowupRule))
	m.HandleFunc("DELETE /api/follow-up-rules/{id}", authMiddleware(deleteFollowupRule))
	m.HandleFunc("GET /api/follow-up-history", authMiddleware(listFollowupHistory))
	m.HandleFunc("GET /api/cases/{id}/stage-history", authMiddleware(caseStageHistory))
	m.HandleFunc("GET /api/students/{id}/application-history", authMiddleware(studentApplicationHistory))
	m.HandleFunc("GET /api/application-queue", authMiddleware(applicationActionQueue))
	m.HandleFunc("GET /api/data-quality", authMiddleware(handleDataQuality))
}

func validateFollowupRule(rule followupRule) string {
	if strings.TrimSpace(rule.Name) == "" || len(rule.Name) > 100 {
		return "Enter a rule name up to 100 characters"
	}
	if rule.Entity != "cases" && rule.Entity != "leads" && rule.Entity != "students" {
		return "Choose cases, leads, or students"
	}
	if rule.InactiveDays < 1 || rule.InactiveDays > 365 {
		return "Inactive days must be between 1 and 365"
	}
	if strings.TrimSpace(rule.TaskTitle) == "" || len(rule.TaskTitle) > 200 {
		return "Enter a task title up to 200 characters"
	}
	if len(rule.Description) > 2000 {
		return "Task instructions must be 2,000 characters or fewer"
	}
	return ""
}

type rowQueryer interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

func newerDate(a, b string) string {
	if a > b {
		return a
	}
	return b
}

func ruleAuditTouches(db rowQueryer, entity, org string, saved diskStore) (map[string]string, error) {
	touches := map[string]string{}
	rows, err := db.Query("SELECT record_id,occurred_at FROM audit_events WHERE org_id=? AND entity=? ORDER BY id DESC", org, entity)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, at string
		if rows.Scan(&id, &at) == nil && touches[id] == "" {
			touches[id] = at
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if entity == "cases" || entity == "students" {
		caseRows, err := db.Query("SELECT case_id,contact_id,created_at FROM conversations WHERE org_id=?", org)
		if err != nil {
			return nil, err
		}
		caseTouches := map[string]string{}
		contactCases := map[string][]string{}
		for _, c := range saved.Cases {
			if c.OrgID == org && c.CurrentStage != "Approved" && c.CurrentStage != "Closed" {
				contactCases[c.ClientID] = append(contactCases[c.ClientID], c.ID)
			}
		}
		for caseRows.Next() {
			var caseID, contactID, at string
			if caseRows.Scan(&caseID, &contactID, &at) == nil {
				if caseID == "" && len(contactCases[contactID]) == 1 {
					caseID = contactCases[contactID][0]
				}
				if caseID != "" {
					caseTouches[caseID] = newerDate(at, caseTouches[caseID])
				}
			}
		}
		if err := caseRows.Err(); err != nil {
			caseRows.Close()
			return nil, err
		}
		caseRows.Close()
		for _, activity := range saved.Activities {
			if activity.OrgID == org && activity.ContactID != "" && len(contactCases[activity.ContactID]) == 1 {
				caseID := contactCases[activity.ContactID][0]
				caseTouches[caseID] = newerDate(activity.OccurredAt, caseTouches[caseID])
			}
		}
		for id, at := range caseTouches {
			touches[id] = newerDate(touches[id], at)
		}
		if entity == "students" {
			for _, c := range saved.Cases {
				if c.OrgID == org && c.StudentID != "" {
					touches[c.StudentID] = newerDate(touches[c.StudentID], touches[c.ID])
				}
			}
		}
	}
	return touches, nil
}

func dayAge(value string) (int, bool) {
	if len(value) >= 10 {
		value = value[:10]
	}
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0, false
	}
	return int(time.Since(d).Hours() / 24), true
}

func followupMatches(rule followupRule, org string, saved diskStore, db rowQueryer) ([]followupMatch, error) {
	matches := []followupMatch{}
	touches, err := ruleAuditTouches(db, rule.Entity, org, saved)
	if err != nil {
		return nil, err
	}
	appendIfStale := func(id, name, stage, created string, scope RecordScope, contactID, owner string) {
		if scope.OrgID != org || (rule.Stage != "" && !strings.EqualFold(rule.Stage, stage)) {
			return
		}
		last := touches[id]
		if last == "" {
			last = created
		}
		age, ok := dayAge(last)
		if ok && age >= rule.InactiveDays {
			matches = append(matches, followupMatch{ID: id, Name: name, Entity: rule.Entity, LastTouch: last, Scope: scope, ContactID: contactID, Owner: owner})
		}
	}
	switch rule.Entity {
	case "cases":
		for _, c := range saved.Cases {
			if c.CurrentStage != "Closed" && c.CurrentStage != "Approved" {
				appendIfStale(c.ID, c.CaseNumber, c.CurrentStage, c.CreatedAt, c.RecordScope, c.ClientID, c.AssignedOfficer)
			}
		}
	case "leads":
		for _, lead := range saved.Leads {
			if lead.Status != "Converted" && lead.Status != "Closed" {
				appendIfStale(lead.ID, lead.Name, lead.Status, lead.CreatedAt, lead.RecordScope, lead.ConvertedContactID, lead.AssignedTo)
			}
		}
	case "students":
		for _, student := range saved.Students {
			if !strings.EqualFold(student.ApplicationStage, "Enrolled") {
				appendIfStale(student.ID, student.Name, student.ApplicationStage, student.CreatedAt, student.RecordScope, "", "")
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].LastTouch == matches[j].LastTouch {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].LastTouch < matches[j].LastTouch
	})
	return matches, nil
}

func followupPreviewToken(rule followupRule, org string, matches []followupMatch) string {
	rule.Enabled = false
	rule.PreviewToken = ""
	payload, _ := json.Marshal(struct {
		Org     string          `json:"org"`
		Rule    followupRule    `json:"rule"`
		Matches []followupMatch `json:"matches"`
	}{org, rule, matches})
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func followupPreviewFresh(rule followupRule) bool {
	expires, err := time.Parse(time.RFC3339, rule.PreviewExpiresAt)
	return err == nil && time.Now().Before(expires) && time.Until(expires) <= 5*time.Minute
}

func decodeFollowupRule(w http.ResponseWriter, r *http.Request) (followupRule, bool) {
	var rule followupRule
	if !decodeRequest(w, r, &rule) {
		return rule, false
	}
	if msg := validateFollowupRule(rule); msg != "" {
		writeError(w, 400, msg)
		return rule, false
	}
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Stage = strings.TrimSpace(rule.Stage)
	rule.TaskTitle = strings.TrimSpace(rule.TaskTitle)
	rule.Description = strings.TrimSpace(rule.Description)
	return rule, true
}

func (state *runtimeStore) previewFollowupRule(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rule, ok := decodeFollowupRule(w, r)
	if !ok {
		return
	}
	matches, err := followupMatches(rule, currentUser(r).OrgID, state.snapshot(), storeDB(r))
	if err != nil {
		writeError(w, 500, "Could not preview activity for this rule")
		return
	}
	preview := matches
	if len(preview) > 20 {
		preview = preview[:20]
	}
	rule.PreviewExpiresAt = time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	writeJSON(w, 200, map[string]any{"count": len(matches), "records": preview, "previewToken": followupPreviewToken(rule, currentUser(r).OrgID, matches), "previewExpiresAt": rule.PreviewExpiresAt})
}

func previewFollowupRule(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.previewFollowupRule(w, r)
}

func (state *runtimeStore) saveFollowupRule(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rule, ok := decodeFollowupRule(w, r)
	if !ok {
		return
	}
	if rule.PreviewToken == "" || !followupPreviewFresh(rule) {
		writeError(w, 409, "Preview this rule before saving it")
		return
	}
	matches, err := followupMatches(rule, currentUser(r).OrgID, state.snapshot(), storeDB(r))
	if err != nil {
		writeError(w, 500, "Could not verify the rule preview")
		return
	}
	if !hmac.Equal([]byte(rule.PreviewToken), []byte(followupPreviewToken(rule, currentUser(r).OrgID, matches))) {
		writeError(w, 409, "Records changed since the preview. Preview the rule again before saving.")
		return
	}
	// Rules are always saved disabled. Staff must inspect a preview before enabling.
	rule.Enabled = false
	id, err := entry(r, "followup_rule", "", rule, false)
	if err != nil {
		writeError(w, 500, "Could not save follow-up rule")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func saveFollowupRule(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.saveFollowupRule(w, r)
}

func (state *runtimeStore) listFollowupRules(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rows, err := queryObjects(r, "SELECT id,data FROM workspace_entries WHERE org_id=? AND category='followup_rule' ORDER BY created_at DESC", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load follow-up rules")
		return
	}
	for _, row := range rows {
		var rule followupRule
		json.Unmarshal([]byte(fmtString(row["data"])), &rule)
		matches, err := followupMatches(rule, currentUser(r).OrgID, state.snapshot(), storeDB(r))
		if err != nil {
			writeError(w, 500, "Could not calculate follow-up rule matches")
			return
		}
		row["count"] = len(matches)
	}
	writeJSON(w, 200, map[string]any{"rules": rows})
}

func listFollowupRules(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listFollowupRules(w, r)
}

func (state *runtimeStore) refreshFollowupPreview(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	var raw string
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='followup_rule'", r.PathValue("id"), currentUser(r).OrgID).Scan(&raw) != nil {
		writeError(w, 404, "Follow-up rule not found")
		return
	}
	var rule followupRule
	if json.Unmarshal([]byte(raw), &rule) != nil {
		writeError(w, 500, "Could not read follow-up rule")
		return
	}
	rule.Enabled = false
	matches, err := followupMatches(rule, currentUser(r).OrgID, state.snapshot(), storeDB(r))
	if err != nil {
		writeError(w, 500, "Could not preview activity for this rule")
		return
	}
	rule.PreviewExpiresAt = time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	rule.PreviewToken = followupPreviewToken(rule, currentUser(r).OrgID, matches)
	if _, err := storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=?", mustJSON(rule), utcNow(), r.PathValue("id"), currentUser(r).OrgID); err != nil {
		writeError(w, 500, "Could not save rule preview")
		return
	}
	preview := matches
	if len(preview) > 20 {
		preview = preview[:20]
	}
	writeJSON(w, 200, map[string]any{"count": len(matches), "records": preview, "previewToken": rule.PreviewToken, "previewExpiresAt": rule.PreviewExpiresAt})
}

func refreshFollowupPreview(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.refreshFollowupPreview(w, r)
}

func (state *runtimeStore) toggleFollowupRule(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	var req struct {
		Enabled      bool   `json:"enabled"`
		PreviewToken string `json:"previewToken"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	var raw string
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='followup_rule'", r.PathValue("id"), currentUser(r).OrgID).Scan(&raw) != nil {
		writeError(w, 404, "Follow-up rule not found")
		return
	}
	var rule followupRule
	if json.Unmarshal([]byte(raw), &rule) != nil {
		writeError(w, 500, "Could not read follow-up rule")
		return
	}
	if req.Enabled {
		matches, err := followupMatches(rule, currentUser(r).OrgID, state.snapshot(), storeDB(r))
		if err != nil {
			writeError(w, 500, "Could not verify the rule preview")
			return
		}
		want := followupPreviewToken(rule, currentUser(r).OrgID, matches)
		if !followupPreviewFresh(rule) || !hmac.Equal([]byte(req.PreviewToken), []byte(rule.PreviewToken)) || !hmac.Equal([]byte(req.PreviewToken), []byte(want)) {
			writeError(w, 409, "Records changed since the preview. Preview the rule again before enabling it.")
			return
		}
	}
	rule.Enabled = req.Enabled
	if _, err := storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=?", mustJSON(rule), utcNow(), r.PathValue("id"), currentUser(r).OrgID); err != nil {
		writeError(w, 500, "Could not update follow-up rule")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "enabled": rule.Enabled})
}

func toggleFollowupRule(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.toggleFollowupRule(w, r)
}

func editFollowupRule(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rule, ok := decodeFollowupRule(w, r)
	if !ok {
		return
	}
	if rule.Enabled {
		writeError(w, 400, "Edited rules must be previewed and enabled separately")
		return
	}
	rule.PreviewToken, rule.PreviewExpiresAt = "", ""
	res, err := storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=? AND category='followup_rule'", mustJSON(rule), utcNow(), r.PathValue("id"), currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not update follow-up rule")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Follow-up rule not found")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "needsPreview": true})
}

func deleteFollowupRule(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	// Retire the rule so existing execution history and task attribution remain intact.
	var raw string
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='followup_rule'", r.PathValue("id"), currentUser(r).OrgID).Scan(&raw) != nil {
		writeError(w, 404, "Follow-up rule not found")
		return
	}
	var rule followupRule
	if json.Unmarshal([]byte(raw), &rule) != nil {
		writeError(w, 500, "Could not read follow-up rule")
		return
	}
	rule.Enabled = false
	if _, err := storeDB(r).Exec("UPDATE workspace_entries SET category='followup_rule_retired',data=?,updated_at=? WHERE id=? AND org_id=?", mustJSON(rule), utcNow(), r.PathValue("id"), currentUser(r).OrgID); err != nil {
		writeError(w, 500, "Could not retire follow-up rule")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "retired"})
}

func listFollowupHistory(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rows, err := queryObjects(r, "SELECT e.id,e.record_id,e.data,e.created_at,r.data AS rule_data FROM workspace_entries e LEFT JOIN workspace_entries r ON r.id=substr(e.record_id,1,instr(e.record_id,':')-1) AND r.org_id=e.org_id AND r.category IN ('followup_rule','followup_rule_retired') WHERE e.org_id=? AND e.category='followup_execution' ORDER BY e.created_at DESC LIMIT 200", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load follow-up history")
		return
	}
	for _, row := range rows {
		key := fmtString(row["record_id"])
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			row["ruleId"] = parts[0]
			row["sourceRecordId"] = parts[1]
		}
		var rule followupRule
		json.Unmarshal([]byte(fmtString(row["rule_data"])), &rule)
		row["ruleName"] = rule.Name
		row["taskId"] = fmtString(row["data"])
		delete(row, "rule_data")
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}

func (state *runtimeStore) caseStageHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s := state.snapshot()
	var c *Case
	for i := range s.Cases {
		if s.Cases[i].ID == id && canRead(s.Cases[i].RecordScope, currentUser(r)) {
			c = &s.Cases[i]
			break
		}
	}
	if c == nil {
		writeError(w, 404, "Case not found")
		return
	}
	rows, err := queryObjects(r, "SELECT stage,entered_at FROM case_stage_events WHERE org_id=? AND case_id=? ORDER BY entered_at,id", currentUser(r).OrgID, id)
	if err != nil {
		writeError(w, 500, "Could not load stage history")
		return
	}
	writeJSON(w, 200, map[string]any{"caseId": id, "owner": c.AssignedOfficer, "currentStage": c.CurrentStage, "history": rows})
}

func caseStageHistory(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.caseStageHistory(w, r)
}

func (state *runtimeStore) studentApplicationHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s := state.snapshot()
	visible := false
	for _, st := range s.Students {
		if st.ID == id && st.OrgID == currentUser(r).OrgID && canRead(st.RecordScope, currentUser(r)) {
			visible = true
			break
		}
	}
	if !visible {
		writeError(w, 404, "Student not found")
		return
	}
	items := []map[string]any{}
	for _, c := range s.Cases {
		if c.StudentID != id || c.OrgID != currentUser(r).OrgID || !canRead(c.RecordScope, currentUser(r)) {
			continue
		}
		history, err := queryObjects(r, "SELECT stage,entered_at FROM case_stage_events WHERE org_id=? AND case_id=? ORDER BY entered_at,id", currentUser(r).OrgID, c.ID)
		if err != nil {
			writeError(w, 500, "Could not load application history")
			return
		}
		items = append(items, map[string]any{"caseId": c.ID, "caseNumber": c.CaseNumber, "stage": c.CurrentStage, "owner": c.AssignedOfficer, "history": history, "route": "/cases/" + c.ID})
	}
	writeJSON(w, 200, map[string]any{"studentId": id, "applications": items})
}

func studentApplicationHistory(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.studentApplicationHistory(w, r)
}

func (state *runtimeStore) applicationActionQueue(w http.ResponseWriter, r *http.Request) {
	s := state.snapshot()
	today := organizationToday(r)
	items := []map[string]any{}
	studentNames := map[string]string{}
	for _, student := range s.Students {
		if student.OrgID == currentUser(r).OrgID {
			studentNames[student.ID] = student.Name
		}
	}
	for _, c := range s.Cases {
		if c.OrgID != currentUser(r).OrgID || !canRead(c.RecordScope, currentUser(r)) || c.CurrentStage == "Approved" || c.CurrentStage == "Closed" {
			continue
		}
		problems := []string{}
		if c.NextDeadline != "" && c.NextDeadline < today {
			problems = append(problems, "Overdue deadline: "+c.NextDeadline)
		}
		if strings.TrimSpace(c.NextAction) == "" {
			problems = append(problems, "Next staff action not set")
		}
		for _, d := range s.Documents {
			if d.CaseID == c.ID && d.Required && !strings.EqualFold(d.Status, "Verified") {
				problems = append(problems, "Document: "+d.DocName+" ("+d.Status+")")
			}
		}
		if len(problems) > 0 {
			studentName := studentNames[c.StudentID]
			if studentName == "" {
				studentName = c.ClientName
			}
			items = append(items, map[string]any{"caseId": c.ID, "caseNumber": c.CaseNumber, "studentId": c.StudentID, "studentName": studentName, "stage": c.CurrentStage, "owner": c.AssignedOfficer, "nextAction": c.NextAction, "deadline": c.NextDeadline, "issues": strings.Join(problems, "; "), "route": "/cases/" + c.ID})
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "total": len(items)})
}

func applicationActionQueue(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.applicationActionQueue(w, r)
}

// runFollowupRules creates at most one open task per rule and source record.
// Each execution marker and its task are committed with the same database write.
func runFollowupRules() {
	runner, r, state, close, err := workerRuntime(context.Background(), "")
	if err != nil {
		followupAutomationFailures.Add(1)
		return
	}
	defer close()
	tx := runner.(*sql.Tx)
	_ = r
	rows, err := tx.Query("SELECT id,org_id,data FROM workspace_entries WHERE category='followup_rule'")
	if err != nil {
		followupAutomationFailures.Add(1)
		return
	}
	type savedRule struct {
		id, org string
		rule    followupRule
	}
	rules := []savedRule{}
	for rows.Next() {
		var item savedRule
		var data string
		if rows.Scan(&item.id, &item.org, &data) == nil && json.Unmarshal([]byte(data), &item.rule) == nil && item.rule.Enabled {
			rules = append(rules, item)
		}
	}
	rows.Close()
	if len(rules) == 0 {
		return
	}
	original, next := state.cloneStore(), state.cloneStore()
	created := []struct{ rule, record, org, task string }{}
	for _, item := range rules {
		matches, err := followupMatches(item.rule, item.org, next, tx)
		if err != nil {
			followupAutomationFailures.Add(1)
			continue
		}
		for _, match := range matches {
			key := item.id + ":" + match.ID
			var exists int
			if tx.QueryRow("SELECT count(*) FROM workspace_entries WHERE org_id=? AND category='followup_execution' AND record_id=?", item.org, key).Scan(&exists) != nil || exists > 0 {
				continue
			}
			ownerName := match.Owner
			if ownerName == "" {
				ownerName = "Unassigned"
			}
			contactName := ""
			for _, contact := range next.Contacts {
				if contact.ID == match.ContactID {
					contactName = contact.Name
					break
				}
			}
			title := strings.ReplaceAll(item.rule.TaskTitle, "{{record}}", match.Name)
			description := strings.TrimSpace(item.rule.Description + "\n\nCreated by follow-up rule: " + item.rule.Name + " · " + match.Entity + " " + match.ID + " · Last updated " + match.LastTouch)
			taskID := newID("t")
			next.Tasks = append(next.Tasks, Task{RecordScope: match.Scope, ID: taskID, Title: title, Description: description, Status: "todo", DueDate: todayForOrgDB(tx, item.org), ReminderDate: todayForOrgDB(tx, item.org), ContactID: match.ContactID, ContactName: contactName, Owner: ownerName, CreatedAt: todayForOrgDB(tx, item.org)})
			created = append(created, struct{ rule, record, org, task string }{item.id, match.ID, item.org, taskID})
		}
	}
	if len(created) == 0 {
		return
	}
	state.restoreStore(next)
	events := changesBetween(original, state.snapshot(), "Follow-up automation")
	err = saveStoreDeltaTx(tx, original, state.snapshot(), events)
	if err == nil {
		for _, item := range created {
			if _, err = tx.Exec("INSERT INTO workspace_entries VALUES(?,?, 'followup_execution','',?,?,?,?)", newID("entry"), item.org, item.rule+":"+item.record, mustJSON(map[string]string{"taskId": item.task}), utcNow(), utcNow()); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		tx.Rollback()
	}
	if err != nil {
		followupAutomationFailures.Add(1)
		state.restoreStore(original)
	}
}

func organizationTodayForOrg(org string) string {
	var zone string
	database.QueryRow("SELECT timezone FROM organizations WHERE id=?", org).Scan(&zone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	return time.Now().In(location).Format("2006-01-02")
}

type qualityFinding struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	RecordID string `json:"recordId"`
	Route    string `json:"route"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

func (state *runtimeStore) handleDataQuality(w http.ResponseWriter, r *http.Request) {
	s := state.snapshot()
	org := currentUser(r).OrgID
	f := []qualityFinding{}
	add := func(entity, id, severity, title, detail string, scope RecordScope) {
		if scope.OrgID == org && canRead(scope, currentUser(r)) {
			route := "/" + entity + "/" + id
			if entity == "documents" {
				for _, document := range s.Documents {
					if document.ID == id {
						route = "/cases/" + document.CaseID
						break
					}
				}
			}
			f = append(f, qualityFinding{ID: entity + ":" + id + ":" + title, Entity: entity, RecordID: id, Route: route, Severity: severity, Title: title, Detail: detail})
		}
	}
	schools, agents, linkedStudents := map[string]bool{}, map[string]bool{}, map[string]bool{}
	studentContacts := map[string][]string{}
	studentsByID := map[string]Student{}
	for _, st := range s.Students {
		if st.OrgID == org {
			studentsByID[st.ID] = st
		}
	}
	for _, x := range s.Schools {
		if x.OrgID == org {
			schools[x.ID] = true
		}
	}
	for _, x := range s.Agents {
		if x.OrgID == org {
			agents[x.ID] = true
		}
	}
	for _, c := range s.Cases {
		if c.OrgID == org {
			if c.StudentID != "" {
				linkedStudents[c.StudentID] = true
				if c.ClientID != "" {
					studentContacts[c.StudentID] = append(studentContacts[c.StudentID], c.ClientID)
				}
			}
			if c.CurrentStage != "Approved" && c.CurrentStage != "Closed" && c.NextAction == "" {
				add("cases", c.ID, "warning", "No next action", c.CaseNumber+" has no next step recorded.", c.RecordScope)
			}
		}
	}
	for _, st := range s.Students {
		if st.OrgID != org {
			continue
		}
		if st.SchoolID != "" && !schools[st.SchoolID] {
			add("students", st.ID, "error", "School link is missing", st.Name+" points to a school that no longer exists.", st.RecordScope)
		}
		if st.AgentID != "" && !agents[st.AgentID] {
			add("students", st.ID, "error", "Agent link is missing", st.Name+" points to an agent that no longer exists.", st.RecordScope)
		}
		if st.SchoolID == "" && st.Program != "" {
			add("students", st.ID, "warning", "School link is missing", st.Name+" has a program but no school linked.", st.RecordScope)
		}
		if st.ApplicationStage != "Enrolled" {
			if !linkedStudents[st.ID] {
				add("students", st.ID, "warning", "No active application case", st.Name+" has no active linked case.", st.RecordScope)
			}
		}
	}
	studentGroups := map[string][]Student{}
	for _, st := range s.Students {
		if st.OrgID != org {
			continue
		}
		key := ""
		if strings.TrimSpace(st.StudentCode) != "" {
			key = "code:" + strings.ToLower(strings.TrimSpace(st.StudentCode))
		} else if st.SchoolID != "" && st.Program != "" {
			key = "application:" + strings.ToLower(strings.TrimSpace(st.Name)) + ":" + st.SchoolID + ":" + strings.ToLower(strings.TrimSpace(st.Program))
		}
		if key != "" {
			studentGroups[key] = append(studentGroups[key], st)
		}
	}
	for _, group := range studentGroups {
		if len(group) < 2 {
			continue
		}
		for _, st := range group {
			add("students", st.ID, "warning", "Possible duplicate student", st.Name+" shares a student code or application identity with another record. Review before merging.", st.RecordScope)
		}
	}
	contacts := map[string]Contact{}
	for _, c := range s.Contacts {
		if c.OrgID == org {
			contacts[c.ID] = c
		}
	}
	missingContactReported := map[string]bool{}
	for studentID, contactIDs := range studentContacts {
		if missingContactReported[studentID] {
			continue
		}
		hasContactDetails := false
		for _, contactID := range contactIDs {
			contact, exists := contacts[contactID]
			if exists && (contact.Email != "" || contact.Phone != "") {
				hasContactDetails = true
				break
			}
		}
		if !hasContactDetails {
			if student, exists := studentsByID[studentID]; exists {
				add("students", student.ID, "warning", "Student contact details missing", student.Name+" has no email address or phone number on the linked contact.", student.RecordScope)
				missingContactReported[student.ID] = true
			}
		}
	}
	for _, d := range s.Documents {
		if d.OrgID == org && d.Required && !strings.EqualFold(d.Status, "Verified") {
			severity, title := "warning", "Required document incomplete"
			if strings.EqualFold(d.Status, "Expired") {
				severity, title = "error", "Required document expired"
			}
			add("documents", d.ID, severity, title, d.DocName+" is "+d.Status+" for "+d.CaseNumber+".", d.RecordScope)
		}
	}
	for _, l := range s.Leads {
		if l.OrgID == org && l.Status != "Converted" && l.Status != "Closed" && l.FollowUpDate == "" {
			add("leads", l.ID, "warning", "Follow-up date missing", l.Name+" has no follow-up date.", l.RecordScope)
		}
	}
	for _, c := range s.Cases {
		if c.OrgID == org && c.CurrentStage != "Approved" && c.CurrentStage != "Closed" && c.NextDeadline != "" && c.NextDeadline < organizationToday(r) {
			add("cases", c.ID, "error", "Deadline is overdue", c.CaseNumber+" passed its deadline on "+c.NextDeadline+".", c.RecordScope)
		}
	}
	sort.Slice(f, func(i, j int) bool {
		if f[i].Severity != f[j].Severity {
			return f[i].Severity == "error"
		}
		return f[i].Title < f[j].Title
	})
	totalAll := len(f)
	severity, entity, search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("severity"))), strings.ToLower(strings.TrimSpace(r.URL.Query().Get("entity"))), strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	filtered := make([]qualityFinding, 0, len(f))
	for _, finding := range f {
		if severity != "" && severity != "all" && severity != strings.ToLower(finding.Severity) {
			continue
		}
		if entity != "" && entity != "all" && entity != strings.ToLower(finding.Entity) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(finding.Title+" "+finding.Detail+" "+finding.Entity+" "+finding.RecordID), search) {
			continue
		}
		filtered = append(filtered, finding)
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	writeJSON(w, 200, map[string]any{"findings": filtered[start:end], "total": total, "totalAll": totalAll, "page": page, "pageSize": pageSize, "pages": (total + pageSize - 1) / pageSize})
}

func handleDataQuality(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleDataQuality(w, r)
}
