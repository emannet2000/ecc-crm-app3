package main

import (
	"encoding/json"
	"testing"
)

func TestStaffObservationsStayWithinWorkspaceAndOwner(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "usability-adviser")
	other := addPermissionUser(t, "member", admin.OrgID, "usability-other")
	foreign := addPermissionUser(t, "admin", "usability-foreign", "usability-foreign-admin")
	h := expansionTestMux()
	path := "/api/usability/feedback"
	observation := UsabilityObservation{Role: "adviser", Task: "student-review", Completed: true, Seconds: 75, Comments: "Needed help finding the portal reply"}
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 201)
	count := func(u User, expected int) {
		t.Helper()
		w := rawUserRequest(t, h, u, "GET", path, nil)
		assertStatus(t, w, 200)
		var out struct{ Observations []json.RawMessage }
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Observations) != expected {
			t.Fatalf("unexpected accessible observations: %s", w.Body.String())
		}
	}
	count(member, 1)
	count(admin, 1)
	count(other, 0)
	count(foreign, 0)
	observation.Role = "finance"
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 400)
	observation.Role = "adviser"
	observation.Seconds = -1
	assertStatus(t, rawUserRequest(t, h, member, "POST", path, observation), 400)
}
