package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ApplicationBlocker struct {
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Action   string `json:"action"`
	Route    string `json:"route"`
	Blocking bool   `json:"blocking"`
}

func readCaseRows[T any](r *http.Request, entity, where string, args ...any) ([]T, error) {
	clean := r.Clone(r.Context())
	url := *r.URL
	url.RawQuery = ""
	clean.URL = &url
	scope, scopeArgs := repositoryPredicate(clean, entity)
	rows, err := storeDB(r).Query("SELECT record.data FROM "+entity+" record WHERE "+scope+" AND "+where+" ORDER BY record.sort_order,record.id", append(scopeArgs, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		var item T
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func handleApplicationBlockers(w http.ResponseWriter, r *http.Request) {
	tx, err := database.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 503, "Could not load application requirements")
		return
	}
	defer tx.Rollback()
	r = withDBTransaction(r, tx)
	record, err := sqlRecord(r, "cases", r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "Application not found")
		return
	}
	var c Case
	json.Unmarshal([]byte(mustJSON(record)), &c)
	docs, err := readCaseRows[Document](r, "documents", "json_extract(record.data,'$.caseId')=?", c.ID)
	if err != nil {
		writeError(w, 500, "Could not load documents")
		return
	}
	invoices, err := readCaseRows[Invoice](r, "invoices", "json_extract(record.data,'$.caseId')=?", c.ID)
	if err != nil {
		writeError(w, 500, "Could not load invoices")
		return
	}
	today := organizationToday(r)
	out := []ApplicationBlocker{}
	for _, d := range docs {
		if !d.Required || (!documentNeedsReview(d, today) && strings.EqualFold(d.Status, "Verified")) {
			continue
		}
		action := "Review document"
		detail := d.Status
		if d.Status == "Requested" || d.Status == "Not Requested" || d.Status == "Correction Required" || d.LatestVersion == 0 && !strings.EqualFold(d.Status, "Verified") {
			action = "Upload document"
		}
		if d.ExpiryDate != "" && d.ExpiryDate < today {
			detail = "Expired on " + d.ExpiryDate
			action = "Upload replacement"
		}
		out = append(out, ApplicationBlocker{"document", d.DocName, detail, action, "/cases/" + c.ID + "#record-tools?tab=documents&document=" + d.ID, c.AutoAdvance && !strings.EqualFold(d.Status, "Verified")})
	}
	for _, inv := range invoices {
		route := "/invoices/" + inv.ID
		if inv.Balance > 0 {
			out = append(out, ApplicationBlocker{"payment", inv.InvoiceNumber, fmt.Sprintf("Outstanding balance: %s %.2f", currencyOf(inv.Currency), inv.Balance), "Review balance and receipts", route, false})
		}
		var pending int
		if err = storeDB(r).QueryRow("SELECT count(*) FROM payments WHERE json_extract(data,'$.orgId')=? AND json_extract(data,'$.invoiceId')=? AND json_extract(data,'$.status')='pending_approval'", inv.OrgID, inv.ID).Scan(&pending); err != nil {
			writeError(w, 500, "Could not load pending approvals")
			return
		}
		if pending > 0 {
			out = append(out, ApplicationBlocker{"approval", inv.InvoiceNumber, "Receipt approval is pending; these receipts do not reduce the posted balance.", "Review pending receipts", route + "#receipt-approvals", false})
		}
		if inv.RefundStatus == "Requested" || inv.RefundStatus == "Approved" {
			out = append(out, ApplicationBlocker{"refund", inv.InvoiceNumber, "Refund " + strings.ToLower(inv.RefundStatus), "Review refund", route + "#record-tools?tab=workflows", false})
		}
	}
	history, err := handoffHistory(r, c.ID)
	if err != nil {
		writeError(w, 500, "Could not load handoffs")
		return
	}
	for _, h := range history {
		if h.Status == "pending" {
			detail := "Awaiting " + h.ToName + "; the current owner remains responsible."
			if h.DueDate < today {
				detail = "Overdue handoff. " + detail
			}
			out = append(out, ApplicationBlocker{"handoff", "Ownership handoff", detail, "Review handoff", "/cases/" + c.ID + "#handoff", false})
		}
	}
	blocking := 0
	for _, b := range out {
		if b.Blocking {
			blocking++
		}
	}
	writeJSON(w, 200, map[string]any{"items": out, "blockingCount": blocking, "autoAdvance": c.AutoAdvance, "scope": "Accessible application records"})
}
