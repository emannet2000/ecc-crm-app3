package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func registerWorkspaceRoutes(mux *http.ServeMux) {
	registerAdministrationRoutes(mux)
	mux.HandleFunc("GET /api/clock", authMiddleware(handleWorkspaceClock))
	mux.HandleFunc("GET /api/control-center", authMiddleware(handleControlCenter))
	mux.HandleFunc("POST /api/admin/users", authMiddleware(handleCreateWorkspaceUser))
	mux.HandleFunc("PATCH /api/admin/users/{user}", authMiddleware(handleManageWorkspaceUser))
	mux.HandleFunc("POST /api/admin/teams", authMiddleware(handleCreateTeam))
	mux.HandleFunc("PUT /api/admin/organization", authMiddleware(handleOrganizationSettings))
	mux.HandleFunc("POST /api/invites/accept", handleAcceptInvite)
	mux.HandleFunc("PATCH /api/access/{entity}/{id}", authMiddleware(handleRecordAccess))
	mux.HandleFunc("PUT /api/admin/smtp", authMiddleware(handleSMTPSettings))
	mux.HandleFunc("POST /api/mail", authMiddleware(handleComposeMail))
	mux.HandleFunc("POST /api/mail/{message}/retry", authMiddleware(handleRetryMail))
	mux.HandleFunc("POST /api/notifications/{notification}/read", authMiddleware(handleReadNotification))
	mux.HandleFunc("PATCH /api/admin/webhooks/{webhook}", authMiddleware(handleManageWebhook))
	mux.HandleFunc("POST /api/admin/deliveries/{delivery}/retry", authMiddleware(handleRetryWebhook))
	mux.HandleFunc("POST /api/admin/webhooks", authMiddleware(handleWebhookSettings))
	mux.HandleFunc("POST /api/push/subscriptions", authMiddleware(handlePushSubscription))
	mux.HandleFunc("POST /api/preferences/notifications", authMiddleware(handleNotificationPreferences))
	mux.HandleFunc("GET /api/activities", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		items := append([]Activity{}, activities...)
		mu.RUnlock()
		items = sortRecords(items, r)
		limit, offset := parseLimitOffset(r, 25, 0)
		writeJSON(w, 200, map[string]any{"activities": slicePage(items, offset, limit), "total": len(items)})
	}))
	registerDocumentRoutes(mux)
	registerDataRoutes(mux)
	registerWorkflowRoutes(mux)
	registerPaymentRoutes(mux)
	registerIdentityRoutes(mux)
	registerExpansionRoutes(mux)
}
func queryObjects(r *http.Request, query string, args ...any) ([]map[string]any, error) {
	rows, err := storeDB(r).Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err = rows.Scan(targets...); err != nil {
			return nil, err
		}
		record := map[string]any{}
		for i, name := range columns {
			value := values[i]
			if data, ok := value.([]byte); ok {
				value = string(data)
			}
			record[name] = value
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
func (state *runtimeStore) handleControlCenter(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	result := map[string]any{"user": user, "role": user.Role, "features": integrationStatusFor(r)}
	queries := map[string]struct {
		query string
		args  []any
	}{
		"organization":  {"SELECT id,name,timezone,currency,ip_allowlist AS ipAllowlist FROM organizations WHERE id=?", []any{user.OrgID}},
		"teams":         {"SELECT id,name FROM teams WHERE org_id=? ORDER BY name", []any{user.OrgID}},
		"sessions":      {"SELECT id,created_at AS createdAt,last_seen AS lastSeen,expires_at AS expiresAt,refresh_expires_at AS refreshExpiresAt,ip,user_agent AS userAgent FROM sessions WHERE user_id=? AND revoked_at IS NULL AND version=? AND refresh_expires_at>? ORDER BY last_seen DESC", []any{user.ID, user.SessionVersion, utcNow()}},
		"notifications": {"SELECT id,kind,title,body,path,created_at AS createdAt,read_at AS readAt FROM notifications WHERE user_id=? ORDER BY created_at DESC LIMIT 100", []any{user.ID}},
		"mail":          {"SELECT id,kind,recipient,subject,status,attempts,last_error AS lastError,created_at AS createdAt,sent_at AS sentAt,contact_id AS contactId,invoice_id AS invoiceId FROM mail_outbox WHERE org_id=? AND (user_id=? OR ? IN ('admin','manager')) AND kind NOT IN ('password_reset','invite') ORDER BY created_at DESC LIMIT 100", []any{user.OrgID, user.ID, user.Role}},
		"entries":       {"SELECT id,category,record_id AS recordId,data,created_at AS createdAt FROM workspace_entries WHERE org_id=? AND category<>'ai_job' AND (user_id=? OR (user_id='' AND category NOT IN ('integration_secret','webhook'))) ORDER BY created_at DESC", []any{user.OrgID, user.ID}},
	}
	if isAdmin(user) {
		queries["webhooks"] = struct {
			query string
			args  []any
		}{"SELECT id,json_extract(data,'$.url') AS url,json_extract(data,'$.enabled') AS enabled FROM workspace_entries WHERE org_id=? AND category='webhook'", []any{user.OrgID}}
		queries["deliveries"] = struct {
			query string
			args  []any
		}{"SELECT id,webhook_id AS webhookId,status,attempts,last_error AS lastError,created_at AS createdAt FROM webhook_deliveries WHERE org_id=? ORDER BY created_at DESC LIMIT 100", []any{user.OrgID}}
		queries["security"] = struct {
			query string
			args  []any
		}{"SELECT id,event,ip,user_agent AS userAgent,user_id AS userId,occurred_at AS occurredAt FROM security_events WHERE org_id=? ORDER BY id DESC LIMIT 100", []any{user.OrgID}}
	}
	for key, query := range queries {
		records, err := queryObjects(r, query.query, query.args...)
		if err != nil {
			writeError(w, 500, "Could not load workspace settings")
			return
		}
		if key == "entries" {
			safe := []map[string]any{}
			for _, record := range records {
				id, _ := record["recordId"].(string)
				if id == "" || visibleRecordID(state.snapshot(), id) || record["category"] == "saved_view" {
					safe = append(safe, record)
				}
			}
			records = safe
		}
		result[key] = records
	}
	directory := []User{}
	state.mu.RLock()
	for _, candidate := range state.users {
		if candidate.OrgID == user.OrgID {
			if isAdmin(user) {
				directory = append(directory, candidate)
			} else {
				directory = append(directory, User{ID: candidate.ID, Name: candidate.Name, Email: candidate.Email, TeamID: candidate.TeamID, Role: candidate.Role})
			}
		}
	}
	state.mu.RUnlock()
	result["users"] = directory
	records := recordsOf(state.snapshot())
	options := map[string][]map[string]string{}
	for entity, items := range records {
		if entity == "users" {
			continue
		}
		options[entity] = []map[string]string{}
		for id, payload := range items {
			options[entity] = append(options[entity], map[string]string{"id": id, "label": recordLabel(payload)})
		}
	}
	result["records"] = options
	result["currentSessionId"] = r.Context().Value(ctxTokenID)
	writeJSON(w, 200, result)
}

func handleControlCenter(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleControlCenter(w, r)
}
func handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct{ Name string }
	if !decodeRequest(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		writeError(w, 400, "Enter a team name of up to 100 characters")
		return
	}
	id := newID("team")
	if _, err := storeDB(r).Exec("INSERT INTO teams VALUES(?,?,?)", id, currentUser(r).OrgID, name); err != nil {
		writeError(w, 409, "A team with this name already exists")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "name": name})
}
func teamInOrganization(r *http.Request, id string) bool {
	if id == "" {
		return true
	}
	var count int
	storeDB(r).QueryRow("SELECT count(*) FROM teams WHERE id=? AND org_id=?", id, currentUser(r).OrgID).Scan(&count)
	return count == 1
}
func (state *runtimeStore) handleCreateWorkspaceUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct{ Name, Email, Password, Role, TeamID string }
	if !decodeRequest(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Role == "" {
		req.Role = "member"
	}
	if req.TeamID == "" {
		req.TeamID = currentUser(r).TeamID
	}
	if req.Name == "" || len(req.Name) > 100 || !validEmail(req.Email) || !validRole(req.Role) || !teamInOrganization(r, req.TeamID) {
		writeError(w, 400, "Enter a name, valid email, role, and team")
		return
	}
	if _, exists := state.findUserByEmail(req.Email); exists {
		writeError(w, 409, "An account with that email already exists")
		return
	}
	invited := req.Password == ""
	if invited {
		req.Password = randomSecret()
	} else if message := passwordError(req.Password); message != "" {
		writeError(w, 400, message)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Could not create user")
		return
	}
	user := User{ID: newID("u"), Name: req.Name, Email: req.Email, Password: string(hash), OrgID: currentUser(r).OrgID, TeamID: req.TeamID, Role: req.Role, Disabled: invited}
	state.mu.Lock()
	state.users[user.Email] = user
	state.mu.Unlock()
	result := map[string]any{"user": user}
	if invited {
		secret := randomSecret()
		link := appURL() + "/accept-invite?token=" + secret
		_, err = storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at) VALUES(?,?,'invite',?)", hashSecret(secret), user.ID, time.Now().Add(72*time.Hour).UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, 500, "Could not create invitation")
			return
		}
		if err = queueMail(r, user, "invite", user.Email, "Join your ECC CRM workspace", "Accept this invitation within 72 hours:\n"+link, "", ""); err != nil {
			writeError(w, 500, "Could not queue invitation")
			return
		}
		result["inviteLink"] = link
	}
	securityEvent(r, user, "admin.user_created")
	writeJSON(w, 201, result)
}

