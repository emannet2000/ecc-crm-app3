package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type studentRequest struct {
	Name             string `json:"name"`
	StudentCode      string `json:"studentCode"`
	CountryCode      string `json:"countryCode"`
	SchoolID         string `json:"schoolId"`
	AgentID          string `json:"agentId"`
	Program          string `json:"program"`
	AcceptanceStatus string `json:"acceptanceStatus"`
	VisaStatus       string `json:"visaStatus"`
	InvoiceStatus    string `json:"invoiceStatus"`
	Notes            string `json:"notes"`
}

var validAcceptanceStatuses = []string{"Pending", "Accepted", "Rejected", "Waitlisted"}
var validVisaStatuses = []string{"Not Started", "Pending", "Approved", "Denied"}
var validInvoiceStatuses = []string{"Not Issued", "Issued", "Paid", "Overdue"}

func (state *runtimeStore) linkedStudentCounts() (map[string]int, map[string]int) {
	agents, schools := map[string]int{}, map[string]int{}
	for _, student := range state.listStudents() {
		if student.AgentID != "" {
			agents[student.AgentID]++
		}
		if student.SchoolID != "" {
			schools[student.SchoolID]++
		}
	}
	return agents, schools
}

func linkedStudentCounts() (map[string]int, map[string]int) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.linkedStudentCounts()
}

func isValidAcceptanceStatus(s string) bool {
	for _, v := range validAcceptanceStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func isValidVisaStatus(s string) bool {
	for _, v := range validVisaStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func isValidInvoiceStatus(s string) bool {
	for _, v := range validInvoiceStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func (state *runtimeStore) validateStudent(req studentRequest) map[string]string {
	fields := map[string]string{}

	name := strings.TrimSpace(req.Name)
	acceptance := strings.TrimSpace(req.AcceptanceStatus)
	visa := strings.TrimSpace(req.VisaStatus)
	invoice := strings.TrimSpace(req.InvoiceStatus)
	schoolID := strings.TrimSpace(req.SchoolID)
	agentID := strings.TrimSpace(req.AgentID)

	if name == "" {
		fields["name"] = "Student name is required"
	} else if len(name) > 200 {
		fields["name"] = "Name must be 200 characters or fewer"
	}

	if acceptance == "" {
		fields["acceptanceStatus"] = "Acceptance status is required"
	} else if !isValidAcceptanceStatus(acceptance) {
		fields["acceptanceStatus"] = "Invalid acceptance status"
	}

	if visa == "" {
		fields["visaStatus"] = "Visa status is required"
	} else if !isValidVisaStatus(visa) {
		fields["visaStatus"] = "Invalid visa status"
	}

	if invoice == "" {
		fields["invoiceStatus"] = "Invoice status is required"
	} else if !isValidInvoiceStatus(invoice) {
		fields["invoiceStatus"] = "Invalid invoice status"
	}

	if schoolID != "" {
		if _, ok := state.getSchoolByID(schoolID); !ok {
			fields["schoolId"] = "School not found"
		}
	}

	if agentID != "" {
		if _, ok := state.getAgentByID(agentID); !ok {
			fields["agentId"] = "Agent not found"
		}
	}

	return fields
}

func validateStudent(req studentRequest) map[string]string {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.validateStudent(req)
}

func (state *runtimeStore) listStudentsHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	agentID := strings.TrimSpace(r.URL.Query().Get("agentId"))
	schoolID := strings.TrimSpace(r.URL.Query().Get("schoolId"))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listStudents()
	filtered := make([]Student, 0, len(all))
	for _, s := range all {
		if (agentID != "" && s.AgentID != agentID) || (schoolID != "" && s.SchoolID != schoolID) {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.StudentCode), q) ||
			strings.Contains(strings.ToLower(s.SchoolName), q) ||
			strings.Contains(strings.ToLower(s.AgentName), q) ||
			strings.Contains(strings.ToLower(s.Program), q) {
			filtered = append(filtered, s)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"students": slicePage(filtered, offset, limit),
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func listStudentsHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listStudentsHandler(w, r)
}

func (state *runtimeStore) getStudentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, ok := state.getStudentByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Student not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"student": s})
}

func getStudentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getStudentHandler(w, r)
}

func (state *runtimeStore) createStudentHandler(w http.ResponseWriter, r *http.Request) {
	var req studentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := state.validateStudent(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	email, _ := r.Context().Value(ctxUserEmail).(string)
	createdBy := "Unknown"
	if email != "" {
		if u, ok := state.findUserByEmail(email); ok {
			createdBy = u.Name
		}
	}

	schoolID := strings.TrimSpace(req.SchoolID)
	schoolName := ""
	if schoolID != "" {
		if sch, ok := state.getSchoolByID(schoolID); ok {
			schoolName = sch.Name
		}
	}

	agentID := strings.TrimSpace(req.AgentID)
	agentName := ""
	if agentID != "" {
		if a, ok := state.getAgentByID(agentID); ok {
			agentName = a.Name
		}
	}

	now := time.Now().Format("2006-01-02")
	newStudent := Student{
		RecordScope:      newRecordScope(r),
		ID:               newID("st"),
		Name:             strings.TrimSpace(req.Name),
		StudentCode:      strings.TrimSpace(req.StudentCode),
		CountryCode:      strings.TrimSpace(strings.ToUpper(req.CountryCode)),
		SchoolID:         schoolID,
		SchoolName:       schoolName,
		AgentID:          agentID,
		AgentName:        agentName,
		Program:          strings.TrimSpace(req.Program),
		AcceptanceStatus: strings.TrimSpace(req.AcceptanceStatus),
		VisaStatus:       strings.TrimSpace(req.VisaStatus),
		InvoiceStatus:    strings.TrimSpace(req.InvoiceStatus),
		Notes:            req.Notes,
		CreatedBy:        createdBy,
		CreatedAt:        now,
	}

	state.mu.Lock()
	state.students = append(state.students, newStudent)
	state.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{"student": newStudent})
}

func createStudentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createStudentHandler(w, r)
}

func (state *runtimeStore) updateStudentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req studentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := state.validateStudent(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	schoolID := strings.TrimSpace(req.SchoolID)
	schoolName := ""
	if schoolID != "" {
		if sch, ok := state.getSchoolByID(schoolID); ok {
			schoolName = sch.Name
		}
	}

	agentID := strings.TrimSpace(req.AgentID)
	agentName := ""
	if agentID != "" {
		if a, ok := state.getAgentByID(agentID); ok {
			agentName = a.Name
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, s := range state.students {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Student not found")
		return
	}

	updated := state.students[idx]
	updated.Name = strings.TrimSpace(req.Name)
	updated.StudentCode = strings.TrimSpace(req.StudentCode)
	updated.CountryCode = strings.TrimSpace(strings.ToUpper(req.CountryCode))
	updated.SchoolID = schoolID
	updated.SchoolName = schoolName
	updated.AgentID = agentID
	updated.AgentName = agentName
	updated.Program = strings.TrimSpace(req.Program)
	updated.AcceptanceStatus = strings.TrimSpace(req.AcceptanceStatus)
	updated.VisaStatus = strings.TrimSpace(req.VisaStatus)
	updated.InvoiceStatus = strings.TrimSpace(req.InvoiceStatus)
	updated.Notes = req.Notes

	state.students[idx] = updated
	writeJSON(w, http.StatusOK, map[string]any{"student": updated})
}

func updateStudentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateStudentHandler(w, r)
}

func (state *runtimeStore) deleteStudentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, s := range state.students {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Student not found")
		return
	}

	state.students = append(state.students[:idx], state.students[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteStudentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteStudentHandler(w, r)
}
