package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func adminCreateUser(t *testing.T, h http.Handler, admin []*http.Cookie, name, role, password string) User {
	t.Helper()
	w := featureRequest(h, "POST", "/api/admin/users", map[string]string{"name": name, "email": name + "@example.com", "role": role, "password": password}, admin)
	assertStatus(t, w, 201)
	var out struct{ User User }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.User
}

func adminVersionRequest(h http.Handler, method, path string, data any, cookies []*http.Cookie, version string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(mustJSON(data)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", version)
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}

func TestSystemAdministrationRequiresAdminAndIsolatesWorkspaces(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	for _, role := range []string{"manager", "member", "viewer"} {
		u := adminCreateUser(t, h, admin, role, role, "AccessTestPassword!")
		login := featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "AccessTestPassword!"}, nil)
		assertStatus(t, login, 200)
		cookies := login.Result().Cookies()
		assertStatus(t, featureRequest(h, "GET", "/api/admin/system", nil, cookies), 403)
		for _, action := range []string{"revoke-sessions", "invitation", "password-reset"} {
			assertStatus(t, featureRequest(h, "POST", "/api/admin/users/u_1/"+action, map[string]any{}, cookies), 403)
		}
		assertStatus(t, featureRequest(h, "PATCH", "/api/admin/users/"+u.ID, map[string]string{"role": "admin"}, cookies), 403)
	}
	w := featureRequest(h, "POST", "/api/register", map[string]string{"name": "Foreign administrator", "email": "foreign-admin@example.com", "password": "ForeignAdminPassword!"}, nil)
	assertStatus(t, w, 201)
	foreign, _ := findUserByEmail("foreign-admin@example.com")
	w = featureRequest(h, "GET", "/api/admin/system", nil, admin)
	assertStatus(t, w, 200)
	var result struct {
		Users        []struct{ ID, AccessVersion, Status string }
		AccessLevels []accessLevel
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 4 || len(result.AccessLevels) != 4 {
		t.Fatal(w.Body.String())
	}
	for _, user := range result.Users {
		if user.AccessVersion == "" || user.Status != "Active" {
			t.Fatal("missing account state")
		}
	}
	for _, secret := range []string{"password", "access_hash", "refresh_hash", "recoveryHashes", foreign.Email} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("admin payload includes %s", secret)
		}
	}
	for _, action := range []string{"revoke-sessions", "invitation", "password-reset"} {
		assertStatus(t, featureRequest(h, "POST", "/api/admin/users/"+foreign.ID+"/"+action, map[string]any{}, admin), 404)
	}
	assertStatus(t, featureRequest(h, "PATCH", "/api/admin/users/"+foreign.ID, map[string]string{"role": "viewer"}, admin), 404)
	assertStatus(t, featureRequest(h, "GET", "/api/admin/system", nil, nil), 401)
}

func TestAdminAccessChangesRevokeSessionsAndRejectStaleEdits(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	u := adminCreateUser(t, h, admin, "managed", "member", "AccessTestPassword!")
	stored, _ := findUserByID(u.ID)
	version := userAccessVersion(stored)
	login := featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "AccessTestPassword!"}, nil)
	assertStatus(t, login, 200)
	cookies := login.Result().Cookies()
	token, err := issueToken(stored)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/admin/users/" + u.ID
	assertStatus(t, adminVersionRequest(h, "PATCH", path, map[string]string{"role": "viewer", "teamId": "", "name": "New name"}, admin, version), 200)
	updated, _ := findUserByID(u.ID)
	if updated.Name != "New name" || updated.TeamID != "" || updated.Role != "viewer" {
		t.Fatal("account changes not saved")
	}
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, cookies), 401)
	assertStatus(t, featureRequest(h, "POST", "/api/session/refresh", nil, cookies), 401)
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	assertStatus(t, w, 401)
	assertStatus(t, adminVersionRequest(h, "PATCH", path, map[string]string{"role": "admin"}, admin, version), 409)
	assertStatus(t, adminVersionRequest(h, "PATCH", path, map[string]string{"role": "superadmin"}, admin, userAccessVersion(updated)), 400)
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]string{"teamId": "foreign_team"}, admin), 400)
	assertStatus(t, featureRequest(h, "PATCH", "/api/admin/users/u_1", map[string]bool{"disabled": true}, admin), 409)
	assertStatus(t, featureRequest(h, "PATCH", "/api/admin/users/u_1", map[string]string{"role": "manager"}, admin), 409)
	login = featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "AccessTestPassword!"}, nil)
	assertStatus(t, login, 200)
	assertStatus(t, featureRequest(h, "POST", path+"/revoke-sessions", map[string]any{}, admin), 200)
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, login.Result().Cookies()), 401)
	w = featureRequest(h, "GET", "/api/admin/system", nil, admin)
	if !strings.Contains(w.Body.String(), "admin.sessions_revoked:"+u.ID) || !strings.Contains(w.Body.String(), `"actor":"demo@northwind.dev"`) {
		t.Fatal("missing actor audit")
	}
}

