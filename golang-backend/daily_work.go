package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

func independentDailyRead(r *http.Request) bool {
	p := r.URL.Path
	return (r.Method == "GET" && (p == "/api/usability/feedback" || p == "/api/my-work" || p == "/api/integrations/status" || strings.HasSuffix(p, "/blockers") && strings.HasPrefix(p, "/api/cases/") || strings.HasSuffix(p, "/finance-status") && strings.HasPrefix(p, "/api/invoices/") || strings.HasSuffix(p, "/journey") && strings.HasPrefix(p, "/api/students/"))) || (r.Method == "POST" && (p == "/api/usability/feedback" || p == "/api/form-quality" || strings.HasPrefix(p, "/api/admin/integrations/") && strings.HasSuffix(p, "/test")))
}
func registerDailyRoutes(m *http.ServeMux) {
	registerHandoffRoutes(m)
	m.HandleFunc("GET /api/cases/{id}/blockers", authMiddleware(handleApplicationBlockers))
	m.HandleFunc("GET /api/invoices/{id}/finance-status", authMiddleware(handleInvoiceFinanceStatus))
	m.HandleFunc("POST /api/payments/{id}/review", authMiddleware(handleReceiptReview))
	m.HandleFunc("GET /api/usability/feedback", authMiddleware(handleUsabilityFeedback))
	m.HandleFunc("POST /api/usability/feedback", authMiddleware(dailyOrigin(handleUsabilityFeedback)))
	m.HandleFunc("GET /api/my-work", authMiddleware(handleMyWork))
	m.HandleFunc("GET /api/students/{id}/journey", authMiddleware(handleStudentJourney))
	m.HandleFunc("POST /api/form-quality", authMiddleware(dailyOrigin(handleFormQuality)))
	m.HandleFunc("GET /api/integrations/status", authMiddleware(handleIntegrationHealth))
	m.HandleFunc("POST /api/admin/integrations/{integration}/test", authMiddleware(dailyOrigin(handleIntegrationTest)))
}

