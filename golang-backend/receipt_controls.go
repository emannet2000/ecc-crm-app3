package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func receiptActive(p Payment) bool {
	return p.Status != "void" && p.Status != "refund" && p.Status != "refund_reversal"
}
func receiptRisk(req paymentRequest, payments []Payment) (exact, possible bool) {
	for _, p := range payments {
		if p.InvoiceID != strings.TrimSpace(req.InvoiceID) || !receiptActive(p) {
			continue
		}
		if strings.TrimSpace(req.Reference) != "" && strings.EqualFold(strings.TrimSpace(req.Reference), strings.TrimSpace(p.Reference)) {
			exact = true
		}
		if money(req.Amount) == money(p.Amount) && normalizeDate(req.PaidOn) == p.PaidOn && strings.EqualFold(strings.TrimSpace(req.Method), strings.TrimSpace(p.Method)) {
			possible = true
		}
	}
	return
}
func pendingReceipt(p Payment) bool { return p.Status == "pending_approval" }

func pendingInvoiceReceiptCounts(r *http.Request) (map[string]int, error) {
	scope, args := scopeSQL(r, "invoice.data")
	args = append(args, currentUser(r).OrgID)
	rows, err := storeDB(r).Query("SELECT invoice.id,count(*) FROM invoices invoice JOIN payments p ON json_extract(p.data,'$.invoiceId')=invoice.id WHERE "+scope+" AND json_extract(p.data,'$.orgId')=? AND json_extract(p.data,'$.status')='pending_approval' GROUP BY invoice.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var count int
		if err = rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		out[id] = count
	}
	return out, rows.Err()
}

func handleReceiptReview(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager approval is required")
		return
	}
	var in struct{ Decision string }
	if !decodeRequest(w, r, &in) {
		return
	}
	if in.Decision != "approve" && in.Decision != "reject" {
		writeError(w, 400, "Choose approve or reject")
		return
	}
	state, done := runtimeForCall(r)
	defer done()
	state.mu.Lock()
	defer state.mu.Unlock()
	for i, p := range state.payments {
		if p.ID != r.PathValue("id") {
			continue
		}
		if !pendingReceipt(p) {
			writeError(w, 409, "This receipt has already been reviewed")
			return
		}
		index := -1
		for j, inv := range state.invoices {
			if inv.ID == p.InvoiceID && canWrite(inv.RecordScope, currentUser(r)) {
				index = j
				break
			}
		}
		if index < 0 {
			writeError(w, 404, "Invoice not found")
			return
		}
		state.recomputeInvoiceLocked(index)
		status := "posted"
		if in.Decision == "reject" {
			status = "void"
		}
		state.payments[i].Status = status
		state.payments[i].ReviewedBy = currentUser(r).ID
		state.payments[i].ReviewedAt = utcNow()
		_, err := entry(r, "receipt_review", p.InvoiceID, map[string]any{"paymentId": p.ID, "decision": in.Decision, "reviewedBy": currentUser(r).ID, "reviewedAt": utcNow()}, false)
		if err != nil {
			writeError(w, 500, "Could not record receipt review")
			return
		}
		if _, err = storeDB(r).Exec("UPDATE notifications SET read_at=COALESCE(read_at,?) WHERE org_id=? AND dedupe_key=?", utcNow(), p.OrgID, "receipt:"+p.ID); err != nil {
			writeError(w, 500, "Could not resolve receipt notifications")
			return
		}
		state.recomputeInvoiceLocked(index)
		writeJSON(w, 200, map[string]any{"payment": state.payments[i], "invoice": state.invoices[index]})
		return
	}
	writeError(w, 404, "Receipt not found")
}

func handleInvoiceFinanceStatus(w http.ResponseWriter, r *http.Request) {
	tx, err := database.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 503, "Could not load invoice balance")
		return
	}
	defer tx.Rollback()
	r = withDBTransaction(r, tx)
	record, err := sqlRecord(r, "invoices", r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "Invoice not found")
		return
	}
	var inv Invoice
	if json.Unmarshal([]byte(mustJSON(record)), &inv) != nil {
		writeError(w, 500, "Could not read invoice")
		return
	}
	payments, err := readCaseRows[Payment](r, "payments", "json_extract(record.data,'$.invoiceId')=?", inv.ID)
	if err != nil {
		writeError(w, 500, "Could not load receipts")
		return
	}
	pending := []Payment{}
	for _, p := range payments {
		if pendingReceipt(p) {
			pending = append(pending, p)
		}
	}
	// Balances are invoice aggregates; pending totals include the full linked
	// ledger while receipt identities remain scoped to the current user.
	var amount float64
	var count int
	err = storeDB(r).QueryRow("SELECT COALESCE(sum(json_extract(data,'$.amount')),0),count(*) FROM payments WHERE json_extract(data,'$.orgId')=? AND json_extract(data,'$.invoiceId')=? AND json_extract(data,'$.status')='pending_approval'", inv.OrgID, inv.ID).Scan(&amount, &count)
	if err != nil {
		writeError(w, 500, "Could not load approval totals")
		return
	}
	writeJSON(w, 200, map[string]any{"invoice": inv, "pendingReceipts": pending, "pendingAmount": money(amount), "pendingCount": count, "canReview": isManager(currentUser(r)), "approvalNote": inv.PaymentApproval})
}
