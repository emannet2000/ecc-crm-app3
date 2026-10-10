package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type dealRequest struct {
	Currency  string  `json:"currency"`
	Title     string  `json:"title"`
	ContactID string  `json:"contactId"`
	Value     float64 `json:"value"`
	Stage     string  `json:"stage"`
	CloseDate string  `json:"closeDate"`
	Owner     string  `json:"owner"`
	Notes     string  `json:"notes"`
}

type dealStageRequest struct {
	Stage string `json:"stage"`
}

var validDealStages = []string{"Lead", "Qualified", "Proposal", "Negotiation", "Won", "Lost"}

func isValidDealStage(stage string) bool {
	for _, s := range validDealStages {
		if s == stage {
			return true
		}
	}
	return false
}

// findContactByID must be called with mu held.
func (state *runtimeStore) findContactByID(id string) (Contact, bool) {
	for _, c := range state.contacts {
		if c.ID == id {
			return c, true
		}
	}
	return Contact{}, false
}

func findContactByID(id string) (Contact, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.findContactByID(id)
}

// validateDeal must be called with mu held.
func (state *runtimeStore) validateDeal(req dealRequest) map[string]string {
	fields := map[string]string{}

	title := strings.TrimSpace(req.Title)
	stage := strings.TrimSpace(req.Stage)
	contactID := strings.TrimSpace(req.ContactID)

	if title == "" {
		fields["title"] = "Title is required"
	} else if len(title) > 200 {
		fields["title"] = "Title must be 200 characters or fewer"
	}

	if req.Currency != "" && !validCurrency(req.Currency) {
		fields["currency"] = "Choose a supported currency"
	}
	if req.Value < 0 {
		fields["value"] = "Value can't be negative"
	}

	if stage != "" && !isValidDealStage(stage) {
		fields["stage"] = "Invalid stage"
	}

	if contactID != "" {
		if _, ok := state.findContactByID(contactID); !ok {
			fields["contactId"] = "Contact not found"
		}
	}

	return fields
}

func validateDeal(req dealRequest) map[string]string {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.validateDeal(req)
}

func (state *runtimeStore) listDeals(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit, offset := parseLimitOffset(r, 50, 0)

	state.mu.RLock()
	defer state.mu.RUnlock()

	filtered := make([]Deal, 0, len(state.deals))
	for _, d := range state.deals {
		if r.URL.Query().Get("mine") == "true" && d.OwnerID != currentUser(r).ID {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(d.Title), q) ||
			strings.Contains(strings.ToLower(d.ContactName), q) ||
			strings.Contains(strings.ToLower(d.Owner), q) {
			filtered = append(filtered, d)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"deals":  slicePage(filtered, offset, limit),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func listDeals(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listDeals(w, r)
}

func (state *runtimeStore) getDeal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.RLock()
	defer state.mu.RUnlock()

	for _, d := range state.deals {
		if d.ID == id {
			writeJSON(w, http.StatusOK, map[string]any{"deal": d})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Deal not found")
}

func getDeal(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getDeal(w, r)
}

func (state *runtimeStore) createDeal(w http.ResponseWriter, r *http.Request) {
	var req dealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if fields := state.validateDeal(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		stage = "Lead"
	}
	contactID := strings.TrimSpace(req.ContactID)
	contactName := ""
	if contactID != "" {
		if c, ok := state.findContactByID(contactID); ok {
			contactName = c.Name
		}
	}

	now := time.Now().Format("2006-01-02")
	newDeal := Deal{
		RecordScope: newRecordScope(r),
		Currency:    currencyOf(req.Currency),
		ID:          newID("d"),
		Title:       strings.TrimSpace(req.Title),
		ContactID:   contactID,
		ContactName: contactName,
		Value:       req.Value,
		Stage:       stage,
		CloseDate:   strings.TrimSpace(req.CloseDate),
		Owner:       strings.TrimSpace(req.Owner),
		Notes:       req.Notes,
		CreatedAt:   now,
	}

	state.deals = append(state.deals, newDeal)

	writeJSON(w, http.StatusCreated, map[string]any{"deal": newDeal})
}

func createDeal(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createDeal(w, r)
}

func (state *runtimeStore) updateDeal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.deals {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Deal not found")
		return
	}

	var req dealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if fields := state.validateDeal(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		stage = "Lead"
	}

	contactID := strings.TrimSpace(req.ContactID)
	contactName := ""
	if contactID != "" {
		if c, ok := state.findContactByID(contactID); ok {
			contactName = c.Name
		}
	}

	updated := state.deals[idx]
	updated.Title = strings.TrimSpace(req.Title)
	updated.ContactID = contactID
	updated.ContactName = contactName
	if len(updated.LineItems) > 0 {
		total := 0.0
		for _, line := range updated.LineItems {
			total += line.Quantity * line.UnitPrice
		}
		if money(req.Value) != money(total) {
			writeFieldErrors(w, map[string]string{"value": "Change products and prices in Workspace tools to update this amount"})
			return
		}
	}
	updated.Value = req.Value
	if req.Currency != "" {
		updated.Currency = req.Currency
	}
	if updated.Stage != stage {
		updated.StageHistory = append(updated.StageHistory, StageChange{updated.Stage, stage, currentUser(r).Name, utcNow()})
	}
	updated.Stage = stage
	updated.CloseDate = strings.TrimSpace(req.CloseDate)
	updated.Owner = strings.TrimSpace(req.Owner)
	updated.Notes = req.Notes

	state.deals[idx] = updated

	writeJSON(w, http.StatusOK, map[string]any{"deal": updated})
}

func updateDeal(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateDeal(w, r)
}

func (state *runtimeStore) updateDealStage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.deals {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Deal not found")
		return
	}

	var req dealStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	stage := strings.TrimSpace(req.Stage)
	if !isValidDealStage(stage) {
		writeFieldErrors(w, map[string]string{"stage": "Invalid stage"})
		return
	}

	if state.deals[idx].Stage != stage {
		state.deals[idx].StageHistory = append(state.deals[idx].StageHistory, StageChange{state.deals[idx].Stage, stage, currentUser(r).Name, utcNow()})
	}
	state.deals[idx].Stage = stage

	writeJSON(w, http.StatusOK, map[string]any{"deal": state.deals[idx]})
}

func updateDealStage(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateDealStage(w, r)
}

func (state *runtimeStore) deleteDeal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, d := range state.deals {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Deal not found")
		return
	}

	state.deals = append(state.deals[:idx], state.deals[idx+1:]...)

	keptActivities := state.activities[:0]
	for _, a := range state.activities {
		if a.DealID != id {
			keptActivities = append(keptActivities, a)
		}
	}
	state.activities = keptActivities

	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteDeal(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteDeal(w, r)
}
