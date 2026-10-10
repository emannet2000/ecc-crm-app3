package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type invoiceRequest struct {
	InvoiceNumber         string  `json:"invoiceNumber"`
	ClientID              string  `json:"clientId"`
	CaseID                string  `json:"caseId"`
	TotalFee              float64 `json:"totalFee"`
	GovernmentFee         float64 `json:"governmentFee"`
	SchoolPartnerFee      float64 `json:"schoolPartnerFee"`
	AmountReceived        float64 `json:"amountReceived"`
	PaymentMilestone      string  `json:"paymentMilestone"`
	PaymentMethod         string  `json:"paymentMethod"`
	OfficialReceiptNumber string  `json:"officialReceiptNumber"`
	RefundStatus          string  `json:"refundStatus"`
	ReferralCommission    float64 `json:"referralCommission"`
	PartnerPayable        float64 `json:"partnerPayable"`
	PaymentApproval       string  `json:"paymentApproval"`
	Notes                 string  `json:"notes"`
}

type paymentRequest struct {
	RequireApproval  bool    `json:"requireApproval"`
	ConfirmDuplicate bool    `json:"confirmDuplicate"`
	InvoiceID        string  `json:"invoiceId"`
	Amount           float64 `json:"amount"`
	PaidOn           string  `json:"paidOn"`
	Method           string  `json:"method"`
	Reference        string  `json:"reference"`
	Notes            string  `json:"notes"`
}

type refundRequest struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

var validPaymentMilestones = []string{
	"Quotation Issued", "Deposit Due", "Deposit Paid",
	"Instalment Due", "Fully Paid", "Refund Review", "Refunded",
}

func isValidPaymentMilestone(s string) bool {
	for _, v := range validPaymentMilestones {
		if v == s {
			return true
		}
	}
	return false
}

// validateInvoice must be called with mu held.
func (state *runtimeStore) validateInvoice(req invoiceRequest) map[string]string {
	fields := map[string]string{}

	clientID := strings.TrimSpace(req.ClientID)
	milestone := strings.TrimSpace(req.PaymentMilestone)

	if clientID == "" {
		fields["clientId"] = "Client is required"
	} else if _, ok := state.findContactByID(clientID); !ok {
		fields["clientId"] = "Client not found"
	}

	if req.TotalFee < 0 {
		fields["totalFee"] = "Total fee can't be negative"
	}
	if req.GovernmentFee < 0 {
		fields["governmentFee"] = "Government fee can't be negative"
	}
	if req.SchoolPartnerFee < 0 {
		fields["schoolPartnerFee"] = "Partner fee can't be negative"
	}
	if req.AmountReceived < 0 {
		fields["amountReceived"] = "Amount received can't be negative"
	}

	if milestone == "" {
		fields["paymentMilestone"] = "Payment milestone is required"
	} else if !isValidPaymentMilestone(milestone) {
		fields["paymentMilestone"] = "Invalid milestone"
	}

	return fields
}

func validateInvoice(req invoiceRequest) map[string]string {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.validateInvoice(req)
}

// ================= INVOICES =================

