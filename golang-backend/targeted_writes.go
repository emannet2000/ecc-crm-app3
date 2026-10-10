package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Contact edits affect no computed aggregates. Load just the selected contact
// and accessible email collisions, retaining the normal handler and SQL commit.
func targetedContactWrite(next http.HandlerFunc, w http.ResponseWriter, r *http.Request, user User) bool {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "api" || parts[1] != "contacts" || r.Method != "PUT" {
		return false
	}
	if user.Role == "viewer" {
		writeError(w, 403, "Your role has read-only access")
		return true
	}
	state, done := runtimeForCall(r)
	defer done()
	saved := state.snapshot()
	record, err := sqlRecord(r, "contacts", r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "Contact not found")
		return true
	}
	var contact Contact
	if json.Unmarshal([]byte(mustJSON(record)), &contact) != nil {
		writeError(w, 500, "Could not read contact")
		return true
	}
	if !canWrite(contact.RecordScope, user) {
		writeError(w, 403, "You can only edit records you own")
		return true
	}
	if !normalizeRequest(w, r) {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		writeError(w, 400, "Request is too large")
		return true
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var request contactRequest
	if json.Unmarshal(body, &request) != nil {
		writeError(w, 400, "Invalid request body")
		return true
	}
	clause, args := scopeSQL(r, "data")
	args = append(args, strings.TrimSpace(request.Email), contact.ID)
	rows, err := storeDB(r).Query("SELECT data FROM contacts WHERE "+clause+" AND lower(json_extract(data,'$.email'))=lower(?) AND id<>? LIMIT 1", args...)
	if err != nil {
		writeError(w, 503, "Could not validate contact")
		return true
	}
	saved.Contacts = []Contact{contact}
	for rows.Next() {
		var payload string
		var other Contact
		if err = rows.Scan(&payload); err == nil {
			err = json.Unmarshal([]byte(payload), &other)
		}
		if err != nil {
			break
		}
		saved.Contacts = append(saved.Contacts, other)
	}
	if rows.Err() != nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeError(w, 503, "Could not validate contact")
		return true
	}
	state.restoreStore(saved)
	state.before = state.cloneStore()
	if !checkRecordVersion(w, r, saved) {
		return true
	}
	next(w, r)
	return true
}
