package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func utcNow() string { return time.Now().UTC().Format(time.RFC3339) }
func randomSecret() string {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data[:])
}
func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(value, "@") && len(value) <= 254
}
func passwordError(value string) string {
	if len(value) < 12 {
		return "Use at least 12 characters"
	}
	if len(value) > 72 {
		return "Use no more than 72 bytes"
	}
	return ""
}
func appURL() string {
	if configured := os.Getenv("APP_URL"); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "7000"
	}
	return "http://localhost:" + port
}
func secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.HasPrefix(appURL(), "https://") || os.Getenv("COOKIE_SECURE") == "true"
}
func validOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host == r.Host || (strings.TrimRight(origin, "/") == appURL())
}
func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		ip, ipErr := netip.ParseAddr(host)
		if err == nil && ipErr == nil && prefix.Contains(ip) {
			forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
			if forwardedIP, err := netip.ParseAddr(forwarded); err == nil {
				return forwardedIP.String()
			}
		}
	}
	return host
}
func ipAllowed(r *http.Request, user User) bool {
	var allowed string
	if err := storeDB(r).QueryRow("SELECT ip_allowlist FROM organizations WHERE id=?", user.OrgID).Scan(&allowed); err != nil {
		return false
	}
	if allowed == "" {
		return true
	}
	ip, err := netip.ParseAddr(requestIP(r))
	if err != nil {
		return false
	}
	for _, raw := range strings.FieldsFunc(allowed, func(c rune) bool { return c == ',' || c == '\n' || c == ' ' }) {
		raw = strings.TrimSpace(raw)
		if prefix, err := netip.ParsePrefix(raw); err == nil && prefix.Contains(ip) {
			return true
		}
		if candidate, err := netip.ParseAddr(raw); err == nil && candidate == ip {
			return true
		}
	}
	return false
}
func (state *runtimeStore) findUserByID(id string) (User, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, user := range state.users {
		if user.ID == id {
			return user, true
		}
	}
	return User{}, false
}

