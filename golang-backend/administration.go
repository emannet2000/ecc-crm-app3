package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Administration is scoped to the signed-in administrator's organization.
// Session hashes, passwords, recovery secrets and auth tokens never enter this view.
func registerAdministrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/system", authMiddleware(handleSystemAdministration))
	mux.HandleFunc("POST /api/admin/users/{user}/revoke-sessions", authMiddleware(handleAdminRevokeSessions))
	mux.HandleFunc("POST /api/admin/users/{user}/invitation", authMiddleware(handleAdminInvitation))
	mux.HandleFunc("POST /api/admin/users/{user}/password-reset", authMiddleware(handleAdminPasswordReset))
}

type accessLevel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Read        string `json:"read"`
	Write       string `json:"write"`
	Recovery    bool   `json:"recovery"`
	Users       bool   `json:"users"`
	Settings    bool   `json:"settings"`
}

var accessLevels = []accessLevel{
	{"admin", "Administrator", "Manages workspace security, people, integrations and all CRM records.", "All workspace records", "All workspace records", true, true, true},
	{"manager", "Manager", "Oversees CRM operations, record ownership, client portals and recovery.", "All workspace records", "All workspace records", true, false, false},
	{"member", "Member", "Creates records and works on records they own.", "Own records and records shared with their team or workspace", "Own records; can create new records", false, false, false},
	{"viewer", "Viewer", "Reads shared records and reports; can manage their own account security.", "Own records and records shared with their team or workspace", "Read only", false, false, false},
}

func userAccessVersion(user User) string {
	return versionOf(map[string]any{"id": user.ID, "name": user.Name, "role": user.Role, "teamId": user.TeamID, "disabled": user.Disabled, "twoFactor": user.TwoFactorEnabled, "sessionVersion": user.SessionVersion})
}

func checkUserAccessVersion(w http.ResponseWriter, r *http.Request, user User) bool {
	if version := strings.Trim(r.Header.Get("If-Match"), `"`); version != "" && version != userAccessVersion(user) {
		writeError(w, 409, "This user's access changed. Refresh administration before saving again.")
		return false
	}
	return true
}

func (state *runtimeStore) adminTarget(w http.ResponseWriter, r *http.Request) (User, bool) {
	if !requireAdmin(w, r) {
		return User{}, false
	}
	user, ok := state.findUserByID(r.PathValue("user"))
	if !ok || user.OrgID != currentUser(r).OrgID {
		writeError(w, 404, "User not found")
		return User{}, false
	}
	return user, checkUserAccessVersion(w, r, user)
}

func adminTarget(w http.ResponseWriter, r *http.Request) (User, bool) {
	state, release := runtimeForCall(r)
	defer release()
	return state.adminTarget(w, r)
}

func (state *runtimeStore) handleSystemAdministration(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	user := currentUser(r)
	result := map[string]any{"currentUserId": user.ID, "accessLevels": accessLevels}
	queries := map[string]string{
		"organization": "SELECT id,name,timezone,currency,ip_allowlist AS ipAllowlist FROM organizations WHERE id=?",
		"teams":        "SELECT id,name FROM teams WHERE org_id=? ORDER BY name",
		"users": `SELECT u.id,u.name,u.email,u.role,u.team_id AS teamId,u.disabled,u.two_factor_enabled AS twoFactorEnabled,
		(SELECT MAX(s.last_seen) FROM sessions s WHERE s.user_id=u.id) AS lastSeen,
		(SELECT COUNT(*) FROM sessions s WHERE s.user_id=u.id AND s.org_id=u.org_id AND s.revoked_at IS NULL AND s.version=u.session_version AND s.refresh_expires_at>strftime('%Y-%m-%dT%H:%M:%SZ','now') AND u.disabled=0) AS activeSessions,
		(SELECT MAX(t.expires_at) FROM auth_tokens t WHERE t.user_id=u.id AND t.purpose='invite' AND t.used_at IS NULL) AS invitationExpiresAt
		FROM users u WHERE u.org_id=? ORDER BY u.name COLLATE NOCASE,u.email`,
		"sessions": `SELECT s.id,s.user_id AS userId,u.name,u.email,s.ip,s.user_agent AS userAgent,s.last_seen AS lastSeen,s.created_at AS createdAt
		FROM sessions s JOIN users u ON u.id=s.user_id AND u.org_id=s.org_id WHERE s.org_id=? AND u.disabled=0 AND s.revoked_at IS NULL AND s.version=u.session_version AND s.refresh_expires_at>strftime('%Y-%m-%dT%H:%M:%SZ','now') ORDER BY s.last_seen DESC LIMIT 100`,
		"security": "SELECT e.event,e.user_id AS userId,u.name,e.ip,e.occurred_at AS occurredAt FROM security_events e LEFT JOIN users u ON e.user_id=u.id AND e.org_id=u.org_id WHERE e.org_id=? ORDER BY e.id DESC LIMIT 100",
		"changes":  "SELECT actor,action,label,record_id AS userId,occurred_at AS occurredAt FROM audit_events WHERE org_id=? AND entity='users' ORDER BY id DESC LIMIT 100",
	}
	for key, query := range queries {
		rows, err := queryObjects(r, query, user.OrgID)
		if err != nil {
			writeError(w, 500, "Could not load system administration")
			return
		}
		if key == "users" {
			for _, row := range rows {
				account, exists := state.findUserByID(fmt.Sprint(row["id"]))
				if !exists {
					continue
				}
				row["accessVersion"] = userAccessVersion(account)
				row["disabled"], row["twoFactorEnabled"] = account.Disabled, account.TwoFactorEnabled
				status := "Active"
				if account.Disabled {
					status = "Disabled"
					if expires, ok := row["invitationExpiresAt"].(string); ok {
						status = "Invited"
						if expires <= utcNow() {
							status = "Invitation expired"
						}
					}
				}
				row["status"] = status
			}
		}
		result[key] = rows
	}
	writeJSON(w, 200, result)
}

