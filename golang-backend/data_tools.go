package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/go-pdf/fpdf"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
)

//go:embed assets/DejaVuSans.ttf
var pdfFont []byte

func newPDF(title string) *fpdf.Fpdf {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddUTF8FontFromBytes("CRM", "", pdfFont)
	p.SetFont("CRM", "", 11)
	p.SetTitle(title, true)
	p.AddPage()
	p.SetTextColor(25, 41, 63)
	p.SetFontSize(20)
	p.MultiCell(190, 10, title, "", "L", false)
	p.Ln(5)
	p.SetFontSize(10)
	return p
}
func invoicePDF(inv Invoice) ([]byte, error) {
	p := newPDF("Invoice " + inv.InvoiceNumber)
	p.MultiCell(190, 7, "Bill to: "+inv.ClientName+"\nCase: "+inv.CaseNumber+"\nIssued: "+inv.CreatedAt+"\nDue: "+inv.DueDate, "", "L", false)
	p.Ln(8)
	currency := currencyOf(inv.Currency)
	for _, line := range []struct {
		label  string
		amount float64
	}{{"Service fee", inv.TotalFee}, {"Government fee", inv.GovernmentFee}, {"School / partner fee", inv.SchoolPartnerFee}, {"Tax", inv.TaxAmount}, {"Discount", -inv.Discount}, {"Credit notes", -inv.CreditAmount}, {"Total", invoiceTotal(inv)}, {"Received", inv.AmountReceived}, {"Balance due", inv.Balance}} {
		p.CellFormat(120, 8, line.label, "B", 0, "L", false, 0, "")
		p.CellFormat(65, 8, fmt.Sprintf("%s %.2f", currency, line.amount), "B", 1, "R", false, 0, "")
	}
	p.Ln(10)
	p.MultiCell(190, 6, inv.Notes, "", "L", false)
	var b bytes.Buffer
	err := p.Output(&b)
	return b.Bytes(), err
}
func validCurrency(s string) bool {
	return strings.Contains("|USD|EUR|GBP|CAD|AUD|NZD|PHP|SGD|INR|JPY|CNY|AED|HKD|CHF|KRW|MYR|THB|IDR|", "|"+s+"|") && s != ""
}
func currencyOf(s string) string {
	if s == "" {
		return "USD"
	}
	return s
}
func money(n float64) float64 { return math.Round(n*100) / 100 }
func validateIPAllowlist(s, ip string) bool {
	if strings.TrimSpace(s) == "" {
		return true
	}
	client := net.ParseIP(ip)
	allowed := false
	for _, token := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' }) {
		if address := net.ParseIP(token); address != nil {
			allowed = allowed || address.Equal(client)
		} else if _, network, err := net.ParseCIDR(token); err == nil {
			allowed = allowed || network.Contains(client)
		} else {
			return false
		}
	}
	return allowed
}
func registerDataRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/imports/contacts", authMiddleware(handleContactImport))
	m.HandleFunc("GET /api/invoices/{id}/pdf", authMiddleware(handleInvoicePDF))
	m.HandleFunc("GET /api/duplicates", authMiddleware(handleDuplicates))
	m.HandleFunc("POST /api/contacts/merge", authMiddleware(handleMergeContacts))
	m.HandleFunc("POST /api/contacts/bulk", authMiddleware(handleBulkContacts))
}
func (state *runtimeStore) handleInvoicePDF(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	var inv Invoice
	ok := false
	for _, item := range state.invoices {
		if item.ID == r.PathValue("id") {
			inv = item
			ok = true
		}
	}
	state.mu.RUnlock()
	if !ok {
		writeError(w, 404, "Invoice not found")
		return
	}
	data, err := invoicePDF(inv)
	if err != nil {
		writeError(w, 500, "Could not create PDF")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="invoice.pdf"`)
	w.Write(data)
}

func handleInvoicePDF(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleInvoicePDF(w, r)
}
func (state *runtimeStore) handleContactImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, 400, "Choose a CSV under 5 MB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "Choose a CSV file")
		return
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		writeError(w, 400, "CSV header is missing")
		return
	}
	indexes := map[string]int{}
	for i, key := range header {
		indexes[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(key, "\ufeff")))] = i
	}
	if _, ok := indexes["name"]; !ok {
		writeError(w, 400, "CSV needs a name column; optional: email, phone, company, title, location, stage, tags, notes")
		return
	}
	get := func(row []string, key string) string {
		if i, ok := indexes[key]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	result := []Contact{}
	errors := []map[string]any{}
	seen := map[string]bool{}
	state.mu.RLock()
	for _, c := range state.contacts {
		if c.Email != "" {
			seen[strings.ToLower(c.Email)] = true
		}
	}
	state.mu.RUnlock()
	line := 1
	for {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		line++
		if line > 5001 {
			writeError(w, 400, "Import up to 5,000 contacts at a time")
			return
		}
		if e != nil {
			errors = append(errors, map[string]any{"row": line, "error": "Malformed CSV row"})
			continue
		}
		name, email, phone := get(row, "name"), strings.ToLower(get(row, "email")), get(row, "phone")
		problem := ""
		if name == "" {
			problem = "Name is required"
		} else if email != "" && !validEmail(email) {
			problem = "Email is invalid"
		} else if email != "" && seen[email] {
			problem = "Email duplicates an existing contact or earlier row"
		} else if !validPhone(phone) {
			problem = "Phone must contain 7–15 digits with an optional + prefix"
		}
		if problem != "" {
			errors = append(errors, map[string]any{"row": line, "error": problem})
			continue
		}
		if email != "" {
			seen[email] = true
		}
		stage := get(row, "stage")
		if stage == "" {
			stage = "Lead"
		}
		result = append(result, Contact{RecordScope: newRecordScope(r), ID: newID("c"), Name: name, Email: email, Phone: normalizePhone(phone), Company: get(row, "company"), Title: get(row, "title"), Location: get(row, "location"), Stage: stage, Tags: strings.FieldsFunc(get(row, "tags"), func(c rune) bool { return c == ';' }), Notes: get(row, "notes"), Owner: currentUser(r).Name, CreatedAt: organizationToday(r)})
	}
	dry := r.FormValue("preview") != "false"
	if !dry && len(errors) > 0 {
		writeJSON(w, 422, map[string]any{"errors": errors, "valid": len(result), "message": "Nothing imported. Fix the invalid rows first."})
		return
	}
	if !dry {
		state.mu.Lock()
		state.contacts = append(state.contacts, result...)
		state.mu.Unlock()
	}
	preview := result
	if len(preview) > 20 {
		preview = preview[:20]
	}
	writeJSON(w, 200, map[string]any{"preview": dry, "valid": len(result), "errors": errors, "contacts": preview})
}

func handleContactImport(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleContactImport(w, r)
}
func normalizePhone(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(s)
}
func validPhone(s string) bool {
	s = normalizePhone(s)
	if s == "" {
		return true
	}
	s = strings.TrimPrefix(s, "+")
	if len(s) < 7 || len(s) > 15 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func (state *runtimeStore) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	groups := map[string][]Contact{}
	state.mu.RLock()
	for _, c := range state.contacts {
		key := strings.ToLower(strings.TrimSpace(c.Email))
		if key == "" {
			key = "name:" + strings.ToLower(strings.TrimSpace(c.Name)) + ":" + normalizePhone(c.Phone)
		}
		groups[key] = append(groups[key], c)
	}
	state.mu.RUnlock()
	out := [][]Contact{}
	for _, g := range groups {
		if len(g) > 1 {
			out = append(out, g)
		}
	}
	writeJSON(w, 200, map[string]any{"groups": out})
}

func handleDuplicates(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleDuplicates(w, r)
}
func (state *runtimeStore) handleMergeContacts(w http.ResponseWriter, r *http.Request) {
	var req struct{ KeepID, RemoveID string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.KeepID == req.RemoveID {
		writeError(w, 400, "Choose two different contacts")
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	keep, ok := state.findContactByID(req.KeepID)
	remove, other := state.findContactByID(req.RemoveID)
	if !ok || !other {
		writeError(w, 404, "Contact not found")
		return
	}
	if !canWrite(keep.RecordScope, currentUser(r)) || !canWrite(remove.RecordScope, currentUser(r)) {
		writeError(w, 403, "You must own both contacts")
		return
	}
	if keep.Email == "" {
		keep.Email = remove.Email
	}
	if keep.Phone == "" {
		keep.Phone = remove.Phone
	}
	keep.Notes += fmt.Sprintf("\n[Merged from %s] %s · %s · %s · %s · %s · %s\n%s", remove.ID, remove.Name, remove.Email, remove.Phone, remove.Company, remove.Title, remove.Location, remove.Notes)
	if remove.LastContact > keep.LastContact {
		keep.LastContact = remove.LastContact
	}
	keep.Tags = append(keep.Tags, remove.Tags...)
	out := []Contact{}
	for _, c := range state.contacts {
		if c.ID == keep.ID {
			c = keep
		}
		if c.ID != remove.ID {
			out = append(out, c)
		}
	}
	state.contacts = out
	for i := range state.deals {
		if state.deals[i].ContactID == remove.ID {
			state.deals[i].ContactID = keep.ID
			state.deals[i].ContactName = keep.Name
		}
	}
	for i := range state.tasks {
		if state.tasks[i].ContactID == remove.ID {
			state.tasks[i].ContactID = keep.ID
			state.tasks[i].ContactName = keep.Name
		}
	}
	for i := range state.activities {
		if state.activities[i].ContactID == remove.ID {
			state.activities[i].ContactID = keep.ID
		}
	}
	for i := range state.cases {
		if state.cases[i].ClientID == remove.ID {
			state.cases[i].ClientID = keep.ID
			state.cases[i].ClientName = keep.Name
		}
	}
	for i := range state.invoices {
		if state.invoices[i].ClientID == remove.ID {
			state.invoices[i].ClientID = keep.ID
			state.invoices[i].ClientName = keep.Name
		}
	}
	writeJSON(w, 200, map[string]any{"contact": keep})
}

func handleMergeContacts(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleMergeContacts(w, r)
}
func (state *runtimeStore) handleBulkContacts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string
		Stage  string
		Delete bool
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > 500 {
		writeError(w, 400, "Select 1–500 contacts")
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	selected := map[string]bool{}
	for _, id := range req.IDs {
		c, ok := state.findContactByID(id)
		if !ok || !canWrite(c.RecordScope, currentUser(r)) {
			writeError(w, 403, "All selected contacts must be editable")
			return
		}
		selected[id] = true
	}
	if req.Delete {
		for _, d := range state.deals {
			if selected[d.ContactID] {
				writeError(w, 409, "Reassign related deals before deleting contacts")
				return
			}
		}
		for _, c := range state.cases {
			if selected[c.ClientID] {
				writeError(w, 409, "Reassign related cases before deleting contacts")
				return
			}
		}
	}
	out := []Contact{}
	for _, c := range state.contacts {
		if selected[c.ID] {
			if req.Delete {
				continue
			}
			if req.Stage != "" {
				c.Stage = req.Stage
			}
		}
		out = append(out, c)
	}
	state.contacts = out
	writeJSON(w, 200, map[string]any{"changed": len(req.IDs)})
}

func handleBulkContacts(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleBulkContacts(w, r)
}
func xmlText(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
func xlsxExport(header []string, records []map[string]any) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	files := map[string]string{"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, "_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`, "xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="CRM export" sheetId="1" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`}
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	row := func(values []string) {
		sheet.WriteString("<row>")
		for _, v := range values {
			sheet.WriteString(`<c t="inlineStr"><is><t xml:space="preserve">` + xmlText(v) + `</t></is></c>`)
		}
		sheet.WriteString("</row>")
	}
	row(header)
	for _, record := range records {
		values := []string{}
		for _, key := range header {
			v := record[key]
			if s, ok := v.(string); ok {
				values = append(values, s)
			} else {
				values = append(values, csvValue(v))
			}
		}
		row(values)
	}
	sheet.WriteString("</sheetData></worksheet>")
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	for name, content := range files {
		f, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	err := z.Close()
	return b.Bytes(), err
}
func pdfExport(title string, header []string, records []map[string]any) ([]byte, error) {
	p := newPDF(title)
	for i, record := range records {
		p.SetFontSize(12)
		p.MultiCell(190, 7, strconv.Itoa(i+1)+". "+recordLabel(mustJSON(record)), "", "L", false)
		p.SetFontSize(9)
		for _, key := range header {
			if record[key] != nil && csvValue(record[key]) != "" {
				p.MultiCell(190, 5, key+": "+csvValue(record[key]), "", "L", false)
			}
		}
		p.Ln(5)
	}
	var b bytes.Buffer
	err := p.Output(&b)
	return b.Bytes(), err
}
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