func findUserByID(id string) (User, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.findUserByID(id)
}
func authorizeWorkspaceRequest(next http.HandlerFunc, w http.ResponseWriter, r *http.Request, user User, sessionID string) {
	if user.Disabled || !ipAllowed(r, user) {
		writeError(w, 403, "This account or network is not allowed")
		return
	}
	ctx := context.WithValue(r.Context(), requestUserKey, user)
	ctx = context.WithValue(ctx, ctxUserID, user.ID)
	ctx = context.WithValue(ctx, ctxUserEmail, user.Email)
	ctx = context.WithValue(ctx, ctxTokenID, sessionID)
	r = r.WithContext(ctx)
	if meta := metadata(r); meta != nil {
		meta.Actor = user.Email
		meta.OrgID = user.OrgID
	}
	scopedHandler(next, w, r, user)
}
func setSessionCookies(w http.ResponseWriter, r *http.Request, access, refresh string, remember bool) {
	maxAge := 0
	expires := time.Time{}
	if remember {
		maxAge = 30 * 24 * 3600
		expires = time.Now().Add(30 * 24 * time.Hour)
	}
	http.SetCookie(w, &http.Cookie{Name: "ecc_session", Value: access, Path: "/", HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: 0})
	http.SetCookie(w, &http.Cookie{Name: "ecc_refresh", Value: refresh, Path: "/api/session", HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: maxAge, Expires: expires})
}
func clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	for _, entry := range []struct{ name, path string }{{"ecc_session", "/"}, {"ecc_refresh", "/api/session"}} {
		http.SetCookie(w, &http.Cookie{Name: entry.name, Value: "", Path: entry.path, HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	}
}
func mintSession(w http.ResponseWriter, r *http.Request, user User, remember bool) error {
	access, refresh := randomSecret(), randomSecret()
	id := newID("session")
	now := utcNow()
	lifetime := 24 * time.Hour
	if remember {
		lifetime = 30 * 24 * time.Hour
	}
	_, err := storeDB(r).Exec(`INSERT INTO sessions(id,user_id,org_id,access_hash,refresh_hash,created_at,last_seen,expires_at,refresh_expires_at,ip,user_agent,version,persistent) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, user.ID, user.OrgID, hashSecret(access), hashSecret(refresh), now, now, time.Now().Add(15*time.Minute).UTC().Format(time.RFC3339), time.Now().Add(lifetime).UTC().Format(time.RFC3339), requestIP(r), r.UserAgent(), user.SessionVersion, remember)
	if err != nil {
		return err
	}
	setSessionCookies(w, r, access, refresh, remember)
	return nil
}
func cookieIdentity(r *http.Request) (User, string, error) {
	cookie, err := r.Cookie("ecc_session")
	if err != nil {
		return User{}, "", err
	}
	var id, userID, expires string
	var version int
	err = storeDB(r).QueryRow("SELECT id,user_id,expires_at,version FROM sessions WHERE access_hash=? AND revoked_at IS NULL", hashSecret(cookie.Value)).Scan(&id, &userID, &expires, &version)
	if err != nil {
		return User{}, "", err
	}
	when, err := time.Parse(time.RFC3339, expires)
	if err != nil || !when.After(time.Now()) {
		return User{}, "", fmt.Errorf("session expired")
	}
	user, exists := committedUser(r, "id", userID)
	if !exists || user.Disabled || user.SessionVersion != version {
		return User{}, "", fmt.Errorf("session revoked")
	}
	_, err = storeDB(r).Exec("UPDATE sessions SET last_seen=? WHERE id=?", utcNow(), id)
	if err != nil {
		return User{}, "", err
	}
	return user, id, nil
}
func (state *runtimeStore) handleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("ecc_refresh")
	if err != nil {
		clearSessionCookies(w, r)
		writeError(w, 401, "Sign in to continue")
		return
	}
	var id, userID, expiry string
	var version int
	var persistent bool
	err = storeDB(r).QueryRow("SELECT id,user_id,refresh_expires_at,version,persistent FROM sessions WHERE refresh_hash=? AND revoked_at IS NULL", hashSecret(cookie.Value)).Scan(&id, &userID, &expiry, &version, &persistent)
	when, parseErr := time.Parse(time.RFC3339, expiry)
	user, exists := state.findUserByID(userID)
	if err != nil || parseErr != nil || !when.After(time.Now()) || !exists || user.Disabled || user.SessionVersion != version || !ipAllowed(r, user) {
		clearSessionCookies(w, r)
		writeError(w, 401, "Session expired. Please sign in again")
		return
	}
	access, refresh := randomSecret(), randomSecret()
	_, err = storeDB(r).Exec("UPDATE sessions SET access_hash=?,refresh_hash=?,last_seen=?,expires_at=? WHERE id=?", hashSecret(access), hashSecret(refresh), utcNow(), time.Now().Add(15*time.Minute).UTC().Format(time.RFC3339), id)
	if err != nil {
		writeError(w, 500, "Could not refresh session")
		return
	}
	setSessionCookies(w, r, access, refresh, persistent)
	writeJSON(w, 200, authResponse{Token: "cookie-session", User: user})
}

func handleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSessionRefresh(w, r)
}
func handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("ecc_session"); err == nil {
		storeDB(r).Exec("UPDATE sessions SET revoked_at=? WHERE access_hash=?", utcNow(), hashSecret(cookie.Value))
	}
	if cookie, err := r.Cookie("ecc_refresh"); err == nil {
		storeDB(r).Exec("UPDATE sessions SET revoked_at=? WHERE refresh_hash=?", utcNow(), hashSecret(cookie.Value))
	}
	clearSessionCookies(w, r)
	writeJSON(w, 200, statusResponse{"ok"})
}
func handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("session")
	result, err := storeDB(r).Exec("UPDATE sessions SET revoked_at=? WHERE id=? AND user_id=?", utcNow(), id, user.ID)
	if err != nil {
		writeError(w, 500, "Could not revoke session")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, 404, "Session not found")
		return
	}
	if id == r.Context().Value(ctxTokenID) {
		clearSessionCookies(w, r)
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func securityEvent(r *http.Request, user User, event string) error {
	_, err := storeDB(r).Exec("INSERT INTO security_events(org_id,user_id,event,ip,user_agent,occurred_at) VALUES(?,?,?,?,?,?)", user.OrgID, user.ID, event, requestIP(r), r.UserAgent(), utcNow())
	return err
}
func loginLocked(r *http.Request, email string) (bool, time.Time) {
	var expiry string
	err := storeDB(r).QueryRow("SELECT locked_until FROM login_attempts WHERE key=?", hashSecret(strings.ToLower(email))).Scan(&expiry)
	if err != nil {
		return false, time.Time{}
	}
	when, _ := time.Parse(time.RFC3339, expiry)
	return when.After(time.Now()), when
}
func failedLogin(r *http.Request, user User, email string) {
	key := hashSecret(strings.ToLower(email))
	now := utcNow()
	locked := time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339)
	storeDB(r).Exec(`INSERT INTO login_attempts VALUES(?,1,?,'') ON CONFLICT(key) DO UPDATE SET failures=CASE WHEN window_start<? THEN 1 ELSE failures+1 END,window_start=CASE WHEN window_start<? THEN excluded.window_start ELSE window_start END,locked_until=CASE WHEN failures>=4 AND window_start>=? THEN ? ELSE '' END`, key, now, time.Now().Add(-15*time.Minute).UTC().Format(time.RFC3339), time.Now().Add(-15*time.Minute).UTC().Format(time.RFC3339), time.Now().Add(-15*time.Minute).UTC().Format(time.RFC3339), locked)
	securityEvent(r, user, "login.failed")
	if meta := metadata(r); meta != nil {
		meta.PersistFailure = true
	}
}
func (state *runtimeStore) handleSecureLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email, Password, OTP string
		Remember             bool
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if locked, expiry := loginLocked(r, email); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(expiry).Seconds())))
		writeError(w, 429, "Too many failed attempts. Try again in 15 minutes")
		return
	}
	user, exists := state.findUserByEmail(email)
	if !exists || user.Disabled || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		failedLogin(r, user, email)
		writeError(w, 401, "Invalid email or password")
		return
	}
	if !ipAllowed(r, user) {
		securityEvent(r, user, "login.ip_denied")
		if meta := metadata(r); meta != nil {
			meta.PersistFailure = true
		}
		writeError(w, 403, "Sign-in from this network is not allowed")
		return
	}
	if user.TwoFactorEnabled && !verifySecondFactor(&user, req.OTP) {
		failedLogin(r, user, email)
		writeError(w, 401, "Enter a valid verification or recovery code")
		return
	}
	state.mu.Lock()
	state.users[email] = user
	state.mu.Unlock()
	if err := mintSession(w, r, user, req.Remember); err != nil {
		writeError(w, 500, "Could not create session")
		return
	}
	storeDB(r).Exec("DELETE FROM login_attempts WHERE key=?", hashSecret(email))
	securityEvent(r, user, "login.succeeded")
	if meta := metadata(r); meta != nil {
		meta.Actor = user.Email
		meta.OrgID = user.OrgID
	}
	writeJSON(w, 200, authResponse{Token: "cookie-session", User: user})
}

func handleSecureLogin(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSecureLogin(w, r)
}
func (state *runtimeStore) handleSecureRegister(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ALLOW_SIGNUP") == "false" {
		writeError(w, 403, "Registration is disabled. Ask an administrator for an invitation")
		return
	}
	var req struct {
		Name, Email, Password string
		Remember              bool
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	if name == "" || !validEmail(email) {
		writeError(w, 400, "Enter a name and valid email address")
		return
	}
	if message := passwordError(req.Password); message != "" {
		writeError(w, 400, message)
		return
	}
	if _, exists := state.findUserByEmail(email); exists {
		writeError(w, 409, "An account with that email already exists")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Could not create account")
		return
	}
	orgID, teamID := newID("org"), newID("team")
	if _, err = storeDB(r).Exec("INSERT INTO organizations(id,name,created_at) VALUES(?,?,?)", orgID, name+"'s workspace", utcNow()); err != nil {
		writeError(w, 500, "Could not create workspace")
		return
	}
	if _, err = storeDB(r).Exec("INSERT INTO teams VALUES(?,?,?)", teamID, orgID, "General"); err != nil {
		writeError(w, 500, "Could not create team")
		return
	}
	user := User{ID: newID("u"), Name: name, Email: email, Password: string(hash), OrgID: orgID, TeamID: teamID, Role: "admin"}
	state.mu.Lock()
	state.users[email] = user
	state.mu.Unlock()
	if err = mintSession(w, r, user, req.Remember); err != nil {
		writeError(w, 500, "Could not create session")
		return
	}
	securityEvent(r, user, "account.created")
	if meta := metadata(r); meta != nil {
		meta.Actor = user.Email
		meta.OrgID = user.OrgID
	}
	writeJSON(w, 201, authResponse{Token: "cookie-session", User: user})
}

func handleSecureRegister(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSecureRegister(w, r)
}
func encryptSecret(plain string) (string, error) {
	key := sha256.Sum256(jwtSecret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}
func decryptSecret(encoded string) (string, error) {
	value, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256(jwtSecret)
	block, _ := aes.NewCipher(key[:])
	aead, _ := cipher.NewGCM(block)
	if len(value) < aead.NonceSize() {
		return "", fmt.Errorf("invalid encrypted secret")
	}
	plain, err := aead.Open(nil, value[:aead.NonceSize()], value[aead.NonceSize():], nil)
	return string(plain), err
}
func totpCode(secret string, step int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 15
	number := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", number%1000000)
}
func validTOTP(secret, code string) bool {
	step := time.Now().Unix() / 30
	for _, offset := range []int64{-1, 0, 1} {
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, step+offset)), []byte(strings.TrimSpace(code))) == 1 {
			return true
		}
	}
	return false
}
func verifySecondFactor(user *User, code string) bool {
	if secret, err := decryptSecret(user.TOTPSecret); err == nil {
		step := time.Now().Unix() / 30
		for _, offset := range []int64{-1, 0, 1} {
			candidate := step + offset
			if candidate > user.LastTOTPStep && subtle.ConstantTimeCompare([]byte(totpCode(secret, candidate)), []byte(strings.TrimSpace(code))) == 1 {
				user.LastTOTPStep = candidate
				return true
			}
		}
	}

	hash := hashSecret(strings.TrimSpace(code))
	for i, saved := range user.RecoveryHashes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(saved)) == 1 {
			user.RecoveryHashes = append(user.RecoveryHashes[:i], user.RecoveryHashes[i+1:]...)
			return true
		}
	}
	return false
}
func (state *runtimeStore) handleTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var req struct{ Action, Password, Code string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		writeError(w, 403, "Current password is incorrect")
		return
	}
	switch req.Action {
	case "setup":
		if user.TwoFactorEnabled {
			writeError(w, 409, "Two-factor authentication is already enabled")
			return
		}
		raw := make([]byte, 20)
		rand.Read(raw)
		secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		encrypted, err := encryptSecret(secret)
		if err != nil {
			writeError(w, 500, "Could not generate authenticator secret")
			return
		}
		user.TOTPSecret = encrypted
		state.mu.Lock()
		state.users[strings.ToLower(user.Email)] = user
		state.mu.Unlock()
		uri := "otpauth://totp/" + url.PathEscape("ECC CRM:"+user.Email) + "?secret=" + secret + "&issuer=ECC%20CRM&algorithm=SHA1&digits=6&period=30"
		writeJSON(w, 200, map[string]any{"secret": secret, "uri": uri})
		return
	case "confirm":
		if user.TwoFactorEnabled {
			writeError(w, 409, "Two-factor authentication is already enabled")
			return
		}
		secret, err := decryptSecret(user.TOTPSecret)
		if err != nil || !validTOTP(secret, req.Code) {
			writeError(w, 400, "Enter the six-digit code from your authenticator")
			return
		}
		codes := []string{}
		user.RecoveryHashes = []string{}
		for i := 0; i < 10; i++ {
			code := randomSecret()[:16]
			codes = append(codes, code)
			user.RecoveryHashes = append(user.RecoveryHashes, hashSecret(code))
		}
		user.LastTOTPStep = time.Now().Unix() / 30
		user.TwoFactorEnabled = true
		user.SessionVersion++
		state.mu.Lock()
		state.users[strings.ToLower(user.Email)] = user
		state.mu.Unlock()
		if err = mintSession(w, r, user, true); err != nil {
			writeError(w, 500, "Could not refresh session")
			return
		}
		securityEvent(r, user, "two_factor.enabled")
		writeJSON(w, 200, map[string]any{"recoveryCodes": codes})
		return
	case "disable":
		if !verifySecondFactor(&user, req.Code) {
			writeError(w, 403, "Enter a valid verification or recovery code")
			return
		}
		user.TwoFactorEnabled = false
		user.TOTPSecret = ""
		user.RecoveryHashes = nil
		user.SessionVersion++
		state.mu.Lock()
		state.users[strings.ToLower(user.Email)] = user
		state.mu.Unlock()
		if err := mintSession(w, r, user, true); err != nil {
			writeError(w, 500, "Could not refresh session")
			return
		}
		securityEvent(r, user, "two_factor.disabled")
		writeJSON(w, 200, statusResponse{"ok"})
		return
	default:
		writeError(w, 400, "Unknown security action")
	}
}

func handleTwoFactor(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleTwoFactor(w, r)
}
func (state *runtimeStore) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string }
	if !decodeRequest(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if user, exists := state.findUserByEmail(email); exists && !user.Disabled {
		secret := randomSecret()
		storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE user_id=? AND purpose='reset' AND used_at IS NULL", utcNow(), user.ID)
		_, err := storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at) VALUES(?,?,'reset',?)", hashSecret(secret), user.ID, time.Now().Add(30*time.Minute).UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, 500, "Could not process recovery request")
			return
		}
		if err = queueMail(r, user, "password_reset", user.Email, "Reset your ECC CRM password", "Use this single-use link within 30 minutes:\n"+appURL()+"/reset-password?token="+url.QueryEscape(secret), "", ""); err != nil {
			writeError(w, 500, "Could not queue recovery email")
			return
		}
		securityEvent(r, user, "password_reset.requested")
	}
	writeJSON(w, 200, map[string]string{"message": "If an active account exists, a recovery email will be sent. Contact your administrator if it does not arrive."})
}

func handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleForgotPassword(w, r)
}
func (state *runtimeStore) consumeAuthToken(r *http.Request, secret, purpose string) (User, error) {
	var userID, expires string
	err := storeDB(r).QueryRow("SELECT user_id,expires_at FROM auth_tokens WHERE hash=? AND purpose=? AND used_at IS NULL", hashSecret(secret), purpose).Scan(&userID, &expires)
	when, parseErr := time.Parse(time.RFC3339, expires)
	user, exists := state.findUserByID(userID)
	if err != nil || parseErr != nil || !when.After(time.Now()) || !exists {
		return User{}, fmt.Errorf("This link is invalid or expired")
	}
	return user, nil
}

func consumeAuthToken(r *http.Request, secret, purpose string) (User, error) {
	state, release := runtimeForCall(r)
	defer release()
	return state.consumeAuthToken(r, secret, purpose)
}
func (state *runtimeStore) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct{ Token, Password string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if message := passwordError(req.Password); message != "" {
		writeError(w, 400, message)
		return
	}
	user, err := state.consumeAuthToken(r, req.Token, "reset")
	if err != nil || user.Disabled {
		writeError(w, 400, "This link is invalid or expired")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Could not change password")
		return
	}
	user.Password = string(hash)
	user.SessionVersion++
	state.mu.Lock()
	state.users[strings.ToLower(user.Email)] = user
	state.mu.Unlock()
	storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE user_id=? AND purpose='reset' AND used_at IS NULL", utcNow(), user.ID)
	securityEvent(r, user, "password_reset.completed")
	clearSessionCookies(w, r)
	writeJSON(w, 200, map[string]string{"message": "Password changed. Sign in with your new password."})
}

func handleResetPassword(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleResetPassword(w, r)
}
func initializePersistentSecret() {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = []byte(secret)
		return
	}
	root := filepath.Join(findProjectRoot(), "data")
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		root = filepath.Dir(path)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		panic(err)
	}
	path := filepath.Join(root, "auth-secret")
	saved, err := os.ReadFile(path)
	if err == nil && len(saved) >= 32 {
		jwtSecret = saved
		return
	}
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	secret := []byte(randomSecret())
	file, err := os.CreateTemp(root, ".auth-secret-*")
	if err != nil {
		panic(err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err = file.Write(secret); err != nil {
		file.Close()
		panic(err)
	}
	if err = file.Sync(); err != nil {
		file.Close()
		panic(err)
	}
	if err = file.Close(); err != nil {
		panic(err)
	}
	if err = os.Link(tempPath, path); err != nil && !os.IsExist(err) {
		panic(err)
	}
	saved, err = os.ReadFile(path)
	if err != nil || len(saved) < 32 {
		panic("Persistent auth secret is unavailable or invalid")
	}
	jwtSecret = saved
}