func handleSystemAdministration(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSystemAdministration(w, r)
}

func (state *runtimeStore) invalidateUserSessions(r *http.Request, target *User) error {
	if _, err := storeDB(r).Exec("UPDATE sessions SET revoked_at=? WHERE user_id=? AND org_id=? AND revoked_at IS NULL", utcNow(), target.ID, target.OrgID); err != nil {
		return err
	}
	// Version changes also revoke bearer tokens and refresh credentials.
	target.SessionVersion++
	state.mu.Lock()
	state.users[strings.ToLower(target.Email)] = *target
	state.mu.Unlock()
	return nil
}

func invalidateUserSessions(r *http.Request, target *User) error {
	state, release := runtimeForCall(r)
	defer release()
	return state.invalidateUserSessions(r, target)
}

func (state *runtimeStore) handleAdminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	target, ok := state.adminTarget(w, r)
	if !ok {
		return
	}
	if err := state.invalidateUserSessions(r, &target); err != nil {
		writeError(w, 500, "Could not revoke sessions")
		return
	}
	if err := securityEvent(r, currentUser(r), "admin.sessions_revoked:"+target.ID); err != nil {
		writeError(w, 500, "Could not record session revocation")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}

func handleAdminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleAdminRevokeSessions(w, r)
}

func pendingUserInvitation(r *http.Request, target User) (bool, error) {
	var count int
	err := storeDB(r).QueryRow("SELECT COUNT(*) FROM auth_tokens WHERE user_id=? AND purpose='invite' AND used_at IS NULL", target.ID).Scan(&count)
	return count > 0, err
}

func (state *runtimeStore) handleAdminInvitation(w http.ResponseWriter, r *http.Request) {
	target, ok := state.adminTarget(w, r)
	if !ok {
		return
	}
	pending, err := pendingUserInvitation(r, target)
	if err != nil {
		writeError(w, 500, "Could not load invitation")
		return
	}
	if !target.Disabled || !pending {
		writeError(w, 409, "Only pending invitations can be reissued")
		return
	}
	issueAdminAccessLink(w, r, target, "invite", 72*time.Hour)
}

func handleAdminInvitation(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleAdminInvitation(w, r)
}

func (state *runtimeStore) handleAdminPasswordReset(w http.ResponseWriter, r *http.Request) {
	target, ok := state.adminTarget(w, r)
	if !ok {
		return
	}
	if target.Disabled {
		writeError(w, 409, "Enable this account before requesting password recovery")
		return
	}
	issueAdminAccessLink(w, r, target, "reset", 30*time.Minute)
}

func handleAdminPasswordReset(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleAdminPasswordReset(w, r)
}

func issueAdminAccessLink(w http.ResponseWriter, r *http.Request, target User, purpose string, ttl time.Duration) {
	secret, expires := randomSecret(), time.Now().Add(ttl).UTC().Format(time.RFC3339)
	if _, err := storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE user_id=? AND purpose=? AND used_at IS NULL", utcNow(), target.ID, purpose); err != nil {
		writeError(w, 500, "Could not replace access link")
		return
	}
	if _, err := storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at) VALUES(?,?,?,?)", hashSecret(secret), target.ID, purpose, expires); err != nil {
		writeError(w, 500, "Could not create access link")
		return
	}
	path, kind, subject := "/accept-invite", "invite", "Join your ECC CRM workspace"
	if purpose == "reset" {
		path, kind, subject = "/reset-password", "password_reset", "Reset your ECC CRM password"
	}
	link := appURL() + path + "?token=" + secret
	if err := queueMail(r, target, kind, target.Email, subject, "Use this single-use link before "+expires+":\n"+link, "", ""); err != nil {
		writeError(w, 500, "Could not queue access email")
		return
	}
	if err := securityEvent(r, currentUser(r), "admin."+purpose+"_link_created:"+target.ID); err != nil {
		writeError(w, 500, "Could not record access link creation")
		return
	}
	// Recovery links are sent only to the account owner. Administrators can copy
	// invitations, but cannot obtain a password-recovery credential from this API.
	result := map[string]any{"expiresAt": expires, "message": "Recovery email queued. Delivery requires configured SMTP."}
	if purpose == "invite" {
		result["inviteLink"], result["message"] = link, "Invitation replaced and email queued. The previous link is invalid."
	}
	writeJSON(w, 200, result)
}
