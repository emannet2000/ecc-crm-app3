package main

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"time"
)

// runtimeStore is a private unit of work, loaded from committed SQL for one
// request or worker transaction. It is never shared between live requests.
type runtimeStore struct {
	mu                   sync.RWMutex
	revokedMu            sync.RWMutex
	users                map[string]User
	contacts             []Contact
	deals                []Deal
	activities           []Activity
	tasks                []Task
	schools              []School
	students             []Student
	agents               []Agent
	leads                []Lead
	cases                []Case
	documents            []Document
	invoices             []Invoice
	payments             []Payment
	partners             []Partner
	revokedTokens        map[string]time.Time
	hiddenLedgerPayments []Payment
	before               diskStore
}

const runtimeStoreKey contextKey = "runtimeStore"

func runtimeForCall(r *http.Request) (*runtimeStore, func()) {
	if r != nil {
		if state, ok := r.Context().Value(runtimeStoreKey).(*runtimeStore); ok {
			return state, func() {}
		}
	}
	// Compatibility for seed/import routines and direct in-package tests only.
	// Production middleware installs a SQL-backed unit of work before handlers.
	mu.RLock()
	revokedMu.RLock()
	state := &runtimeStore{users: users, contacts: contacts, deals: deals, activities: activities, tasks: tasks, schools: schools, students: students, agents: agents, leads: leads, cases: cases, documents: documents, invoices: invoices, payments: payments, partners: partners, revokedTokens: revokedTokens, hiddenLedgerPayments: hiddenLedgerPayments}
	revokedMu.RUnlock()
	mu.RUnlock()
	return state, func() { publishLegacyRuntime(state) }
}
func publishLegacyRuntime(state *runtimeStore) {
	mu.Lock()
	defer mu.Unlock()
	revokedMu.Lock()
	defer revokedMu.Unlock()
	users, contacts, deals, activities, tasks = state.users, state.contacts, state.deals, state.activities, state.tasks
	schools, students, agents, leads = state.schools, state.students, state.agents, state.leads
	cases, documents, invoices, payments, partners = state.cases, state.documents, state.invoices, state.payments, state.partners
	revokedTokens = state.revokedTokens
}
func attachRuntime(r *http.Request, saved diskStore) (*http.Request, *runtimeStore) {
	state := &runtimeStore{}
	state.restoreStore(saved)
	state.before = state.cloneStore()
	return r.WithContext(context.WithValue(r.Context(), runtimeStoreKey, state)), state
}
func lockSQLWriter(r *http.Request) error {
	// Acquire SQLite's write reservation before reading the unit of work. A second
	// process waits here rather than reading a snapshot it cannot later upgrade.
	_, err := storeDB(r).Exec("INSERT INTO app_meta(key,value) VALUES('writer_lock','1') ON CONFLICT(key) DO UPDATE SET value=value")
	return err
}

func workerRuntime(ctx context.Context, org string) (DBRunner, *http.Request, *runtimeStore, func(), error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, func() {}, err
	}
	r, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal/worker", nil)
	r = withDBTransaction(r, tx)
	if err = lockSQLWriter(r); err != nil {
		tx.Rollback()
		return nil, nil, nil, func() {}, err
	}
	var saved diskStore
	if org == "" {
		saved, err = readDatabase(tx)
	} else {
		r = r.WithContext(context.WithValue(r.Context(), requestUserKey, User{OrgID: org, Role: "admin"}))
		saved, err = sqlWorkspace(r)
		if err == nil {
			identity, e := readIdentityDatabase(tx)
			err = e
			saved.Users, saved.Revoked = identity.Users, identity.Revoked
		}
	}
	if err != nil {
		tx.Rollback()
		return nil, nil, nil, func() {}, err
	}
	r, state := attachRuntime(r, saved)
	return tx, r, state, func() { tx.Rollback() }, nil
}
func todayForOrgDB(db rowQueryer, org string) string {
	var zone string
	db.QueryRow("SELECT timezone FROM organizations WHERE id=?", org).Scan(&zone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	return time.Now().In(location).Format("2006-01-02")
}

func legacyRuntimeView() *runtimeStore {
	return &runtimeStore{users: users, contacts: contacts, deals: deals, activities: activities, tasks: tasks, schools: schools, students: students, agents: agents, leads: leads, cases: cases, documents: documents, invoices: invoices, payments: payments, partners: partners, revokedTokens: revokedTokens, hiddenLedgerPayments: hiddenLedgerPayments}
}
func runtimeForReadCall(r *http.Request) (*runtimeStore, func()) {
	if r != nil {
		if state, ok := r.Context().Value(runtimeStoreKey).(*runtimeStore); ok {
			return state, func() {}
		}
	}
	return legacyRuntimeView(), func() {}
}

func claimDelivery(table, id string) bool {
	if table != "mail_outbox" && table != "webhook_deliveries" {
		return false
	}
	result, err := database.Exec("UPDATE "+table+" SET status='sending',next_attempt=? WHERE id=? AND status IN ('queued','waiting_configuration') AND (next_attempt='' OR next_attempt<=?)", time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339), id, utcNow())
	if err != nil {
		return false
	}
	n, err := result.RowsAffected()
	return err == nil && n == 1
}

func cloneStoreValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneStoreValue(value.Elem()))
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(cloneStoreValue(value.Index(i)))
		}
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneStoreValue(iter.Value()))
		}
		return out
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		out.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath == "" {
				out.Field(i).Set(cloneStoreValue(value.Field(i)))
			}
		}
		return out
	default:
		return value
	}
}
