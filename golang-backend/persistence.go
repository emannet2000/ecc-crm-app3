package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Password hashes are persisted separately from the public User representation.
type storedUser struct {
	LastTOTPStep   int64
	User           User
	Password       string
	SessionVersion int
	TOTPSecret     string
	RecoveryHashes []string
}
type diskStore struct {
	Version    int
	Users      map[string]storedUser
	Contacts   []Contact
	Deals      []Deal
	Activities []Activity
	Tasks      []Task
	Schools    []School
	Students   []Student
	Agents     []Agent
	Leads      []Lead
	Cases      []Case
	Documents  []Document
	Invoices   []Invoice
	Payments   []Payment
	Partners   []Partner
	Revoked    map[string]time.Time
}

var mutationMu sync.Mutex
var dataPath string

func (state *runtimeStore) snapshot() diskStore {
	state.mu.RLock()
	defer state.mu.RUnlock()
	state.revokedMu.RLock()
	defer state.revokedMu.RUnlock()
	savedUsers := make(map[string]storedUser, len(state.users))
	for key, user := range state.users {
		savedUsers[key] = storedUser{User: user, Password: user.Password, SessionVersion: user.SessionVersion, TOTPSecret: user.TOTPSecret, RecoveryHashes: user.RecoveryHashes, LastTOTPStep: user.LastTOTPStep}
	}
	return diskStore{1, savedUsers, state.contacts, state.deals, state.activities, state.tasks, state.schools, state.students, state.agents, state.leads, state.cases, state.documents, state.invoices, state.payments, state.partners, state.revokedTokens}
}

func snapshot() diskStore {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.snapshot()
}

func (state *runtimeStore) restoreStore(saved diskStore) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.users = make(map[string]User, len(saved.Users))
	for key, stored := range saved.Users {
		user := stored.User
		user.Password = stored.Password
		user.SessionVersion = stored.SessionVersion
		user.TOTPSecret = stored.TOTPSecret
		user.LastTOTPStep = stored.LastTOTPStep
		user.RecoveryHashes = stored.RecoveryHashes
		state.users[key] = user
	}
	state.contacts, state.deals, state.activities, state.tasks = saved.Contacts, saved.Deals, saved.Activities, saved.Tasks
	state.schools, state.students, state.agents, state.leads = saved.Schools, saved.Students, saved.Agents, saved.Leads
	state.cases, state.documents, state.invoices, state.payments, state.partners = saved.Cases, saved.Documents, saved.Invoices, saved.Payments, saved.Partners
	state.revokedMu.Lock()
	state.revokedTokens = saved.Revoked
	if state.revokedTokens == nil {
		state.revokedTokens = map[string]time.Time{}
	}
	state.revokedMu.Unlock()
}

func restoreStore(saved diskStore) {
	state, release := runtimeForCall(nil)
	defer release()
	state.restoreStore(saved)
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *bufferedResponse) Header() http.Header { return w.header }
func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *bufferedResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(p)
}

func persistMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || independentRead(r) {
			next.ServeHTTP(w, r)
			return
		}
		meta := &requestMeta{}
		r = r.WithContext(context.WithValue(r.Context(), requestMetaKey, meta))
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			saved, err := readIdentityDatabase(database)
			if err != nil {
				writeError(w, 503, "Database unavailable")
				return
			}
			r, _ = attachRuntime(r, saved)
			if strings.HasPrefix(r.URL.Path, "/api/portal/") {
				full, err := readDatabase(database)
				if err != nil {
					writeError(w, 503, "Database unavailable")
					return
				}
				r, _ = attachRuntime(r, full)
			}
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/api/saml/acs" && r.URL.Path != "/api/payments/stripe/webhook" && !validOrigin(r) {
			writeError(w, 403, "Cross-origin changes are not allowed")
			return
		}
		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, 503, "Database unavailable")
			return
		}
		defer tx.Rollback()
		r = r.WithContext(context.WithValue(r.Context(), requestTxKey, tx))
		if err = lockSQLWriter(r); err != nil {
			writeError(w, 503, "Database busy; retry the change")
			return
		}
		before, err := readIdentityDatabase(tx)
		if strings.HasPrefix(r.URL.Path, "/api/portal/") || strings.Contains(r.URL.Path, "webhook") || strings.HasPrefix(r.URL.Path, "/api/inbound/") {
			before, err = readDatabase(tx)
		}
		if err != nil {
			writeError(w, 503, "Could not load committed records")
			return
		}
		r, state := attachRuntime(r, before)

		response := &bufferedResponse{header: make(http.Header)}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = 200
		}
		success := (response.status >= 200 && response.status < 300) || (r.URL.Path == "/api/saml/acs" && response.status == 303)
		if success {
			actor := meta.Actor
			if actor == "" {
				actor = requestActor(r)
			}
			before = state.before
			after := state.snapshot()
			events := changesBetween(before, after, actor)
			err = archiveDeleted(r, before, after, actor)
			if err == nil {
				err = saveStoreDeltaTx(tx, before, after, events)
			}
			if err == nil {
				err = state.queueChangeEvents(r, events)
			}
			if err == nil {
				err = tx.Commit()
			}
		} else {
			state.restoreStore(before)
			if meta.PersistFailure {
				err = tx.Commit()
			}
		}
		if err != nil || !success {
			state.restoreStore(before)
			for _, cleanup := range meta.Rollback {
				cleanup()
			}
		}
		if err != nil {
			writeError(w, 500, "Could not save changes to the database")
			return
		}
		if success {
			for _, action := range meta.AfterCommit {
				action()
			}
		}
		for key, values := range response.header {
			w.Header()[key] = values
		}
		w.WriteHeader(response.status)
		w.Write(response.body.Bytes())
	})
}