// Keep the records, related messages and totals on a single committed SQL snapshot.
func dailySnapshot(w http.ResponseWriter, r *http.Request) (*http.Request, diskStore, func(), bool) {
	tx, err := database.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		writeError(w, 503, "Database unavailable; try Refresh again")
		return r, diskStore{}, func() {}, false
	}
	r = r.WithContext(context.WithValue(r.Context(), requestTxKey, tx))
	s, err := sqlWorkspace(r)
	if err != nil {
		tx.Rollback()
		writeError(w, 500, "Could not load workspace records")
		return r, s, func() {}, false
	}
	return r, s, func() { tx.Rollback() }, true
}
func handleMyWork(w http.ResponseWriter, r *http.Request) {
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	filter := r.URL.Query().Get("scope")
	if filter == "" {
		filter = "mine"
	}
	if filter != "mine" && filter != "all" && filter != "unassigned" {
		writeError(w, 400, "Choose mine, unassigned or all")
		return
	}
	today := organizationToday(r)
	items := collectWorkload(currentUser(r), s, today, filter, true)
	pendingApprovals := map[string]int{}
	approvalCount := 0
	// Invoice amounts are kept per invoice/currency rather than adding unlike currencies.
	if r.URL.Query().Get("focus") == "finance" {
		var err error
		pendingApprovals, err = pendingInvoiceReceiptCounts(r)
		if err != nil {
			writeError(w, 500, "Could not load receipt approval totals")
			return
		}
		for _, inv := range s.Invoices {
			if inv.Balance <= 0 && pendingApprovals[inv.ID] == 0 || !workAssigned(inv.OwnerID, inv.RecordScope, currentUser(r), filter) {
				continue
			}
			priority, reason, rank := "medium", "Outstanding invoice", 3
			if days, valid := dateDistance(inv.DueDate, today); valid && days < 0 {
				priority, reason, rank = "urgent", "Invoice is overdue", 0
			}
			if pendingApprovals[inv.ID] > 0 {
				reason = "Receipts awaiting approval"
				priority = "high"
				if rank > 1 {
					rank = 1
				}
				approvalCount += pendingApprovals[inv.ID]
			}
			items = append(items, productivityItem{ID: inv.ID, Entity: "invoice", Name: inv.InvoiceNumber, Reason: reason, Action: "Review balance and recorded payments", Deadline: inv.DueDate, Priority: priority, Route: "/invoices/" + inv.ID, Rank: rank})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rank != items[j].Rank {
			return items[i].Rank < items[j].Rank
		}
		return items[i].Deadline != "" && (items[j].Deadline == "" || items[i].Deadline < items[j].Deadline)
	})
	handoffCount, overdueHandoffs := 0, 0
	visibleCases := map[string]Case{}
	for _, c := range s.Cases {
		visibleCases[c.ID] = c
	}
	rows, err := storeDB(r).Query("SELECT data FROM workspace_entries WHERE org_id=? AND category='application_handoff'", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load pending handoffs")
		return
	}
	for rows.Next() {
		var raw string
		var h ApplicationHandoff
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &h); err != nil {
			break
		}
		c, visible := visibleCases[h.CaseID]
		if !visible || h.Status != "pending" || filter == "unassigned" || (filter == "mine" && h.ToID != currentUser(r).ID && h.FromID != currentUser(r).ID) {
			continue
		}
		reason, priority := "Handoff awaiting "+h.ToName, "high"
		if h.DueDate < today {
			reason = "Overdue handoff awaiting " + h.ToName
			priority = "urgent"
			overdueHandoffs++
		}
		items = append(items, productivityItem{ID: h.ID, Entity: "handoff", Name: c.CaseNumber, Reason: reason, Action: h.NextAction, Deadline: h.DueDate, Priority: priority, Route: "/cases/" + c.ID + "#handoff", Rank: 0})
		handoffCount++
	}
	if rows.Err() != nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeError(w, 500, "Could not load pending handoffs")
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Rank < items[j].Rank })
	counts := map[string]int{"applications": 0, "followups": 0, "documents": 0, "deadlines": 0, "tasks": 0, "invoices": 0, "handoffs": handoffCount, "overdueHandoffs": overdueHandoffs, "pendingApprovals": approvalCount}
	documentsByCase := map[string]bool{}
	for _, doc := range s.Documents {
		if documentNeedsReview(doc, today) {
			documentsByCase[doc.CaseID] = true
		}
	}
	for _, item := range items {
		switch item.Entity {
		case "case":
			counts["applications"]++
			if documentsByCase[item.ID] {
				counts["documents"]++
			}
		case "lead":
			counts["followups"]++
		case "task":
			counts["tasks"]++
		case "invoice":
			counts["invoices"]++
		}
		if days, valid := dateDistance(item.Deadline, today); valid && days <= 7 {
			counts["deadlines"]++
		}
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	writeJSON(w, 200, map[string]any{"items": slicePage(items, offset, limit), "total": len(items), "counts": counts, "today": today, "scope": filter, "limit": limit, "offset": offset})
}
func handleStudentJourney(w http.ResponseWriter, r *http.Request) {
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	var student Student
	found := false
	for _, st := range s.Students {
		if st.ID == r.PathValue("id") {
			student = st
			found = true
			break
		}
	}
	if !found {
		writeError(w, 404, "Student not found")
		return
	}
	cases := []Case{}
	documents := []Document{}
	invoices := []Invoice{}
	payments := []Payment{}
	messages := []map[string]any{}
	linked := map[string]bool{}
	invoiceIDs := map[string]bool{}
	for _, c := range s.Cases {
		if c.StudentID == student.ID {
			cases = append(cases, c)
			linked[c.ID] = true
		}
	}
	required, verified := 0, 0
	for _, d := range s.Documents {
		if linked[d.CaseID] {
			documents = append(documents, d)
			if d.Required {
				required++
				if strings.EqualFold(d.Status, "Verified") && !documentNeedsReview(d, organizationToday(r)) {
					verified++
				}
			}
		}
	}
	for _, inv := range s.Invoices {
		if linked[inv.CaseID] {
			invoices = append(invoices, inv)
			invoiceIDs[inv.ID] = true
		}
	}
	for _, p := range s.Payments {
		if invoiceIDs[p.InvoiceID] {
			payments = append(payments, p)
		}
	}
	// Only explicitly case-linked messages enter the student overview. Shared contacts
	// may have other applications; those messages must not spill into this student.
	rows, err := queryObjects(r, "SELECT id,case_id AS caseId,channel,direction,subject,body,status,created_at AS createdAt FROM conversations WHERE org_id=? ORDER BY created_at DESC,id", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load application messages")
		return
	}
	totalMessages := 0
	for _, row := range rows {
		if linked[fmtString(row["caseId"])] {
			totalMessages++
			if len(messages) < 25 {
				messages = append(messages, row)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"student": student, "applications": cases, "documents": documents, "invoices": invoices, "payments": payments, "messages": messages, "messageTotal": totalMessages, "required": required, "verified": verified})
}

type formQualityRequest struct {
	Entity       string `json:"entity"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	StudentCode  string `json:"studentCode"`
	SchoolID     string `json:"schoolId"`
	Program      string `json:"program"`
	StudentID    string `json:"studentId"`
	ClientID     string `json:"clientId"`
	FollowUpDate string `json:"followUpDate"`
	Status       string `json:"status"`
	NextAction   string `json:"nextAction"`
	NextDeadline string `json:"nextDeadline"`
	CurrentStage string `json:"currentStage"`
}

func qualityKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func formQuality(s diskStore, in formQualityRequest, today string) []qualityFinding {
	findings := []qualityFinding{}
	add := func(title, detail, entity, id string) {
		route := ""
		if id != "" {
			route = "/" + entity + "/" + id
		}
		findings = append(findings, qualityFinding{Title: title, Detail: detail, Severity: "warning", Entity: entity, RecordID: id, Route: route})
	}
	switch in.Entity {
	case "students":
		for _, st := range s.Students {
			if st.ID == in.ID {
				continue
			}
			sameCode := qualityKey(in.StudentCode) != "" && qualityKey(st.StudentCode) == qualityKey(in.StudentCode)
			sameApplication := qualityKey(in.Name) != "" && qualityKey(in.Program) != "" && in.SchoolID != "" && qualityKey(st.Name) == qualityKey(in.Name) && st.SchoolID == in.SchoolID && qualityKey(st.Program) == qualityKey(in.Program)
			if sameCode || sameApplication {
				add("Possible duplicate student", "Review "+st.Name+" before creating another student record.", "students", st.ID)
			}
		}
		if strings.TrimSpace(in.Program) != "" && in.SchoolID == "" {
			add("Choose a school", "Link this program to a school so the application can be tracked.", "students", "")
		}
	case "leads":
		if in.Status != "Converted" && in.Status != "Closed" && in.FollowUpDate == "" {
			add("Set a follow-up date", "Choose when to contact this lead next.", "leads", "")
		}
		for _, l := range s.Leads {
			if l.ID == in.ID {
				continue
			}
			if qualityKey(in.Email) != "" && qualityKey(l.Email) == qualityKey(in.Email) || normalizePhone(in.Phone) != "" && normalizePhone(l.Phone) == normalizePhone(in.Phone) {
				add("Possible duplicate lead", "Review "+l.Name+" with matching contact details.", "leads", l.ID)
			}
		}
	case "cases":
		if in.CurrentStage != "Approved" && in.CurrentStage != "Closed" {
			if strings.TrimSpace(in.NextAction) == "" {
				add("Set the next action", "Record the next staff action before saving this application.", "cases", "")
			}
			if in.NextDeadline == "" {
				add("Set a deadline", "Choose a deadline for the next staff action.", "cases", "")
			} else if days, valid := dateDistance(in.NextDeadline, today); valid && days < 0 {
				add("Deadline is overdue", "This deadline has already passed; confirm the intended date.", "cases", "")
			}
			if in.StudentID == "" {
				add("Link a student", "Link this case to a student to include it in their journey.", "cases", "")
			}
			for _, d := range s.Documents {
				if d.CaseID == in.ID && documentNeedsReview(d, today) {
					add("Required document incomplete", d.DocName+" is "+d.Status+". Review it in application tools.", "cases", in.ID)
				}
			}
		}
	}
	return findings
}
func handleFormQuality(w http.ResponseWriter, r *http.Request) {
	var in formQualityRequest
	if !decodeRequest(w, r, &in) {
		return
	}
	if in.Entity != "students" && in.Entity != "cases" && in.Entity != "leads" {
		writeError(w, 400, "Choose students, cases or leads")
		return
	}
	r, s, close, ok := dailySnapshot(w, r)
	if !ok {
		return
	}
	defer close()
	if in.ID != "" {
		exists := false
		for id := range recordsOf(s)[in.Entity] {
			if id == in.ID {
				exists = true
				break
			}
		}
		if !exists {
			writeError(w, 404, "Record not found")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"findings": formQuality(s, in, organizationToday(r))})
}

func dailyOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validOrigin(r) {
			writeError(w, 403, "Cross-origin changes are not allowed")
			return
		}
		next(w, r)
	}
}
