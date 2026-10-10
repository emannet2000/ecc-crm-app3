package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
)

// Older fixtures edit and inspect package variables. Commit those explicit fixture
// edits before a request and reload assertions from SQL afterward; production
// requests never publish their private unit of work into these bootstrap values.
var fixtureBaseline diskStore

func serveFixtureRequest(h http.Handler, w http.ResponseWriter, r *http.Request) {
	commitFixtureEdits()
	h.ServeHTTP(w, r)
	if database != nil {
		saved, err := readDatabase(database)
		if err != nil {
			panic(err)
		}
		restoreStore(saved)
		fixtureBaseline = cloneStore()
	}
}

func reloadFixtureAssertions() {
	saved, err := readDatabase(database)
	if err != nil {
		panic(err)
	}
	restoreStore(saved)
	fixtureBaseline = cloneStore()
}

func commitFixtureEdits() {
	if database != nil && fixtureBaseline.Users != nil {
		current := cloneStore()
		if !reflect.DeepEqual(current, fixtureBaseline) {
			committed, err := readDatabase(database)
			if err != nil {
				panic(err)
			}
			var desired diskStore
			json.Unmarshal([]byte(mustJSON(committed)), &desired)
			for _, change := range changesBetween(fixtureBaseline, current, "fixture") {
				if change.Entity == "users" {
					continue
				}
				payload := recordsOf(current)[change.Entity][change.RecordID]
				if payload != "" {
					if err = replaceSavedRecord(&desired, change.Entity, change.RecordID, payload); err != nil {
						panic(err)
					}
				} else {
					field := reflect.ValueOf(&desired).Elem().FieldByName(strings.ToUpper(change.Entity[:1]) + change.Entity[1:])
					out := reflect.MakeSlice(field.Type(), 0, field.Len())
					for i := 0; i < field.Len(); i++ {
						if idOfValue(field.Index(i)) != change.RecordID {
							out = reflect.Append(out, field.Index(i))
						}
					}
					field.Set(out)
				}
			}
			for key, u := range current.Users {
				if !reflect.DeepEqual(u, fixtureBaseline.Users[key]) {
					desired.Users[key] = u
				}
			}
			for key, v := range current.Revoked {
				if v != fixtureBaseline.Revoked[key] {
					desired.Revoked[key] = v
				}
			}
			tx, err := database.Begin()
			if err != nil {
				panic(err)
			}
			if err = saveStoreDeltaTx(tx, committed, desired, nil); err != nil {
				tx.Rollback()
				panic(err)
			}
			if err = tx.Commit(); err != nil {
				panic(err)
			}
		}
	}
}
