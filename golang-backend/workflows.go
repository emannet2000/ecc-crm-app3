package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
}
type StageChange struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Actor string `json:"actor"`
	At    string `json:"at"`
}
type ChecklistItem struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
type Program struct {
	Name     string  `json:"name"`
	Fee      float64 `json:"fee"`
	Currency string  `json:"currency"`
	Duration string  `json:"duration"`
}

func registerWorkflowRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/workflows/{action}", authMiddleware(handleWorkflow))
	m.HandleFunc("GET /api/workflows", authMiddleware(handleWorkflowOverview))
	m.HandleFunc("POST /api/invoices/{id}/email", authMiddleware(handleInvoiceEmail))
	m.HandleFunc("POST /api/views", authMiddleware(handleSaveView))
	m.HandleFunc("DELETE /api/views/{view}", authMiddleware(handleDeleteView))
}
func entry(r *http.Request, category, record string, data any, private bool) (string, error) {
	id := newID("entry")
	user := ""
	if private {
		user = currentUser(r).ID
	}
	_, err := storeDB(r).Exec("INSERT INTO workspace_entries VALUES(?,?,?,?,?,?,?,?)", id, currentUser(r).OrgID, category, user, record, mustJSON(data), utcNow(), utcNow())
	return id, err
}
func invoiceTotal(i Invoice) float64 {
	return money(math.Max(0, i.TotalFee+i.GovernmentFee+i.SchoolPartnerFee+i.TaxAmount-i.Discount-i.CreditAmount))
}

// The opening amount preserves imported historical receipts. Every later receipt
// and refund is recomputed from the ledger, including deletion of a payment.
var hiddenLedgerPayments []Payment // protected by mu and the serialized request scope

func (state *runtimeStore) recomputeInvoiceLocked(index int) {
	i := state.invoices[index]
	sum := 0.0
	for _, p := range append(append([]Payment{}, state.payments...), state.hiddenLedgerPayments...) {
		if p.InvoiceID == i.ID && p.Status != "void" && !pendingReceipt(p) {
			sum += p.Amount
		}
	}
	if !i.LedgerInitialized {
		i.OpeningReceived = money(i.AmountReceived - sum)
		i.LedgerInitialized = true
	}
	i.AmountReceived = money(i.OpeningReceived + sum)
	i.Balance = money(invoiceTotal(i) - i.AmountReceived)
	if i.Balance <= 0 {
		i.PaymentMilestone = "Fully Paid"
	} else if i.AmountReceived > 0 {
		i.PaymentMilestone = "Instalment Due"
	}
	state.invoices[index] = i
}

func recomputeInvoiceLocked(index int) {
	state, release := runtimeForCall(nil)
	defer release()
	state.recomputeInvoiceLocked(index)
}
func (state *runtimeStore) handleWorkflowOverview(w http.ResponseWriter, r *http.Request) {
	s := state.snapshot()
	rates, err := queryObjects(r, "SELECT id,data FROM workspace_entries WHERE org_id=? AND category='exchange_rate'", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load workflows")
		return
	}
	templates, _ := queryObjects(r, "SELECT id,data FROM workspace_entries WHERE org_id=? AND category='case_template'", currentUser(r).OrgID)
	ledger, _ := queryObjects(r, "SELECT id,category,record_id AS recordId,data,created_at AS createdAt FROM workspace_entries WHERE org_id=? AND category IN ('refund','credit_note','commission','payout') ORDER BY created_at DESC", currentUser(r).OrgID)
	filtered := []map[string]any{}
	for _, e := range ledger {
		record, _ := e["recordId"].(string)
		if record == "" || visibleRecordID(s, record) {
			filtered = append(filtered, e)
		}
	}
	writeJSON(w, 200, map[string]any{"deals": s.Deals, "cases": s.Cases, "documents": s.Documents, "students": s.Students, "schools": s.Schools, "agents": s.Agents, "partners": s.Partners, "leads": s.Leads, "contacts": s.Contacts, "tasks": s.Tasks, "invoices": s.Invoices, "payments": s.Payments, "rates": rates, "templates": templates, "ledger": filtered})
}

