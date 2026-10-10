package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type documentRequest struct {
	CaseID               string `json:"caseId"`
	DocName              string `json:"docName"`
	Required             bool   `json:"required"`
	DateRequested        string `json:"dateRequested"`
	DateReceived         string `json:"dateReceived"`
	ExpiryDate           string `json:"expiryDate"`
	VerifiedBy           string `json:"verifiedBy"`
	VerificationDate     string `json:"verificationDate"`
	Status               string `json:"status"`
	RejectionReason      string `json:"rejectionReason"`
	TranslationRequired  bool   `json:"translationRequired"`
	LegalizationRequired bool   `json:"legalizationRequired"`
	FilePath             string `json:"filePath"`
	Notes                string `json:"notes"`
}

type documentStatusRequest struct {
	Status          string `json:"status"`
	RejectionReason string `json:"rejectionReason"`
}

var validDocumentStatuses = []string{
	"Not Requested", "Requested", "Received",
	"Under Review", "Correction Required", "Verified", "Expired",
}

func isValidDocumentStatus(s string) bool {
	for _, v := range validDocumentStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// validateDocument must be called with mu held.
func (state *runtimeStore) validateDocument(req documentRequest) map[string]string {
	fields := map[string]string{}

	caseID := strings.TrimSpace(req.CaseID)
	docName := strings.TrimSpace(req.DocName)
	status := strings.TrimSpace(req.Status)

	if caseID == "" {
		fields["caseId"] = "Case is required"
	} else if _, ok := state.findCaseByID(caseID); !ok {
		fields["caseId"] = "Case not found"
	}

	if docName == "" {
		fields["docName"] = "Document name is required"
	}

	if status == "" {
		fields["status"] = "Status is required"
	} else if !isValidDocumentStatus(status) {
		fields["status"] = "Invalid status"
	}

	return fields
}

func validateDocument(req documentRequest) map[string]string {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.validateDocument(req)
}

func (state *runtimeStore) listDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	caseID := strings.TrimSpace(r.URL.Query().Get("caseId"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listDocuments()
	filtered := make([]Document, 0, len(all))
	for _, d := range all {
		if caseID != "" && d.CaseID != caseID {
			continue
		}
		if status != "" && d.Status != status {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(d.DocName), q) ||
			strings.Contains(strings.ToLower(d.CaseNumber), q) {
			filtered = append(filtered, d)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"documents": slicePage(filtered, offset, limit),
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

func listDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listDocumentsHandler(w, r)
}

func (state *runtimeStore) getDocumentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, d := range state.documents {
		if d.ID == id {
			writeJSON(w, http.StatusOK, map[string]any{"document": d})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Document not found")
}

func getDocumentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getDocumentHandler(w, r)
}

func (state *runtimeStore) createDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var req documentRequest
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

	if req.Status == "Verified" && r.PathValue("id") == "" {
		writeError(w, 400, "Upload a file version before verification")
		return
	}
	if fields := state.validateDocument(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	caseID := strings.TrimSpace(req.CaseID)
	caseNumber := ""
	if c, ok := state.findCaseByID(caseID); ok {
		caseNumber = c.CaseNumber
	}

	now := time.Now().Format("2006-01-02")
	newDoc := Document{
		RecordScope:          newRecordScope(r),
		ID:                   newID("doc"),
		CaseID:               caseID,
		CaseNumber:           caseNumber,
		DocName:              strings.TrimSpace(req.DocName),
		Required:             req.Required,
		DateRequested:        normalizeDate(req.DateRequested),
		DateReceived:         strings.TrimSpace(req.DateReceived),
		ExpiryDate:           strings.TrimSpace(req.ExpiryDate),
		VerifiedBy:           strings.TrimSpace(req.VerifiedBy),
		VerificationDate:     strings.TrimSpace(req.VerificationDate),
		Status:               strings.TrimSpace(req.Status),
		RejectionReason:      strings.TrimSpace(req.RejectionReason),
		LatestVersion:        1,
		TranslationRequired:  req.TranslationRequired,
		LegalizationRequired: req.LegalizationRequired,
		FilePath:             "",
		Notes:                req.Notes,
		CreatedBy:            createdBy,
		CreatedAt:            now,
	}

	state.documents = append(state.documents, newDoc)
	writeJSON(w, http.StatusCreated, map[string]any{"document": newDoc})
}

func createDocumentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createDocumentHandler(w, r)
}

func (state *runtimeStore) updateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req documentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.documents {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Document not found")
		return
	}

	if fields := state.validateDocument(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	caseID := strings.TrimSpace(req.CaseID)
	caseNumber := ""
	if c, ok := state.findCaseByID(caseID); ok {
		caseNumber = c.CaseNumber
	}

	updated := state.documents[idx]
	updated.CaseID = caseID
	updated.CaseNumber = caseNumber
	updated.DocName = strings.TrimSpace(req.DocName)
	updated.Required = req.Required
	updated.DateRequested = strings.TrimSpace(req.DateRequested)
	updated.DateReceived = strings.TrimSpace(req.DateReceived)
	updated.ExpiryDate = strings.TrimSpace(req.ExpiryDate)
	updated.VerifiedBy = strings.TrimSpace(req.VerifiedBy)
	updated.VerificationDate = strings.TrimSpace(req.VerificationDate)
	if !documentTransition(w, r, updated, strings.TrimSpace(req.Status)) {
		return
	}
	updated.Status = strings.TrimSpace(req.Status)
	updated.RejectionReason = strings.TrimSpace(req.RejectionReason)
	updated.TranslationRequired = req.TranslationRequired
	updated.LegalizationRequired = req.LegalizationRequired
	// Protected file paths are assigned only by the upload endpoint.
	updated.Notes = req.Notes

	state.documents[idx] = updated
	writeJSON(w, http.StatusOK, map[string]any{"document": updated})
}

func updateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateDocumentHandler(w, r)
}

func (state *runtimeStore) updateDocumentStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req documentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.documents {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Document not found")
		return
	}

	status := strings.TrimSpace(req.Status)
	if !isValidDocumentStatus(status) {
		writeFieldErrors(w, map[string]string{"status": "Invalid status"})
		return
	}

	if !documentTransition(w, r, state.documents[idx], status) {
		return
	}
	state.documents[idx].Status = status
	if status == "Correction Required" {
		state.documents[idx].RejectionReason = strings.TrimSpace(req.RejectionReason)
	}
	if status == "Verified" {
		state.documents[idx].VerificationDate = organizationToday(r)
		state.documents[idx].VerifiedBy = currentUser(r).Name
	}

	writeJSON(w, http.StatusOK, map[string]any{"document": state.documents[idx]})
}

func updateDocumentStatusHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateDocumentStatusHandler(w, r)
}

func (state *runtimeStore) deleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.documents {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Document not found")
		return
	}

	state.documents = append(state.documents[:idx], state.documents[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteDocumentHandler(w, r)
}

func documentTransition(w http.ResponseWriter, r *http.Request, doc Document, status string) bool {
	if status == "Verified" {
		var n int
		if storeDB(r).QueryRow("SELECT count(*) FROM file_versions WHERE document_id=? AND org_id=?", doc.ID, currentUser(r).OrgID).Scan(&n) != nil || n == 0 {
			writeError(w, 409, "Upload a file version before verifying this document")
			return false
		}
		if doc.ExpiryDate != "" && doc.ExpiryDate < organizationToday(r) {
			writeError(w, 409, "An expired document cannot be verified")
			return false
		}
	}
	return true
}
