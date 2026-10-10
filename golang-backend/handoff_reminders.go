package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func remindPendingHandoffs(db DBRunner, s diskStore, day func(string, int) string) error {
	rows, err := db.Query("SELECT data FROM workspace_entries WHERE category='application_handoff' AND json_extract(data,'$.status')='pending'")
	if err != nil {
		return err
	}
	pending := []ApplicationHandoff{}
	for rows.Next() {
		var raw string
		var h ApplicationHandoff
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal([]byte(raw), &h)
		}
		if err != nil {
			break
		}
		pending = append(pending, h)
	}
	if rows.Err() != nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	cases := map[string]Case{}
	for _, c := range s.Cases {
		cases[c.ID] = c
	}
	people := map[string]User{}
	for _, u := range s.Users {
		people[u.User.ID] = u.User
	}
	for _, h := range pending {
		c, exists := cases[h.CaseID]
		if !exists {
			continue
		}
		today := day(c.OrgID, 0)
		created, e := time.Parse(time.RFC3339, h.CreatedAt)
		overdue := h.DueDate < today
		if !overdue && (e != nil || time.Since(created) < 24*time.Hour) {
			continue
		}
		title := "Handoff awaiting acceptance"
		if overdue {
			title = "Overdue application handoff"
		}
		body := fmt.Sprintf("%s: awaiting %s. Next action: %s. Due %s. The current owner remains responsible.", c.CaseNumber, h.ToName, h.NextAction, h.DueDate)
		recipients := map[string]bool{h.FromID: true, h.ToID: true}
		for id := range recipients {
			u, ok := people[id]
			if id == "" || !ok || u.Disabled || !canRead(c.RecordScope, u) {
				continue
			}
			if _, err = db.Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), c.OrgID, id, "handoff", title, body, "/cases/"+c.ID+"#handoff", "handoff-reminder:"+h.ID+":"+today, utcNow()); err != nil {
				return err
			}
		}
	}
	return nil
}