func handleWorkflowOverview(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleWorkflowOverview(w, r)
}
func visibleRecordID(s diskStore, id string) bool {
	for _, items := range recordsOf(s) {
		if _, ok := items[id]; ok {
			return true
		}
	}
	return false
}
func (state *runtimeStore) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	var req map[string]json.RawMessage
	if !decodeRequest(w, r, &req) {
		return
	}
	text := func(k string) string { var v string; json.Unmarshal(req[k], &v); return strings.TrimSpace(v) }
	number := func(k string) float64 {
		var v float64
		if json.Unmarshal(req[k], &v) != nil {
			v, _ = strconv.ParseFloat(text(k), 64)
		}
		return v
	}
	id := text("id")
	action := r.PathValue("action")
	u := currentUser(r)
	if action == "case-template" || action == "exchange-rate" {
		if !isManager(u) {
			writeError(w, 403, "Manager access is required")
			return
		}
		if action == "exchange-rate" {
			if !validCurrency(text("from")) || !validCurrency(text("to")) || number("rate") <= 0 || text("asOf") == "" {
				writeError(w, 400, "Enter currencies, a positive rate, and its date")
				return
			}
		} else if text("name") == "" || text("documents") == "" {
			writeError(w, 400, "Enter a template name and one document per line")
			return
		}
		category := strings.ReplaceAll(action, "-", "_")
		saved, err := entry(r, category, "", req, false)
		if err != nil {
			writeError(w, 500, "Could not save configuration")
			return
		}
		writeJSON(w, 201, map[string]string{"id": saved})
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	fail := func(msg string) { writeError(w, 400, msg) }
	editable := func(scope RecordScope) bool {
		if !canWrite(scope, u) {
			writeError(w, 403, "You can only change records you own")
			return false
		}
		return true
	}
	switch action {
	case "deal":
		for i, d := range state.deals {
			if d.ID != id {
				continue
			}
			if !editable(d.RecordScope) {
				return
			}
			c := text("currency")
			if !validCurrency(c) {
				fail("Choose a supported currency")
				return
			}
			p := number("probability")
			if p < 0 || p > 100 {
				fail("Probability must be between 0 and 100")
				return
			}
			p /= 100
			d.Currency = c
			d.Probability = &p
			var lines []LineItem
			if len(req["lineItems"]) > 0 && json.Unmarshal(req["lineItems"], &lines) != nil {
				fail("Invalid line items")
				return
			}
			total := 0.0
			for _, line := range lines {
				if line.Description == "" || line.Quantity <= 0 || line.UnitPrice < 0 {
					fail("Each line needs a description, positive quantity and nonnegative price")
					return
				}
				total += line.Quantity * line.UnitPrice
			}
			if _, provided := req["lineItems"]; provided {
				d.LineItems = lines
			}
			if len(lines) > 0 {
				d.Value = money(total)
			} else if _, provided := req["value"]; provided {
				value := number("value")
				if value < 0 {
					fail("Deal amount cannot be negative")
					return
				}
				d.Value = money(value)
			}
			state.deals[i] = d
			writeJSON(w, 200, d)
			return
		}
	case "lead-convert":
		for i, l := range state.leads {
			if l.ID != id {
				continue
			}
			if !editable(l.RecordScope) {
				return
			}
			if l.ConvertedContactID != "" {
				writeJSON(w, 200, map[string]string{"contactId": l.ConvertedContactID})
				return
			}
			var contact Contact
			for _, c := range state.contacts {
				if l.Email != "" && strings.EqualFold(l.Email, c.Email) {
					if !editable(c.RecordScope) {
						return
					}
					contact = c
					break
				}
			}
			if contact.ID == "" {
				contact = Contact{RecordScope: l.RecordScope, ID: newID("c"), Name: l.Name, Email: l.Email, Phone: l.Phone, Location: l.CurrentCountry, Stage: "Lead", Owner: l.AssignedTo, Notes: l.Notes, Tags: []string{}, CreatedAt: organizationToday(r)}
				state.contacts = append(state.contacts, contact)
			}
			state.leads[i].ConvertedContactID = contact.ID
			state.leads[i].Status = "Converted"
			writeJSON(w, 200, map[string]string{"contactId": contact.ID})
			return
		}
	case "lead":
		for i, l := range state.leads {
			if l.ID == id {
				if !editable(l.RecordScope) {
					return
				}
				score := number("score")
				if score < 0 || score > 100 || number("sourceCost") < 0 {
					fail("Score must be 0–100 and source cost nonnegative")
					return
				}
				state.leads[i].Score = int(score)
				state.leads[i].SourceCost = number("sourceCost")
				if date := text("followUpDate"); date != "" {
					if _, err := time.Parse("2006-01-02", date); err != nil {
						fail("Enter a valid follow-up date")
						return
					}
					state.leads[i].FollowUpDate = date
					state.tasks = append(state.tasks, Task{RecordScope: l.RecordScope, ID: newID("t"), Title: "Follow up: " + l.Name, Description: text("message"), Status: "todo", DueDate: date, Owner: u.Name, CreatedAt: organizationToday(r)})
				}
				writeJSON(w, 200, state.leads[i])
				return
			}
		}
	case "task":
		for i, t := range state.tasks {
			if t.ID == id {
				if !editable(t.RecordScope) {
					return
				}
				rec := text("recurrence")
				if rec != "" && rec != "none" && rec != "daily" && rec != "weekly" && rec != "monthly" {
					fail("Choose daily, weekly, monthly, or none")
					return
				}
				date := text("reminderDate")
				if date != "" {
					if _, err := time.Parse("2006-01-02", date); err != nil {
						fail("Enter a valid reminder date")
						return
					}
				}
				state.tasks[i].Recurrence = rec
				state.tasks[i].ReminderDate = date
				writeJSON(w, 200, state.tasks[i])
				return
			}
		}
	case "student":
		for i, s := range state.students {
			if s.ID == id {
				if !editable(s.RecordScope) {
					return
				}
				kind := text("testType")
				score := number("testScore")
				if (kind == "IELTS" && (score < 0 || score > 9)) || (kind == "TOEFL" && (score < 0 || score > 120)) {
					fail("IELTS scores are 0–9; TOEFL scores are 0–120")
					return
				}
				s.TestType = kind
				s.TestScore = score
				s.Accommodation = text("accommodation")
				s.TravelDate = text("travelDate")
				stage := text("applicationStage")
				validStage := false
				for _, allowed := range []string{"Planning", "Applying", "Submitted", "Offer received", "Accepted", "Enrolled"} {
					validStage = validStage || stage == allowed
				}
				if !validStage {
					fail("Choose a valid application stage")
					return
				}
				s.ApplicationStage = stage
				s.Intake = text("intake")
				if len(s.Intake) > 100 {
					fail("Intake must be 100 characters or fewer")
					return
				}
				if kind != "" && kind != "IELTS" && kind != "TOEFL" {
					fail("Choose IELTS or TOEFL")
					return
				}
				if s.TravelDate != "" {
					if _, err := time.Parse("2006-01-02", s.TravelDate); err != nil {
						fail("Enter a valid travel date")
						return
					}
				}
				if json.Unmarshal(req["preDeparture"], &s.PreDeparture) != nil {
					fail("Choose valid checklist items")
					return
				}
				state.students[i] = s
				writeJSON(w, 200, s)
				return
			}
		}
	case "student-case":
		for _, s := range state.students {
			if s.ID == id {
				if !editable(s.RecordScope) {
					return
				}
				contact := Contact{}
				for _, c := range state.contacts {
					if strings.EqualFold(c.Name, s.Name) {
						contact = c
						break
					}
				}
				if contact.ID == "" {
					contact = Contact{RecordScope: s.RecordScope, ID: newID("c"), Name: s.Name, Stage: "Client", Owner: u.Name, Tags: []string{}, CreatedAt: organizationToday(r)}
					state.contacts = append(state.contacts, contact)
				}
				c := Case{RecordScope: s.RecordScope, ID: newID("case"), CaseNumber: fmt.Sprintf("ECC-%d", time.Now().UnixNano()), ClientID: contact.ID, ClientName: s.Name, ServiceCategory: "Study", DestinationCountry: s.CountryCode, VisaType: "Student", SchoolOrEmployer: s.SchoolName, AssignedOfficer: u.Name, DateOpened: organizationToday(r), CurrentStage: "Assessment", Priority: "Medium", CreatedBy: u.Name, CreatedAt: organizationToday(r)}
				c.StudentID = s.ID
				state.cases = append(state.cases, c)
				writeJSON(w, 201, map[string]any{"case": c})
				return
			}
		}
	case "case":
		for i, c := range state.cases {
			if c.ID != id {
				continue
			}
			if !editable(c.RecordScope) {
				return
			}
			parent := text("parentCaseId")
			if parent != "" {
				p, ok := state.findCaseByID(parent)
				if !ok || p.ID == c.ID {
					fail("Choose a different visible case")
					return
				}
				ancestor := p
				visited := map[string]bool{c.ID: true}
				for ancestor.ID != "" {
					if visited[ancestor.ID] {
						fail("Linked cases cannot form a cycle")
						return
					}
					visited[ancestor.ID] = true
					ancestor, _ = state.findCaseByID(ancestor.ParentCaseID)
				}
			}
			days := int(number("slaDays"))
			if days < 0 || days > 3650 {
				fail("SLA must be 0–3650 days")
				return
			}
			partner := text("partnerId")
			if partner != "" {
				found := false
				for _, p := range state.partners {
					found = found || p.ID == partner
				}
				if !found {
					fail("Partner not found")
					return
				}
				c.PartnerID = partner
			}
			c.ParentCaseID = parent
			c.SLADays = days
			if days > 0 {
				start, err := time.Parse("2006-01-02", c.DateOpened)
				if err != nil {
					start = time.Now()
				}
				c.NextDeadline = start.AddDate(0, 0, days).Format("2006-01-02")
			}
			var auto bool
			json.Unmarshal(req["autoAdvance"], &auto)
			c.AutoAdvance = auto
			if template := text("templateId"); template != "" && template != c.TemplateID {
				var payload string
				if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='case_template'", template, u.OrgID).Scan(&payload) != nil {
					fail("Template not found")
					return
				}
				var t map[string]string
				json.Unmarshal([]byte(payload), &t)
				for _, name := range strings.Split(t["documents"], "\n") {
					if strings.TrimSpace(name) != "" {
						state.documents = append(state.documents, Document{RecordScope: c.RecordScope, ID: newID("doc"), CaseID: c.ID, CaseNumber: c.CaseNumber, DocName: strings.TrimSpace(name), Required: true, Status: "Not Requested", CreatedBy: u.Name, CreatedAt: organizationToday(r)})
					}
				}
				c.TemplateID = template
			}
			state.cases[i] = c
			writeJSON(w, 200, c)
			return
		}
	case "school":
		for i, s := range state.schools {
			if s.ID == id {
				if !editable(s.RecordScope) {
					return
				}
				var programs []Program
				if json.Unmarshal(req["programs"], &programs) != nil {
					fail("Invalid programs")
					return
				}
				for _, p := range programs {
					if p.Name == "" || p.Fee < 0 || !validCurrency(p.Currency) {
						fail("Each program needs a name, nonnegative fee, and currency")
						return
					}
				}
				state.schools[i].Programs = programs
				writeJSON(w, 200, state.schools[i])
				return
			}
		}
	case "invoice":
		for i, inv := range state.invoices {
			if inv.ID != id {
				continue
			}
			if !editable(inv.RecordScope) {
				return
			}
			if !validCurrency(text("currency")) || number("taxAmount") < 0 || number("discount") < 0 {
				fail("Enter a valid currency and nonnegative tax and discount")
				return
			}
			for _, p := range state.payments {
				if p.InvoiceID == id && currencyOf(inv.Currency) != text("currency") {
					fail("Currency cannot change after payments are recorded")
					return
				}
			}
			inv.Currency = text("currency")
			inv.TaxAmount = money(number("taxAmount"))
			inv.Discount = money(number("discount"))
			inv.DueDate = text("dueDate")
			if inv.DueDate != "" {
				if _, err := time.Parse("2006-01-02", inv.DueDate); err != nil {
					fail("Enter a valid due date")
					return
				}
			}
			json.Unmarshal(req["dunning"], &inv.Dunning)
			state.invoices[i] = inv
			state.recomputeInvoiceLocked(i)
			writeJSON(w, 200, state.invoices[i])
			return
		}
	case "credit-note":
		if !isManager(u) {
			writeError(w, 403, "Manager approval is required")
			return
		}
		for i, inv := range state.invoices {
			if inv.ID == id {
				amount := money(number("amount"))
				if amount <= 0 || amount > invoiceTotal(inv) || text("reason") == "" {
					fail("Enter a reason and credit no larger than the invoice total")
					return
				}
				_, err := entry(r, "credit_note", id, map[string]any{"amount": amount, "currency": currencyOf(inv.Currency), "reason": text("reason"), "actor": u.ID}, false)
				if err != nil {
					writeError(w, 500, "Could not save credit note")
					return
				}
				state.invoices[i].CreditAmount += amount
				state.recomputeInvoiceLocked(i)
				writeJSON(w, 200, state.invoices[i])
				return
			}
		}
	case "refund-request":
		for i, inv := range state.invoices {
			if inv.ID == id {
				if !editable(inv.RecordScope) {
					return
				}
				amount := money(number("amount"))
				if amount <= 0 || amount > inv.AmountReceived || text("reason") == "" {
					fail("Enter a reason and amount no larger than the received balance")
					return
				}
				e, err := entry(r, "refund", id, map[string]any{"amount": amount, "currency": currencyOf(inv.Currency), "reason": text("reason"), "status": "requested", "requestedBy": u.ID}, false)
				if err != nil {
					writeError(w, 500, "Could not request refund")
					return
				}
				state.invoices[i].RefundStatus = "Requested"
				writeJSON(w, 201, map[string]string{"id": e})
				return
			}
		}
	case "refund-review", "refund-process", "refund-reverse":
		if !isManager(u) {
			writeError(w, 403, "Manager approval is required")
			return
		}
		var payload, invoiceID string
		if storeDB(r).QueryRow("SELECT data,record_id FROM workspace_entries WHERE id=? AND org_id=? AND category='refund'", id, u.OrgID).Scan(&payload, &invoiceID) != nil {
			fail("Refund not found")
			return
		}
		var refund map[string]any
		json.Unmarshal([]byte(payload), &refund)
		index := -1
		for i, inv := range state.invoices {
			if inv.ID == invoiceID {
				index = i
			}
		}
		if index < 0 {
			fail("Invoice not found")
			return
		}
		status, _ := refund["status"].(string)
		amount, _ := refund["amount"].(float64)
		if action == "refund-review" {
			if status != "requested" {
				fail("Only requested refunds can be reviewed")
				return
			}
			decision := text("decision")
			if decision != "approved" && decision != "rejected" {
				fail("Choose approved or rejected")
				return
			}
			refund["status"] = decision
			refund["reviewedBy"] = u.ID
		} else if action == "refund-process" {
			if status != "approved" || amount > state.invoices[index].AmountReceived || text("reference") == "" {
				fail("Refund must be approved, within available receipts, and have a payment reference")
				return
			}
			p := Payment{RecordScope: state.invoices[index].RecordScope, ID: newID("pay"), InvoiceID: invoiceID, Amount: -amount, Status: "refund", PaidOn: organizationToday(r), Method: "Refund", Reference: text("reference"), CreatedBy: u.Name, CreatedAt: organizationToday(r)}
			state.payments = append(state.payments, p)
			refund["paymentId"] = p.ID
			refund["status"] = "processed"
			refund["reference"] = text("reference")
			state.recomputeInvoiceLocked(index)
		} else {
			if status != "processed" || text("reference") == "" {
				fail("Only processed refunds can be reversed with a reference")
				return
			}
			state.payments = append(state.payments, Payment{RecordScope: state.invoices[index].RecordScope, ID: newID("pay"), InvoiceID: invoiceID, Amount: amount, Status: "refund_reversal", PaidOn: organizationToday(r), Method: "Refund reversal", Reference: text("reference"), CreatedBy: u.Name, CreatedAt: organizationToday(r)})
			refund["status"] = "reversed"
			state.recomputeInvoiceLocked(index)
		}
		state.invoices[index].RefundStatus = fmt.Sprint(refund["status"])
		_, err := storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=?", mustJSON(refund), utcNow(), id)
		if err != nil {
			writeError(w, 500, "Could not save refund")
			return
		}
		writeJSON(w, 200, refund)
		return
	case "commission", "payout":
		if !isManager(u) {
			writeError(w, 403, "Manager access is required")
			return
		}
		kind := text("entity")
		found := false
		for _, a := range state.agents {
			if kind == "agents" && a.ID == id {
				found = true
			}
		}
		for _, p := range state.partners {
			if kind == "partners" && p.ID == id {
				found = true
			}
		}
		if !found {
			fail("Choose an agent or partner")
			return
		}
		amount := number("amount")
		if action == "commission" {
			base, rate := number("base"), number("rate")
			if base < 0 || rate < 0 || rate > 100 {
				fail("Enter a nonnegative base and rate from 0–100")
				return
			}
			amount = money(base * rate / 100)
		}
		if amount <= 0 || !validCurrency(text("currency")) {
			fail("Enter a positive amount and currency")
			return
		}
		if action == "payout" && text("reference") == "" {
			fail("Record the bank or payment reference")
			return
		}
		saved, err := entry(r, action, id, map[string]any{"entity": kind, "amount": amount, "currency": text("currency"), "reference": text("reference"), "base": number("base"), "rate": number("rate"), "actor": u.ID}, false)
		if err != nil {
			writeError(w, 500, "Could not save ledger entry")
			return
		}
		writeJSON(w, 201, map[string]string{"id": saved})
		return
	}
	writeError(w, 404, "Record or workflow not found")
}