func TestAdminInvitationRenewalAndCancellation(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	w := featureRequest(h, "POST", "/api/admin/users", map[string]string{"name": "Pending admin", "email": "pending@example.com", "role": "admin"}, admin)
	assertStatus(t, w, 201)
	var created struct {
		User       User
		InviteLink string
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	old, _ := url.Parse(created.InviteLink)
	path := "/api/admin/users/" + created.User.ID
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]bool{"disabled": false}, admin), 409)
	// A disabled invited administrator is not the last active administrator.
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]string{"role": "member"}, admin), 200)
	_, err := database.Exec("UPDATE auth_tokens SET expires_at='2000-01-01T00:00:00Z' WHERE user_id=?", created.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	w = featureRequest(h, "GET", "/api/admin/system", nil, admin)
	if !strings.Contains(w.Body.String(), "Invitation expired") {
		t.Fatal(w.Body.String())
	}
	w = featureRequest(h, "POST", path+"/invitation", map[string]any{}, admin)
	assertStatus(t, w, 200)
	var renewed struct{ InviteLink string }
	json.Unmarshal(w.Body.Bytes(), &renewed)
	next, _ := url.Parse(renewed.InviteLink)
	accept := func(token string) *httptest.ResponseRecorder {
		return featureRequest(h, "POST", "/api/invites/accept", map[string]string{"token": token, "password": "AcceptedPassword!"}, nil)
	}
	assertStatus(t, accept(old.Query().Get("token")), 400)
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]bool{"disabled": true}, admin), 200)
	assertStatus(t, accept(next.Query().Get("token")), 400)
	assertStatus(t, featureRequest(h, "POST", path+"/invitation", map[string]any{}, admin), 409)
	account, _ := findUserByID(created.User.ID)
	if !account.Disabled {
		t.Fatal("cancelled invitation activated an account")
	}
	assertStatus(t, featureRequest(h, "POST", path+"/password-reset", map[string]any{}, admin), 409)
}

func TestAdminPasswordRecoveryAndTransactionalFailure(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	u := adminCreateUser(t, h, admin, "recovery", "member", "RecoveryTestPassword!")
	path := "/api/admin/users/" + u.ID
	w := featureRequest(h, "POST", path+"/password-reset", map[string]any{}, admin)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "token") || strings.Contains(w.Body.String(), "Link") {
		t.Fatal("admin received recovery credential")
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM auth_tokens WHERE user_id=? AND purpose='reset' AND used_at IS NULL", u.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing recovery token: %v", err)
	}
	login := featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "RecoveryTestPassword!"}, nil)
	assertStatus(t, login, 200)
	if _, err := database.Exec(`CREATE TRIGGER reject_admin_event BEFORE INSERT ON security_events WHEN NEW.event LIKE 'admin.user_updated:%' BEGIN SELECT RAISE(ABORT,'test audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, featureRequest(h, "PATCH", path, map[string]bool{"disabled": true}, admin), 500)
	account, _ := findUserByID(u.ID)
	if account.Disabled {
		t.Fatal("failed transaction changed account")
	}
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, login.Result().Cookies()), 200)
	if err := database.QueryRow("SELECT COUNT(*) FROM auth_tokens WHERE user_id=? AND purpose='reset' AND used_at IS NULL", u.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed transaction revoked recovery link")
	}
}

func TestAccessLevelsEnforceRecordVisibilityAndOwnership(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	for _, level := range accessLevels {
		t.Run(level.ID, func(t *testing.T) {
			account := adminCreateUser(t, h, admin, "scope-"+level.ID, level.ID, "AccessLevelPassword!")
			stored, _ := findUserByID(account.ID)
			token, err := issueToken(stored)
			if err != nil {
				t.Fatal(err)
			}
			for _, visibility := range []string{"private", "team", "organization", "owned", "foreign"} {
				scope := RecordScope{OrgID: stored.OrgID, OwnerID: "u_1", TeamID: stored.TeamID, Visibility: visibility}
				if visibility == "owned" {
					scope.OwnerID, scope.Visibility = stored.ID, "private"
				}
				if visibility == "foreign" {
					scope.OrgID, scope.Visibility = "another_workspace", "organization"
				}
				id := "access_" + level.ID + "_" + visibility
				mu.Lock()
				contacts = append(contacts, Contact{RecordScope: scope, ID: id, Name: "Access test", Email: id + "@example.com", Stage: "Lead"})
				mu.Unlock()
				if err := saveStore(); err != nil {
					t.Fatal(err)
				}
				request := func(method, path string) *httptest.ResponseRecorder {
					r := httptest.NewRequest(method, path, strings.NewReader(mustJSON(map[string]string{"name": "Edited record", "email": id + "@example.com", "stage": "Lead"})))
					r.Header.Set("Authorization", "Bearer "+token)
					r.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					serveFixtureRequest(h, w, r)
					return w
				}
				readStatus := 200
				if visibility == "foreign" || visibility == "private" && level.ID != "admin" && level.ID != "manager" {
					readStatus = 404
				}
				assertStatus(t, request("GET", "/api/records/contacts/"+id), readStatus)
				writeStatus := 200
				if level.ID == "viewer" {
					writeStatus = 403
				} else if readStatus == 404 {
					writeStatus = 404
				} else if level.ID == "member" && visibility != "owned" {
					writeStatus = 403
				}
				assertStatus(t, request("PUT", "/api/contacts/"+id), writeStatus)
			}
		})
	}
}