func handleCreateWorkspaceUser(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleCreateWorkspaceUser(w, r)
}
func (state *runtimeStore) handleManageWorkspaceUser(w http.ResponseWriter, r *http.Request) {
	target, ok := state.adminTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		Name           *string
		Role           string
		TeamID         *string
		Disabled       *bool
		ResetTwoFactor bool
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Role != "" && !validRole(req.Role) || (req.TeamID != nil && !teamInOrganization(r, *req.TeamID)) {
		writeError(w, 400, "Invalid role or team")
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 100 {
			writeError(w, 400, "Enter a name of up to 100 characters")
			return
		}
		target.Name = name
	}
	if target.Disabled && req.Disabled != nil && !*req.Disabled {
		pending, err := pendingUserInvitation(r, target)
		if err != nil {
			writeError(w, 500, "Could not check invitation")
			return
		}
		if pending {
			writeError(w, 409, "This user must accept their invitation before activation")
			return
		}
	}
	removingAdmin := (req.Role != "" && req.Role != "admin") || (req.Disabled != nil && *req.Disabled)
	if target.Role == "admin" && !target.Disabled && removingAdmin {
		count := 0
		state.mu.RLock()
		for _, user := range state.users {
			if user.OrgID == target.OrgID && user.Role == "admin" && !user.Disabled {
				count++
			}
		}
		state.mu.RUnlock()
		if count <= 1 {
			writeError(w, 409, "Keep at least one active administrator")
			return
		}
	}
	if req.Role != "" {
		target.Role = req.Role
	}
	if req.TeamID != nil {
		target.TeamID = *req.TeamID
	}
	if req.Disabled != nil {
		target.Disabled = *req.Disabled
		if *req.Disabled {
			if _, err := storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE user_id=? AND used_at IS NULL", utcNow(), target.ID); err != nil {
				writeError(w, 500, "Could not revoke account access links")
				return
			}
		}
	}
	if req.ResetTwoFactor {
		target.TwoFactorEnabled = false
		target.TOTPSecret = ""
		target.RecoveryHashes = nil
	}
	if err := state.invalidateUserSessions(r, &target); err != nil {
		writeError(w, 500, "Could not revoke previous sessions")
		return
	}
	if err := securityEvent(r, currentUser(r), "admin.user_updated:"+target.ID); err != nil {
		writeError(w, 500, "Could not record account update")
		return
	}
	writeJSON(w, 200, map[string]any{"user": target})
}

