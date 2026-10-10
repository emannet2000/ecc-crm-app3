package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var database *sql.DB
var entityTables = []string{"contacts", "deals", "activities", "tasks", "schools", "students", "agents", "leads", "cases", "documents", "invoices", "payments", "partners"}

func openDatabase(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	// Create with private permissions before SQLite opens the file.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String() + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_synchronous=FULL&_txlock=immediate"
	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*sql.DB, error) { db.Close(); return nil, err }
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version > 4 {
		return fail(fmt.Errorf("database schema %d is newer than this app", version))
	}
	tx, err := db.Begin()
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS app_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE COLLATE NOCASE, name TEXT NOT NULL, password_hash TEXT NOT NULL, session_version INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS revoked_tokens (id TEXT PRIMARY KEY, expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS audit_events (id INTEGER PRIMARY KEY AUTOINCREMENT, actor TEXT NOT NULL, action TEXT NOT NULL, entity TEXT NOT NULL, record_id TEXT NOT NULL, label TEXT NOT NULL, occurred_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS audit_events_recent ON audit_events(id DESC)`,
	}
	for _, table := range entityTables {
		statements = append(statements, `CREATE TABLE IF NOT EXISTS `+table+` (id TEXT PRIMARY KEY, sort_order INTEGER NOT NULL, data TEXT NOT NULL CHECK(json_valid(data)))`)
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return fail(err)
		}
	}
	if _, err = tx.Exec("PRAGMA user_version=4"); err != nil {
		return fail(err)
	}
	if err = migrateWorkspace(tx, version); err != nil {
		return fail(err)
	}
	if err = migrateExpansion(tx); err != nil {
		return fail(err)
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	return db, nil
}

func loadStore(path string) error {
	if database != nil {
		database.Close()
		database = nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dataPath = absolute
	db, err := openDatabase(absolute)
	if err != nil {
		return err
	}
	database = db
	ok := false
	defer func() {
		if !ok {
			db.Close()
			database = nil
		}
	}()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO app_meta(key,value) VALUES('writer_lock','1') ON CONFLICT(key) DO UPDATE SET value=value"); err != nil {
		return err
	}
	var initialized string
	err = tx.QueryRow("SELECT value FROM app_meta WHERE key='initialized'").Scan(&initialized)
	if err == sql.ErrNoRows {
		legacy := os.Getenv("LEGACY_DATA_FILE")
		if legacy == "" {
			legacy = filepath.Join(filepath.Dir(absolute), "crm.json")
		}
		payload, readErr := os.ReadFile(legacy)
		if readErr == nil {
			var saved diskStore
			if err = json.Unmarshal(payload, &saved); err != nil {
				return fmt.Errorf("legacy data could not be imported: %w", err)
			}
			if saved.Version != 1 || saved.Users == nil {
				return fmt.Errorf("unsupported legacy CRM data")
			}
			restoreStore(saved)
		} else if os.IsNotExist(readErr) {
			restoreStore(diskStore{Users: map[string]storedUser{}})
			seedData()
		} else {
			return readErr
		}
		normalizeWorkspaceData()
		applyComputedData()
		if err = saveDatabaseTx(tx, nil); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		saved, err := readDatabase(tx)
		if err != nil {
			return err
		}
		restoreStore(saved)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	ok = true
	return nil
}

func readRows[T any](db rowQueryer, table string) ([]T, error) {
	rows, err := db.Query("SELECT data FROM " + table + " ORDER BY sort_order,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []T{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return nil, err
		}
		var item T
		if err = json.Unmarshal(payload, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func readIdentityDatabase(db rowQueryer) (diskStore, error) {
	saved := diskStore{Version: 1, Users: map[string]storedUser{}, Revoked: map[string]time.Time{}}
	rows, err := db.Query("SELECT id,email,name,password_hash,session_version,org_id,team_id,role,disabled,two_factor_enabled,auth_data FROM users")
	if err != nil {
		return saved, err
	}
	for rows.Next() {
		var user User
		var authData string
		if err = rows.Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.SessionVersion, &user.OrgID, &user.TeamID, &user.Role, &user.Disabled, &user.TwoFactorEnabled, &authData); err != nil {
			rows.Close()
			return saved, err
		}
		var private struct {
			LastTOTPStep   int64
			TOTPSecret     string
			RecoveryHashes []string
		}
		if err = json.Unmarshal([]byte(authData), &private); err != nil {
			rows.Close()
			return saved, err
		}
		user.TOTPSecret = private.TOTPSecret
		user.LastTOTPStep = private.LastTOTPStep
		user.RecoveryHashes = private.RecoveryHashes
		saved.Users[strings.ToLower(strings.TrimSpace(user.Email))] = storedUser{User: user, Password: user.Password, SessionVersion: user.SessionVersion, TOTPSecret: user.TOTPSecret, RecoveryHashes: user.RecoveryHashes, LastTOTPStep: user.LastTOTPStep}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return saved, err
	}
	rows, err = db.Query("SELECT id,expires_at FROM revoked_tokens")
	if err != nil {
		return saved, err
	}
	for rows.Next() {
		var id, expiry string
		if err = rows.Scan(&id, &expiry); err != nil {
			rows.Close()
			return saved, err
		}
		when, parseErr := time.Parse(time.RFC3339Nano, expiry)
		if parseErr != nil {
			rows.Close()
			return saved, parseErr
		}
		saved.Revoked[id] = when
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return saved, err
	}
	return saved, nil
}

func readDatabase(db rowQueryer) (diskStore, error) {
	saved, err := readIdentityDatabase(db)
	if err != nil {
		return saved, err
	}
	if saved.Contacts, err = readRows[Contact](db, "contacts"); err != nil {
		return saved, err
	}
	if saved.Deals, err = readRows[Deal](db, "deals"); err != nil {
		return saved, err
	}
	if saved.Activities, err = readRows[Activity](db, "activities"); err != nil {
		return saved, err
	}
	if saved.Tasks, err = readRows[Task](db, "tasks"); err != nil {
		return saved, err
	}
	if saved.Schools, err = readRows[School](db, "schools"); err != nil {
		return saved, err
	}
	if saved.Students, err = readRows[Student](db, "students"); err != nil {
		return saved, err
	}
	if saved.Agents, err = readRows[Agent](db, "agents"); err != nil {
		return saved, err
	}
	if saved.Leads, err = readRows[Lead](db, "leads"); err != nil {
		return saved, err
	}
	if saved.Cases, err = readRows[Case](db, "cases"); err != nil {
		return saved, err
	}
	if saved.Documents, err = readRows[Document](db, "documents"); err != nil {
		return saved, err
	}
	if saved.Invoices, err = readRows[Invoice](db, "invoices"); err != nil {
		return saved, err
	}
	if saved.Payments, err = readRows[Payment](db, "payments"); err != nil {
		return saved, err
	}
	if saved.Partners, err = readRows[Partner](db, "partners"); err != nil {
		return saved, err
	}
	return saved, nil
}

func syncRows[T any](tx *sql.Tx, table string, items []T) error {
	rows, err := tx.Query("SELECT id FROM " + table)
	if err != nil {
		return err
	}
	stale := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		stale[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	statement, err := tx.Prepare("INSERT INTO " + table + " (id,sort_order,data) VALUES (?,?,?) ON CONFLICT(id) DO UPDATE SET sort_order=excluded.sort_order,data=excluded.data WHERE sort_order!=excluded.sort_order OR data!=excluded.data")
	if err != nil {
		return err
	}
	defer statement.Close()
	for order, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return err
		}
		var record struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(payload, &record); err != nil {
			return err
		}
		if record.ID == "" {
			return fmt.Errorf("%s record has no ID", table)
		}
		delete(stale, record.ID)
		if _, err = statement.Exec(record.ID, order, string(payload)); err != nil {
			return err
		}
	}
	for id := range stale {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE id=?", id); err != nil {
			return err
		}
	}
	return nil
}

func saveStore() error { return saveDatabase(nil) }
func saveDatabase(events []AuditEvent) error {
	if database == nil {
		return fmt.Errorf("database is not open")
	}
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = saveDatabaseTx(tx, events); err != nil {
		return err
	}
	return tx.Commit()
}

func saveDatabaseTx(tx *sql.Tx, events []AuditEvent) error {
	saved := snapshot()
	if err := trackCaseStages(tx, saved); err != nil {
		return err
	}
	var err error
	for _, stored := range saved.Users {
		u := stored.User
		private, _ := json.Marshal(struct {
			LastTOTPStep   int64
			TOTPSecret     string
			RecoveryHashes []string
		}{stored.LastTOTPStep, stored.TOTPSecret, stored.RecoveryHashes})
		_, err = tx.Exec(`INSERT INTO users(id,email,name,password_hash,session_version,org_id,team_id,role,disabled,two_factor_enabled,auth_data) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET email=excluded.email,name=excluded.name,password_hash=excluded.password_hash,session_version=excluded.session_version,org_id=excluded.org_id,team_id=excluded.team_id,role=excluded.role,disabled=excluded.disabled,two_factor_enabled=excluded.two_factor_enabled,auth_data=excluded.auth_data`, u.ID, u.Email, u.Name, stored.Password, stored.SessionVersion, u.OrgID, u.TeamID, u.Role, u.Disabled, u.TwoFactorEnabled, string(private))
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM revoked_tokens"); err != nil {
		return err
	}
	for id, expiry := range saved.Revoked {
		if expiry.After(time.Now()) {
			if _, err = tx.Exec("INSERT INTO revoked_tokens VALUES(?,?)", id, expiry.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		}
	}
	writers := []func() error{
		func() error { return syncRows(tx, "contacts", saved.Contacts) }, func() error { return syncRows(tx, "deals", saved.Deals) }, func() error { return syncRows(tx, "activities", saved.Activities) }, func() error { return syncRows(tx, "tasks", saved.Tasks) },
		func() error { return syncRows(tx, "schools", saved.Schools) }, func() error { return syncRows(tx, "students", saved.Students) }, func() error { return syncRows(tx, "agents", saved.Agents) }, func() error { return syncRows(tx, "leads", saved.Leads) },
		func() error { return syncRows(tx, "cases", saved.Cases) }, func() error { return syncRows(tx, "documents", saved.Documents) }, func() error { return syncRows(tx, "invoices", saved.Invoices) }, func() error { return syncRows(tx, "payments", saved.Payments) }, func() error { return syncRows(tx, "partners", saved.Partners) },
	}
	for _, write := range writers {
		if err = write(); err != nil {
			return err
		}
	}
	for _, event := range events {
		if _, err = tx.Exec("INSERT INTO audit_events(actor,action,entity,record_id,label,occurred_at,org_id,scope_data) VALUES(?,?,?,?,?,?,?,?)", event.Actor, event.Action, event.Entity, event.RecordID, event.Label, event.OccurredAt, event.OrgID, mustJSON(event.Scope)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT OR REPLACE INTO app_meta VALUES('initialized','1')"); err != nil {
		return err
	}
	return nil
}
