package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type schoolRequest struct {
	Name             string `json:"name"`
	CountryCode      string `json:"countryCode"`
	CommissionRate   string `json:"commissionRate"`
	ContractStatus   string `json:"contractStatus"`
	StudentsEnrolled int    `json:"studentsEnrolled"`
	ContactPerson    string `json:"contactPerson"`
	Website          string `json:"website"`
	Notes            string `json:"notes"`
}

var validContractStatuses = []string{"Signed", "Pending", "Follow-up Required"}

func isValidContractStatus(s string) bool {
	for _, v := range validContractStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func validateSchool(req schoolRequest) map[string]string {
	fields := map[string]string{}

	name := strings.TrimSpace(req.Name)
	status := strings.TrimSpace(req.ContractStatus)

	if name == "" {
		fields["name"] = "School name is required"
	} else if len(name) > 200 {
		fields["name"] = "School name must be 200 characters or fewer"
	}

	if status == "" {
		fields["contractStatus"] = "Contract status is required"
	} else if !isValidContractStatus(status) {
		fields["contractStatus"] = "Invalid contract status"
	}

	if req.StudentsEnrolled < 0 {
		fields["studentsEnrolled"] = "Student count can't be negative"
	}

	return fields
}

func (state *runtimeStore) listSchoolsHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listSchools()
	_, schoolCounts := state.linkedStudentCounts()
	filtered := make([]School, 0, len(all))
	for _, s := range all {
		s.StudentsEnrolled = schoolCounts[s.ID]
		if q == "" ||
			strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.CountryCode), q) ||
			strings.Contains(strings.ToLower(s.ContactPerson), q) {
			filtered = append(filtered, s)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"schools": slicePage(filtered, offset, limit),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func listSchoolsHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listSchoolsHandler(w, r)
}

func (state *runtimeStore) getSchoolHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, ok := state.getSchoolByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "School not found")
		return
	}
	_, schoolCounts := state.linkedStudentCounts()
	s.StudentsEnrolled = schoolCounts[s.ID]
	writeJSON(w, http.StatusOK, map[string]any{"school": s})
}

func getSchoolHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getSchoolHandler(w, r)
}

func (state *runtimeStore) createSchoolHandler(w http.ResponseWriter, r *http.Request) {
	var req schoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := validateSchool(req); len(fields) > 0 {
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

	now := time.Now().Format("2006-01-02")
	newSchool := School{
		RecordScope:      newRecordScope(r),
		ID:               newID("s"),
		Name:             strings.TrimSpace(req.Name),
		CountryCode:      strings.TrimSpace(strings.ToUpper(req.CountryCode)),
		CommissionRate:   strings.TrimSpace(req.CommissionRate),
		ContractStatus:   strings.TrimSpace(req.ContractStatus),
		StudentsEnrolled: req.StudentsEnrolled,
		ContactPerson:    strings.TrimSpace(req.ContactPerson),
		Website:          strings.TrimSpace(req.Website),
		Notes:            req.Notes,
		CreatedBy:        createdBy,
		CreatedAt:        now,
	}

	state.mu.Lock()
	state.schools = append(state.schools, newSchool)
	state.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{"school": newSchool})
}

func createSchoolHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createSchoolHandler(w, r)
}

func (state *runtimeStore) updateSchoolHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req schoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := validateSchool(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, s := range state.schools {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "School not found")
		return
	}

	updated := state.schools[idx]
	updated.Name = strings.TrimSpace(req.Name)
	updated.CountryCode = strings.TrimSpace(strings.ToUpper(req.CountryCode))
	updated.CommissionRate = strings.TrimSpace(req.CommissionRate)
	updated.ContractStatus = strings.TrimSpace(req.ContractStatus)
	updated.StudentsEnrolled = req.StudentsEnrolled
	updated.ContactPerson = strings.TrimSpace(req.ContactPerson)
	updated.Website = strings.TrimSpace(req.Website)
	updated.Notes = req.Notes

	state.schools[idx] = updated
	writeJSON(w, http.StatusOK, map[string]any{"school": updated})
}

func updateSchoolHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateSchoolHandler(w, r)
}

func (state *runtimeStore) deleteSchoolHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, s := range state.schools {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "School not found")
		return
	}

	state.schools = append(state.schools[:idx], state.schools[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteSchoolHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteSchoolHandler(w, r)
}
