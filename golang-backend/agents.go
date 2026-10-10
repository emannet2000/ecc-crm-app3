package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type agentRequest struct {
	Name             string `json:"name"`
	AgentCode        string `json:"agentCode"`
	CountryCode      string `json:"countryCode"`
	ContractStatus   string `json:"contractStatus"`
	AgentStatus      string `json:"agentStatus"`
	StudentsReferred int    `json:"studentsReferred"`
	Notes            string `json:"notes"`
}

var validAgentContractStatuses = []string{"Signed", "Not Signed", "Pending"}
var validAgentStatuses = []string{"Active", "Inactive"}

func isValidAgentContractStatus(s string) bool {
	for _, v := range validAgentContractStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func isValidAgentStatus(s string) bool {
	for _, v := range validAgentStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func validateAgent(req agentRequest) map[string]string {
	fields := map[string]string{}

	name := strings.TrimSpace(req.Name)
	contract := strings.TrimSpace(req.ContractStatus)
	status := strings.TrimSpace(req.AgentStatus)

	if name == "" {
		fields["name"] = "Agent name is required"
	} else if len(name) > 200 {
		fields["name"] = "Name must be 200 characters or fewer"
	}

	if contract == "" {
		fields["contractStatus"] = "Contract status is required"
	} else if !isValidAgentContractStatus(contract) {
		fields["contractStatus"] = "Invalid contract status"
	}

	if status == "" {
		fields["agentStatus"] = "Agent status is required"
	} else if !isValidAgentStatus(status) {
		fields["agentStatus"] = "Invalid agent status"
	}

	if req.StudentsReferred < 0 {
		fields["studentsReferred"] = "Student count can't be negative"
	}

	return fields
}

func (state *runtimeStore) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit, offset := parseLimitOffset(r, 50, 0)

	all := state.listAgents()
	agentCounts, _ := state.linkedStudentCounts()
	filtered := make([]Agent, 0, len(all))
	for _, a := range all {
		a.StudentsReferred = agentCounts[a.ID]
		if q == "" ||
			strings.Contains(strings.ToLower(a.Name), q) ||
			strings.Contains(strings.ToLower(a.AgentCode), q) ||
			strings.Contains(strings.ToLower(a.CountryCode), q) {
			filtered = append(filtered, a)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": slicePage(filtered, offset, limit),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listAgentsHandler(w, r)
}

func (state *runtimeStore) getAgentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, ok := state.getAgentByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Agent not found")
		return
	}
	agentCounts, _ := state.linkedStudentCounts()
	a.StudentsReferred = agentCounts[a.ID]
	writeJSON(w, http.StatusOK, map[string]any{"agent": a})
}

func getAgentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getAgentHandler(w, r)
}

func (state *runtimeStore) createAgentHandler(w http.ResponseWriter, r *http.Request) {
	var req agentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := validateAgent(req); len(fields) > 0 {
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
	newAgent := Agent{
		RecordScope:      newRecordScope(r),
		ID:               newID("ag"),
		Name:             strings.TrimSpace(req.Name),
		AgentCode:        strings.TrimSpace(req.AgentCode),
		CountryCode:      strings.TrimSpace(strings.ToUpper(req.CountryCode)),
		ContractStatus:   strings.TrimSpace(req.ContractStatus),
		AgentStatus:      strings.TrimSpace(req.AgentStatus),
		StudentsReferred: req.StudentsReferred,
		Notes:            req.Notes,
		CreatedBy:        createdBy,
		CreatedAt:        now,
	}

	state.mu.Lock()
	state.agents = append(state.agents, newAgent)
	state.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{"agent": newAgent})
}

func createAgentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createAgentHandler(w, r)
}

func (state *runtimeStore) updateAgentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req agentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := validateAgent(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, a := range state.agents {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Agent not found")
		return
	}

	updated := state.agents[idx]
	updated.Name = strings.TrimSpace(req.Name)
	updated.AgentCode = strings.TrimSpace(req.AgentCode)
	updated.CountryCode = strings.TrimSpace(strings.ToUpper(req.CountryCode))
	updated.ContractStatus = strings.TrimSpace(req.ContractStatus)
	updated.AgentStatus = strings.TrimSpace(req.AgentStatus)
	updated.StudentsReferred = req.StudentsReferred
	updated.Notes = req.Notes

	state.agents[idx] = updated
	writeJSON(w, http.StatusOK, map[string]any{"agent": updated})
}

func updateAgentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateAgentHandler(w, r)
}

func (state *runtimeStore) deleteAgentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, a := range state.agents {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Agent not found")
		return
	}

	state.agents = append(state.agents[:idx], state.agents[idx+1:]...)

	for i := range state.students {
		if state.students[i].AgentID == id {
			state.students[i].AgentID = ""
			state.students[i].AgentName = ""
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteAgentHandler(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteAgentHandler(w, r)
}