func handleWorkflow(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleWorkflow(w, r)
}
func (state *runtimeStore) handleInvoiceEmail(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	var inv Invoice
	var client Contact
	ok := false
	for _, i := range state.invoices {
		if i.ID == r.PathValue("id") {
			inv = i
			client, _ = state.findContactByID(i.ClientID)
			ok = true
		}
	}
	state.mu.RUnlock()
	if !ok {
		writeError(w, 404, "Invoice not found")
		return
	}
	if !validEmail(client.Email) {
		writeError(w, 400, "Add a valid client email first")
		return
	}
	if err := queueMail(r, currentUser(r), "invoice", client.Email, "Invoice "+inv.InvoiceNumber, fmt.Sprintf("Hello %s,\n\nYour invoice is attached. Balance due: %s %.2f.", client.Name, currencyOf(inv.Currency), inv.Balance), client.ID, inv.ID); err != nil {
		writeError(w, 500, "Could not queue invoice email")
		return
	}
	writeJSON(w, 202, map[string]string{"status": "queued"})
}

func handleInvoiceEmail(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleInvoiceEmail(w, r)
}
func handleSaveView(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name, Entity, Search, Status, Sort, Direction string
		Columns                                       []string
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	allowed := false
	for _, e := range entityTables {
		allowed = allowed || e == req.Entity
	}
	if !allowed || req.Name == "" {
		writeError(w, 400, "Enter a view name and module")
		return
	}
	id, err := entry(r, "saved_view", req.Entity, req, true)
	if err != nil {
		writeError(w, 500, "Could not save view")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}
func handleDeleteView(w http.ResponseWriter, r *http.Request) {
	res, err := storeDB(r).Exec("DELETE FROM workspace_entries WHERE id=? AND user_id=? AND category='saved_view'", r.PathValue("view"), currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "Could not delete view")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "View not found")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func (state *runtimeStore) applyComputedData() {
	state.mu.Lock()
	defer state.mu.Unlock()

	agentCount, schoolCount, partnerCount, partnerConverted := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, s := range state.students {
		agentCount[s.AgentID]++
		if s.AcceptanceStatus == "Enrolled" || s.ApplicationStage == "Enrolled" {
			schoolCount[s.SchoolID]++
		}
	}
	for _, c := range state.cases {
		partnerCount[c.PartnerID]++
		if c.CurrentStage == "Approved" || c.CurrentStage == "Closed" {
			partnerConverted[c.PartnerID]++
		}
	}
	for i := range state.agents {
		state.agents[i].StudentsReferred = agentCount[state.agents[i].ID]
	}
	for i := range state.schools {
		state.schools[i].StudentsEnrolled = schoolCount[state.schools[i].ID]
	}
	for i := range state.partners {
		state.partners[i].CasesReferred = partnerCount[state.partners[i].ID]
		state.partners[i].CasesConverted = partnerConverted[state.partners[i].ID]
	}
	currencies := map[string]string{}
	for _, inv := range state.invoices {
		currencies[inv.ID] = currencyOf(inv.Currency)
	}
	sums := map[string]float64{}
	for i := range state.payments {
		p := &state.payments[i]
		if currency, ok := currencies[p.InvoiceID]; ok {
			p.Currency = currency
		}
		if p.Status == "" {
			p.Status = "posted"
		}
		if p.Status != "void" && !pendingReceipt(*p) {
			sums[p.InvoiceID] += p.Amount
		}
	}
	for _, p := range state.hiddenLedgerPayments {
		if p.Status != "void" && !pendingReceipt(p) {
			sums[p.InvoiceID] += p.Amount
		}
	}
	for i := range state.invoices {
		inv := &state.invoices[i]
		sum := sums[inv.ID]
		if !inv.LedgerInitialized {
			inv.OpeningReceived = money(inv.AmountReceived - sum)
			inv.LedgerInitialized = true
		}
		inv.AmountReceived = money(inv.OpeningReceived + sum)
		inv.Balance = money(invoiceTotal(*inv) - inv.AmountReceived)
		if inv.Balance <= 0 {
			inv.PaymentMilestone = "Fully Paid"
		} else if inv.AmountReceived > 0 {
			inv.PaymentMilestone = "Instalment Due"
		}
	}
	required, verified := map[string]int{}, map[string]int{}
	for _, d := range state.documents {
		if d.Required {
			required[d.CaseID]++
			if d.Status == "Verified" {
				verified[d.CaseID]++
			}
		}
	}
	for i := range state.cases {
		c := &state.cases[i]
		if !c.AutoAdvance {
			continue
		}
		if required[c.ID] > verified[c.ID] && c.CurrentStage == "Ready for Submission" {
			c.CurrentStage = "Documents Under Review"
		}
		if required[c.ID] > 0 && required[c.ID] == verified[c.ID] && (c.CurrentStage == "Documents Pending" || c.CurrentStage == "Documents Under Review") {
			c.CurrentStage = "Ready for Submission"
		}
	}

}

func applyComputedData() {
	state, release := runtimeForCall(nil)
	defer release()
	state.applyComputedData()
}
func sortRecords[T any](items []T, r *http.Request) []T {
	field := r.URL.Query().Get("sort")
	if field == "" {
		return items
	}
	sort.SliceStable(items, func(i, j int) bool {
		var a, b map[string]any
		json.Unmarshal([]byte(mustJSON(items[i])), &a)
		json.Unmarshal([]byte(mustJSON(items[j])), &b)
		less := false
		if x, ok := a[field].(float64); ok {
			y, _ := b[field].(float64)
			less = x < y
		} else {
			less = strings.ToLower(fmt.Sprint(a[field])) < strings.ToLower(fmt.Sprint(b[field]))
		}
		if r.URL.Query().Get("direction") == "desc" {
			a, b = b, a
			if x, ok := a[field].(float64); ok {
				y, _ := b[field].(float64)
				return x < y
			}
			return strings.ToLower(fmt.Sprint(a[field])) < strings.ToLower(fmt.Sprint(b[field]))
		}
		return less
	})
	return items
}