func handleManageWorkspaceUser(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleManageWorkspaceUser(w, r)
}
func (state *runtimeStore) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct{ Token, Password, Name string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if message := passwordError(req.Password); message != "" {
		writeError(w, 400, message)
		return
	}
	user, err := state.consumeAuthToken(r, req.Token, "invite")
	if err != nil {
		writeError(w, 400, "Invitation is invalid or expired")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Could not activate account")
		return
	}
	user.Password = string(hash)
	user.Disabled = false
	user.SessionVersion++
	if strings.TrimSpace(req.Name) != "" {
		user.Name = strings.TrimSpace(req.Name)
	}
	state.mu.Lock()
	state.users[strings.ToLower(user.Email)] = user
	state.mu.Unlock()
	storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE hash=?", utcNow(), hashSecret(req.Token))
	securityEvent(r, user, "invite.accepted")
	writeJSON(w, 200, map[string]string{"message": "Invitation accepted. Sign in to your workspace."})
}

func handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleAcceptInvite(w, r)
}
func handleOrganizationSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct{ Name, Timezone, Currency, IPAllowlist string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, 400, "Workspace name is required")
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, 400, "Select a valid timezone")
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if !validCurrency(req.Currency) {
		writeError(w, 400, "Choose a supported currency")
		return
	}
	if !validateIPAllowlist(req.IPAllowlist, requestIP(r)) {
		writeError(w, 400, "Use valid IP addresses/CIDRs and include your current network")
		return
	}
	_, err := storeDB(r).Exec("UPDATE organizations SET name=?,timezone=?,currency=?,ip_allowlist=? WHERE id=?", strings.TrimSpace(req.Name), req.Timezone, req.Currency, req.IPAllowlist, currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not save organization")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func (state *runtimeStore) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	entity, id := r.PathValue("entity"), r.PathValue("id")
	items := recordsOf(state.snapshot())[entity]
	payload, exists := items[id]
	if !exists {
		writeError(w, 404, "Record not found")
		return
	}
	var scope RecordScope
	json.Unmarshal([]byte(payload), &scope)
	if !canWrite(scope, user) {
		writeError(w, 403, "You cannot change this record's access")
		return
	}
	var req struct{ OwnerID, TeamID, Visibility string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Visibility != "organization" && req.Visibility != "team" && req.Visibility != "private" {
		writeError(w, 400, "Choose organization, team, or private visibility")
		return
	}
	if req.OwnerID == "" {
		req.OwnerID = scope.OwnerID
	}
	owner, exists := state.findUserByID(req.OwnerID)
	if !exists || owner.OrgID != user.OrgID || !teamInOrganization(r, req.TeamID) {
		writeError(w, 400, "Owner and team must belong to your workspace")
		return
	}
	if !isManager(user) && req.OwnerID != user.ID {
		writeError(w, 403, "Only managers can reassign ownership")
		return
	}
	state.setRecordScope(entity, id, RecordScope{user.OrgID, req.OwnerID, req.TeamID, req.Visibility})
	writeJSON(w, 200, statusResponse{"ok"})
}

func handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRecordAccess(w, r)
}
func (state *runtimeStore) setRecordScope(entity, id string, scope RecordScope) {
	state.mu.Lock()
	defer state.mu.Unlock()
	update := func(items any) {
		value := reflect.ValueOf(items)
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i)
			if item.FieldByName("ID").String() == id {
				item.FieldByName("RecordScope").Set(reflect.ValueOf(scope))
				ownerName := ""
				for _, u := range state.users {
					if u.ID == scope.OwnerID {
						ownerName = u.Name
					}
				}
				for _, name := range []string{"Owner", "AssignedTo"} {
					field := item.FieldByName(name)
					if field.IsValid() && field.CanSet() && ownerName != "" {
						field.SetString(ownerName)
					}
				}
				return
			}
		}
	}
	switch entity {
	case "contacts":
		update(state.contacts)
	case "deals":
		update(state.deals)
	case "tasks":
		update(state.tasks)
	case "schools":
		update(state.schools)
	case "students":
		update(state.students)
	case "agents":
		update(state.agents)
	case "leads":
		update(state.leads)
	case "cases":
		update(state.cases)
	case "documents":
		update(state.documents)
	case "invoices":
		update(state.invoices)
	case "payments":
		update(state.payments)
	case "partners":
		update(state.partners)
	case "activities":
		update(state.activities)
	}
}

func setRecordScope(entity, id string, scope RecordScope) {
	state, release := runtimeForCall(nil)
	defer release()
	state.setRecordScope(entity, id, scope)
}

func handleWorkspaceClock(w http.ResponseWriter, r *http.Request) {
	var unread int
	if storeDB(r).QueryRow("SELECT count(*) FROM notifications WHERE user_id=? AND read_at IS NULL", currentUser(r).ID).Scan(&unread) != nil {
		writeError(w, 500, "Could not load workspace clock")
		return
	}
	writeJSON(w, 200, map[string]any{"today": organizationToday(r), "unread": unread})
}