func (state *runtimeStore) listInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
	milestone := strings.TrimSpace(r.URL.Query().Get("milestone"))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listInvoices()
	filtered := make([]Invoice, 0, len(all))
	for _, inv := range all {
		if clientID != "" && inv.ClientID != clientID {
			continue
		}
		if milestone != "" && inv.PaymentMilestone != milestone {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(inv.InvoiceNumber), q) ||
			strings.Contains(strings.ToLower(inv.ClientName), q) ||
			strings.Contains(strings.ToLower(inv.CaseNumber), q) {
			filtered = append(filtered, inv)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"invoices": slicePage(filtered, offset, limit),
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func listInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listInvoicesHandler(w, r)
}

func (state *runtimeStore) getInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, inv := range state.invoices {
		if inv.ID == id {
			writeJSON(w, http.StatusOK, map[string]any{"invoice": inv})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Invoice not found")
}

func getInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getInvoiceHandler(w, r)
}

func (state *runtimeStore) createInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	var req invoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email, _ := r.Context().Value(ctxUserEmail).(string)
	createdBy := "Unknown"
	if email != "" {
		if u, ok := state.findUserByEmail(email); ok {
			createdBy = u.Name
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if fields := state.validateInvoice(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	clientID := strings.TrimSpace(req.ClientID)
	clientName := ""
	if c, ok := state.findContactByID(clientID); ok {
		clientName = c.Name
	}

	caseID := strings.TrimSpace(req.CaseID)
	caseNumber := ""
	if c, ok := state.findCaseByID(caseID); ok {
		caseNumber = c.CaseNumber
	}

	now := time.Now().Format("2006-01-02")
	invNumber := strings.TrimSpace(req.InvoiceNumber)
	if invNumber == "" {
		invNumber = fmt.Sprintf("ECC-INV-%d-%05d",
			time.Now().Year(), time.Now().UnixNano()%100000)
	}

	totalDue := req.TotalFee + req.GovernmentFee + req.SchoolPartnerFee
	balance := totalDue - req.AmountReceived

	newInv := Invoice{
		RecordScope:           newRecordScope(r),
		ID:                    newID("inv"),
		InvoiceNumber:         invNumber,
		ClientID:              clientID,
		ClientName:            clientName,
		CaseID:                caseID,
		CaseNumber:            caseNumber,
		TotalFee:              req.TotalFee,
		GovernmentFee:         req.GovernmentFee,
		SchoolPartnerFee:      req.SchoolPartnerFee,
		AmountReceived:        req.AmountReceived,
		Balance:               balance,
		PaymentMilestone:      strings.TrimSpace(req.PaymentMilestone),
		PaymentMethod:         strings.TrimSpace(req.PaymentMethod),
		OfficialReceiptNumber: strings.TrimSpace(req.OfficialReceiptNumber),
		RefundStatus:          strings.TrimSpace(req.RefundStatus),
		ReferralCommission:    req.ReferralCommission,
		PartnerPayable:        req.PartnerPayable,
		PaymentApproval:       strings.TrimSpace(req.PaymentApproval),
		Notes:                 req.Notes,
		CreatedBy:             createdBy,
		CreatedAt:             now,
	}

	state.invoices = append(state.invoices, newInv)
	writeJSON(w, http.StatusCreated, map[string]any{"invoice": newInv})
}

func createInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createInvoiceHandler(w, r)
}

func (state *runtimeStore) updateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req invoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, inv := range state.invoices {
		if inv.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	if fields := state.validateInvoice(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	clientID := strings.TrimSpace(req.ClientID)
	clientName := ""
	if c, ok := state.findContactByID(clientID); ok {
		clientName = c.Name
	}

	caseID := strings.TrimSpace(req.CaseID)
	caseNumber := ""
	if c, ok := state.findCaseByID(caseID); ok {
		caseNumber = c.CaseNumber
	}

	totalDue := req.TotalFee + req.GovernmentFee + req.SchoolPartnerFee
	balance := totalDue - req.AmountReceived

	updated := state.invoices[idx]
	updated.InvoiceNumber = strings.TrimSpace(req.InvoiceNumber)
	updated.ClientID = clientID
	updated.ClientName = clientName
	updated.CaseID = caseID
	updated.CaseNumber = caseNumber
	updated.TotalFee = req.TotalFee
	updated.GovernmentFee = req.GovernmentFee
	updated.SchoolPartnerFee = req.SchoolPartnerFee
	// Receipt amounts are controlled by the ledger.
	updated.AmountReceived = state.invoices[idx].AmountReceived
	updated.Balance = balance
	updated.PaymentMilestone = strings.TrimSpace(req.PaymentMilestone)
	updated.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	updated.OfficialReceiptNumber = strings.TrimSpace(req.OfficialReceiptNumber)
	// Refund state is managed by the approval and processing workflow.
	updated.RefundStatus = state.invoices[idx].RefundStatus
	updated.ReferralCommission = req.ReferralCommission
	updated.PartnerPayable = req.PartnerPayable
	updated.PaymentApproval = strings.TrimSpace(req.PaymentApproval)
	updated.Notes = req.Notes

	state.invoices[idx] = updated
	state.recomputeInvoiceLocked(idx)
	updated = state.invoices[idx]
	writeJSON(w, http.StatusOK, map[string]any{"invoice": updated})
}

func updateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateInvoiceHandler(w, r)
}

func (state *runtimeStore) requestRefundHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req refundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		writeFieldErrors(w, map[string]string{"reason": "Reason is required"})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, inv := range state.invoices {
		if inv.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	amount := req.Amount
	if amount == 0 {
		amount = state.invoices[idx].AmountReceived
	}
	if amount <= 0 || amount > state.invoices[idx].AmountReceived {
		writeError(w, 400, "Refund cannot exceed the amount received")
		return
	}
	if _, err := entry(r, "refund", id, map[string]any{"amount": money(amount), "currency": currencyOf(state.invoices[idx].Currency), "reason": reason, "status": "requested", "requestedBy": currentUser(r).ID}, false); err != nil {
		writeError(w, 500, "Could not save refund request")
		return
	}
	state.invoices[idx].RefundStatus = "Requested"
	state.invoices[idx].PaymentMilestone = "Refund Review"
	state.invoices[idx].Notes = state.invoices[idx].Notes + "\n[Refund requested] " + reason

	writeJSON(w, http.StatusOK, map[string]any{"invoice": state.invoices[idx]})
}

func requestRefundHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.requestRefundHandler(w, r)
}

func (state *runtimeStore) deleteInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, inv := range state.invoices {
		if inv.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	for _, p := range state.payments {
		if p.InvoiceID == id {
			writeError(w, 409, "Invoices with payments must be retained for the ledger")
			return
		}
	}
	state.invoices = append(state.invoices[:idx], state.invoices[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteInvoiceHandler(w, r)
}

// ================= PAYMENTS =================

func (state *runtimeStore) listPaymentsHandler(w http.ResponseWriter, r *http.Request) {
	invoiceID := strings.TrimSpace(r.URL.Query().Get("invoiceId"))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listPayments()
	filtered := make([]Payment, 0, len(all))
	for _, p := range all {
		if invoiceID != "" && p.InvoiceID != invoiceID {
			continue
		}
		filtered = append(filtered, p)
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"payments": slicePage(filtered, offset, limit),
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func listPaymentsHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listPaymentsHandler(w, r)
}

func (state *runtimeStore) createPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var req paymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email, _ := r.Context().Value(ctxUserEmail).(string)
	createdBy := "Unknown"
	if email != "" {
		if u, ok := state.findUserByEmail(email); ok {
			createdBy = u.Name
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	invoiceID := strings.TrimSpace(req.InvoiceID)
	if invoiceID == "" {
		writeFieldErrors(w, map[string]string{"invoiceId": "Invoice is required"})
		return
	}

	invIdx := -1
	for i, inv := range state.invoices {
		if inv.ID == invoiceID {
			invIdx = i
			break
		}
	}
	if invIdx == -1 {
		writeFieldErrors(w, map[string]string{"invoiceId": "Invoice not found"})
		return
	}

	if money(req.Amount) <= 0 {
		writeFieldErrors(w, map[string]string{"amount": "Amount must be positive"})
		return
	}
	ledger := append(append([]Payment{}, state.payments...), state.hiddenLedgerPayments...)
	exact, possible := receiptRisk(req, ledger)
	if exact {
		writeFieldErrors(w, map[string]string{"form": "This invoice already has a receipt with this reference. Check payment history before recording another receipt."})
		return
	}
	if possible && !req.ConfirmDuplicate {
		writeFieldErrors(w, map[string]string{"form": "A receipt with the same amount, date and method already exists. Check payment history and confirm only if this is a separate receipt.", "duplicate": "Confirm this is a separate receipt"})
		return
	}
	if possible && req.ConfirmDuplicate && strings.TrimSpace(req.Reference) == "" {
		writeFieldErrors(w, map[string]string{"form": "Enter a distinct receipt reference when confirming a separate receipt."})
		return
	}

	now := time.Now().Format("2006-01-02")
	newPayment := Payment{
		RecordScope: newRecordScope(r),
		ID:          newID("pay"),
		InvoiceID:   invoiceID,
		Amount:      money(req.Amount),
		PaidOn:      normalizeDate(req.PaidOn),
		Method:      strings.TrimSpace(req.Method),
		Reference:   strings.TrimSpace(req.Reference),
		Notes:       req.Notes,
		CreatedBy:   createdBy,
		CreatedAt:   now,
	}

	state.recomputeInvoiceLocked(invIdx)
	newPayment.RecordScope = state.invoices[invIdx].RecordScope
	newPayment.Currency = currencyOf(state.invoices[invIdx].Currency)
	newPayment.Status = "posted"
	if req.RequireApproval {
		newPayment.Status = "pending_approval"
	}
	if possible && req.ConfirmDuplicate {
		newPayment.Notes += "\nConfirmed as a separate receipt after duplicate warning."
	}
	state.payments = append(state.payments, newPayment)
	if req.RequireApproval {
		for _, u := range state.users {
			if !u.Disabled && isManager(u) && canRead(newPayment.RecordScope, u) {
				if _, err := storeDB(r).Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), u.OrgID, u.ID, "receipt", "Receipt awaiting approval", state.invoices[invIdx].InvoiceNumber, "/invoices/"+invoiceID, "receipt:"+newPayment.ID, utcNow()); err != nil {
					writeError(w, 500, "Could not notify receipt reviewers")
					return
				}
			}
		}
	}

	state.recomputeInvoiceLocked(invIdx)
	inv := state.invoices[invIdx]

	writeJSON(w, http.StatusCreated, map[string]any{
		"payment": newPayment,
		"invoice": inv,
	})
}

func createPaymentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createPaymentHandler(w, r)
}

func (state *runtimeStore) deletePaymentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, p := range state.payments {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Payment not found")
		return
	}
	if pendingReceipt(state.payments[idx]) || state.payments[idx].Status == "void" {
		writeError(w, 409, "Use manager review for pending receipts; reviewed receipts must remain in payment history")
		return
	}
	if state.payments[idx].ReviewedAt != "" {
		writeError(w, 409, "Use the refund workflow for approved receipts; approval history must be retained")
		return
	}

	if state.payments[idx].Status == "refund" || state.payments[idx].Status == "refund_reversal" || strings.HasPrefix(state.payments[idx].Method, "Stripe") {
		writeError(w, 409, "Use the refund workflow to reverse this payment")
		return
	}
	invoiceID := state.payments[idx].InvoiceID
	for i := range state.invoices {
		if state.invoices[i].ID == invoiceID {
			state.recomputeInvoiceLocked(i)
		}
	}
	state.payments = append(state.payments[:idx], state.payments[idx+1:]...)
	for i := range state.invoices {
		if state.invoices[i].ID == invoiceID {
			state.recomputeInvoiceLocked(i)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deletePaymentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deletePaymentHandler(w, r)
}
