package main

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func registerIdentityRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/sso/start", handleSSOStart)
	m.HandleFunc("GET /api/sso/callback", handleSSOCallback)
	m.HandleFunc("POST /api/sso/finish", handleSSOFinish)
	m.HandleFunc("GET /api/saml/start", handleSAMLStart)
	m.HandleFunc("GET /api/saml/metadata", handleSAML)
	m.HandleFunc("POST /api/saml/acs", handleSAML)
}

var identityHTTPClient = &http.Client{Timeout: 15 * time.Second}

func oidcConfiguration(ctx context.Context) (*oidc.Provider, oauth2.Config, error) {
	issuer := os.Getenv("OIDC_ISSUER_URL")
	if issuer == "" || os.Getenv("OIDC_CLIENT_ID") == "" {
		return nil, oauth2.Config{}, fmt.Errorf("Organization sign-in is not configured")
	}
	ctx = oidc.ClientContext(ctx, identityHTTPClient)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, oauth2.Config{}, fmt.Errorf("Identity provider is unavailable")
	}
	return provider, oauth2.Config{ClientID: os.Getenv("OIDC_CLIENT_ID"), ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"), Endpoint: provider.Endpoint(), RedirectURL: appURL() + "/api/sso/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}, nil
}
func handleSSOStart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	_, conf, err := oidcConfiguration(ctx)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	state, nonce, verifier := randomSecret(), randomSecret(), oauth2.GenerateVerifier()
	if _, err := storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at) VALUES(?,'','oidc_state',?)", hashSecret(state), time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339)); err != nil {
		writeError(w, 500, "Could not start sign-in")
		return
	}
	cookie, _ := encryptSecret(mustJSON(map[string]string{"state": state, "nonce": nonce, "verifier": verifier, "expires": time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)}))
	http.SetCookie(w, &http.Cookie{Name: "ecc_oidc", Value: cookie, Path: "/api/sso", HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, conf.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), 302)
}
func (state *runtimeStore) finishIdentity(w http.ResponseWriter, r *http.Request, email, provider string) error {
	u, ok := state.findUserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if !ok || u.Disabled || !ipAllowed(r, u) {
		return fmt.Errorf("Ask your administrator to create an active account for your identity-provider email")
	}
	if u.TwoFactorEnabled {
		token := randomSecret()
		_, err := storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at) VALUES(?,?,'sso',?)", hashSecret(token), u.ID, time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		http.Redirect(w, r, appURL()+"/account/sso?token="+url.QueryEscape(token), 303)
		return nil
	}
	if err := mintSession(w, r, u, true); err != nil {
		return err
	}
	securityEvent(r, u, "login."+provider)
	http.Redirect(w, r, appURL()+"/", 303)
	return nil
}

func finishIdentity(w http.ResponseWriter, r *http.Request, email, provider string) error {
	state, release := runtimeForCall(r)
	defer release()
	return state.finishIdentity(w, r, email, provider)
}
func (state *runtimeStore) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("ecc_oidc")
	if err != nil {
		writeError(w, 400, "Sign-in request expired")
		return
	}
	payload, err := decryptSecret(cookie.Value)
	var saved map[string]string
	if err != nil || json.Unmarshal([]byte(payload), &saved) != nil || saved["state"] != r.URL.Query().Get("state") {
		writeError(w, 400, "Invalid sign-in state")
		return
	}
	expiry, _ := time.Parse(time.RFC3339, saved["expires"])
	if !expiry.After(time.Now()) {
		writeError(w, 400, "Sign-in request expired")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	provider, conf, err := oidcConfiguration(ctx)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	ctx = oidc.ClientContext(ctx, identityHTTPClient)
	token, err := conf.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(saved["verifier"]))
	if err != nil {
		writeError(w, 400, "Identity provider rejected sign-in")
		return
	}
	raw, _ := token.Extra("id_token").(string)
	identity, err := provider.Verifier(&oidc.Config{ClientID: conf.ClientID}).Verify(ctx, raw)
	if err != nil || identity.Nonce != saved["nonce"] {
		writeError(w, 400, "Invalid identity token")
		return
	}
	var claims struct {
		Email    string
		Verified bool `json:"email_verified"`
	}
	if identity.Claims(&claims) != nil || !claims.Verified {
		writeError(w, 403, "A verified email is required")
		return
	}
	used, err := storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE hash=? AND purpose='oidc_state' AND used_at IS NULL AND expires_at>?", utcNow(), hashSecret(saved["state"]), utcNow())
	if err != nil {
		writeError(w, 500, "Could not finish sign-in")
		return
	}
	n, _ := used.RowsAffected()
	if n != 1 {
		writeError(w, 400, "Sign-in request already used or expired")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ecc_oidc", Value: "", Path: "/api/sso", HttpOnly: true, Secure: secureCookie(r), MaxAge: -1, SameSite: http.SameSiteLaxMode})
	if err = state.finishIdentity(w, r, claims.Email, "oidc"); err != nil {
		writeError(w, 403, err.Error())
	}
}

func handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSSOCallback(w, r)
}
func (state *runtimeStore) handleSSOFinish(w http.ResponseWriter, r *http.Request) {
	var req struct{ Token, Code string }
	if !decodeRequest(w, r, &req) {
		return
	}
	u, err := state.consumeAuthToken(r, req.Token, "sso")
	if err != nil || !verifySecondFactor(&u, req.Code) {
		writeError(w, 401, "Invalid or expired verification")
		return
	}
	if !ipAllowed(r, u) {
		writeError(w, 403, "Network is not allowed")
		return
	}
	storeDB(r).Exec("UPDATE auth_tokens SET used_at=? WHERE hash=?", utcNow(), hashSecret(req.Token))
	state.mu.Lock()
	state.users[strings.ToLower(u.Email)] = u
	state.mu.Unlock()
	if err = mintSession(w, r, u, true); err != nil {
		writeError(w, 500, "Could not complete sign-in")
		return
	}
	securityEvent(r, u, "login.sso_mfa")
	writeJSON(w, 200, statusResponse{"ok"})
}

func handleSSOFinish(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleSSOFinish(w, r)
}

type crmSAMLSession struct{}

func (crmSAMLSession) CreateSession(w http.ResponseWriter, r *http.Request, a *saml.Assertion) error {
	email := ""
	attribute := os.Getenv("SAML_EMAIL_ATTRIBUTE")
	if attribute == "" {
		attribute = "email"
	}
	for _, statement := range a.AttributeStatements {
		for _, item := range statement.Attributes {
			if item.Name == attribute || item.FriendlyName == attribute {
				for _, value := range item.Values {
					email = value.Value
					break
				}
			}
		}
	}
	if email == "" && a.Subject != nil && a.Subject.NameID != nil {
		email = a.Subject.NameID.Value
	}
	if !validEmail(email) {
		return fmt.Errorf("SAML email attribute is missing")
	}
	_, err := storeDB(r).Exec("INSERT INTO auth_tokens(hash,user_id,purpose,expires_at,used_at) VALUES(?,'','saml_replay',?,?)", hashSecret("saml:"+a.ID), time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339), utcNow())
	if err != nil {
		return fmt.Errorf("Assertion already used")
	}
	response := &bufferedResponse{header: make(http.Header)}
	if err := finishIdentity(response, r, email, "saml"); err != nil {
		return err
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	metadata(r).SAMLRedirect = response.header.Get("Location")
	return nil
}
func (crmSAMLSession) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	clearSessionCookies(w, r)
	return nil
}
func (crmSAMLSession) GetSession(r *http.Request) (samlsp.Session, error) {
	return nil, samlsp.ErrNoSession
}
func samlConfiguration(r *http.Request) (*samlsp.Middleware, error) {
	if os.Getenv("SAML_METADATA_URL") == "" {
		return nil, fmt.Errorf("SAML is not configured")
	}
	if !strings.HasPrefix(appURL(), "https://") {
		return nil, fmt.Errorf("SAML sign-in requires an HTTPS APP_URL")
	}
	pair, err := tls.LoadX509KeyPair(os.Getenv("SAML_CERT_FILE"), os.Getenv("SAML_KEY_FILE"))
	if err != nil {
		return nil, fmt.Errorf("SAML certificate configuration is invalid")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(appURL() + "/")
	metadataURL, err := url.Parse(os.Getenv("SAML_METADATA_URL"))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	idp, err := samlsp.FetchMetadata(ctx, &http.Client{Timeout: 10 * time.Second}, *metadataURL)
	if err != nil {
		return nil, fmt.Errorf("SAML metadata is unavailable")
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("SAML key must support signing")
	}
	sp, err := samlsp.New(samlsp.Options{URL: *base, Key: signer, Certificate: cert, IDPMetadata: idp, CookieSameSite: http.SameSiteNoneMode, AllowIDPInitiated: false})
	if err != nil {
		return nil, err
	}
	sp.ServiceProvider.MetadataURL = *base.ResolveReference(&url.URL{Path: "/api/saml/metadata"})
	sp.ServiceProvider.AcsURL = *base.ResolveReference(&url.URL{Path: "/api/saml/acs"})
	sp.ServiceProvider.DefaultRedirectURI = appURL() + "/"
	sp.Session = crmSAMLSession{}
	return sp, nil
}
func handleSAMLStart(w http.ResponseWriter, r *http.Request) {
	sp, err := samlConfiguration(r)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	sp.HandleStartAuthFlow(w, r)
}
func handleSAML(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	sp, err := samlConfiguration(r)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	response := &bufferedResponse{header: make(http.Header)}
	sp.ServeHTTP(response, r)
	if meta := metadata(r); meta != nil && meta.SAMLRedirect != "" {
		response.header.Set("Location", meta.SAMLRedirect)
		response.status = 303
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	if response.status == 0 {
		response.status = 200
	}
	w.WriteHeader(response.status)
	w.Write(response.body.Bytes())
}
