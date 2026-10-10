package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type processRequest struct {
	Method, Path, Token, Version string
	Body                         json.RawMessage
}
type processResponse struct {
	Status int
	Body   json.RawMessage
}

func TestSQLProcessChild(t *testing.T) {
	path := os.Getenv("ECC_SQL_PROCESS_DATABASE")
	if path == "" {
		t.Skip("subprocess helper")
	}
	jwtSecret = []byte("test-secret")
	if err := loadStore(path); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := featureMux()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var in processRequest
		if err := json.Unmarshal(scanner.Bytes(), &in); err != nil {
			t.Fatal(err)
		}
		out := processResponse{}
		if in.Method == "WORKER" {
			generateReminders()
			out.Status = 200
			out.Body = json.RawMessage(`{}`)
		} else {
			r := httptest.NewRequest(in.Method, in.Path, bytes.NewReader(in.Body))
			r.Header.Set("Authorization", "Bearer "+in.Token)
			r.Header.Set("Content-Type", "application/json")
			if in.Version != "" {
				r.Header.Set("If-Match", in.Version)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			out.Status = w.Code
			out.Body = append([]byte{}, w.Body.Bytes()...)
		}
		payload, _ := json.Marshal(out)
		fmt.Println(string(payload))
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

type sqlProcessClient struct {
	command    *exec.Cmd
	input      *json.Encoder
	output     *bufio.Scanner
	closeInput func() error
	stderr     bytes.Buffer
}

func startSQLProcess(t *testing.T, path string) *sqlProcessClient {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	command := exec.CommandContext(ctx, binary, "-test.run=^TestSQLProcessChild$")
	command.Env = append(os.Environ(), "ECC_SQL_PROCESS_DATABASE="+path)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	client := &sqlProcessClient{command: command, input: json.NewEncoder(input), output: bufio.NewScanner(output), closeInput: input.Close}
	command.Stderr = &client.stderr
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("SQL process failed: %v %s", err, client.stderr.String())
		}
		cancel()
	})
	return client
}
func (client *sqlProcessClient) call(in processRequest) (processResponse, error) {
	var out processResponse
	if err := client.input.Encode(in); err != nil {
		for client.output.Scan() {
			fmt.Fprintln(&client.stderr, client.output.Text())
		}
		return out, fmt.Errorf("write failed: %w; %s", err, client.stderr.String())
	}
	var messages strings.Builder
	for client.output.Scan() {
		line := client.output.Text()
		if strings.HasPrefix(line, "{") {
			return out, json.Unmarshal([]byte(line), &out)
		}
		messages.WriteString(line + "\n")
	}
	return out, fmt.Errorf("process stopped: %s %s", messages.String(), client.stderr.String())
}

func TestSeparateProcessesKeepSQLUpdatesAndWorkerExecutions(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	token, _ := issueToken(u)
	member := u
	member.ID = "sql-member"
	member.Email = "sql-member@example.com"
	member.Role = "member"
	mu.Lock()
	users[member.Email] = member
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	memberToken, _ := issueToken(member)
	first, second := startSQLProcess(t, dataPath), startSQLProcess(t, dataPath)
	for _, client := range []*sqlProcessClient{first, second} {
		out, err := client.call(processRequest{Method: "GET", Path: "/api/contacts", Token: token})
		if err != nil || out.Status != 200 {
			t.Fatalf("startup read: %v %+v", err, out)
		}
	}
	var wait sync.WaitGroup
	failures := make(chan error, 2)
	for i, client := range []*sqlProcessClient{first, second} {
		wait.Add(1)
		go func(i int, client *sqlProcessClient) {
			defer wait.Done()
			for j := 0; j < 8; j++ {
				body := json.RawMessage(mustJSON(map[string]string{"name": fmt.Sprintf("Process %d client %d", i, j), "email": fmt.Sprintf("process-%d-%d@example.com", i, j), "stage": "Lead"}))
				out, err := client.call(processRequest{Method: "POST", Path: "/api/contacts", Token: token, Body: body})
				if err != nil || out.Status != 201 {
					failures <- fmt.Errorf("create: %v %+v", err, out)
					return
				}
			}
		}(i, client)
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow("SELECT count(*) FROM contacts WHERE json_extract(data,'$.email') LIKE 'process-%'").Scan(&count); err != nil || count != 16 {
		t.Fatalf("lost process writes: %d %v", count, err)
	}
	// An already-running process must observe account changes committed by another.
	out, err := first.call(processRequest{Method: "GET", Path: "/api/contacts", Token: memberToken})
	if err != nil || out.Status != 200 {
		t.Fatalf("member read: %v %+v", err, out)
	}
	out, err = second.call(processRequest{Method: "PATCH", Path: "/api/admin/users/" + member.ID, Token: token, Body: json.RawMessage(`{"role":"viewer"}`)})
	if err != nil || out.Status != 200 {
		t.Fatalf("role change: %v %+v", err, out)
	}
	out, err = first.call(processRequest{Method: "GET", Path: "/api/contacts", Token: memberToken})
	if err != nil || out.Status != 401 {
		t.Fatalf("stale role/session accepted: %v %+v", err, out)
	}
	var raw string
	database.QueryRow("SELECT data FROM contacts WHERE id='c_1'").Scan(&raw)
	var record map[string]any
	json.Unmarshal([]byte(raw), &record)
	version := versionOf(record)
	results := make(chan processResponse, 2)
	errs := make(chan error, 2)
	for i, client := range []*sqlProcessClient{first, second} {
		wait.Add(1)
		go func(i int, client *sqlProcessClient) {
			defer wait.Done()
			out, err := client.call(processRequest{Method: "PUT", Path: "/api/contacts/c_1", Token: token, Version: version, Body: json.RawMessage(fmt.Sprintf(`{"name":"Edit %d","email":"ada@analytical.com","stage":"Lead"}`, i))})
			if err != nil {
				errs <- err
			} else {
				results <- out
			}
		}(i, client)
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	statuses := map[int]int{}
	for out := range results {
		statuses[out.Status]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatalf("concurrent revision protection: %+v", statuses)
	}
	task := Task{RecordScope: RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}, ID: "sql-recurring-task", Title: "SQL process recurring task", Status: "done", DueDate: "2026-10-09", Recurrence: "daily"}
	if _, err := database.Exec("INSERT INTO tasks VALUES(?,10000,?)", task.ID, mustJSON(task)); err != nil {
		t.Fatal(err)
	}
	for _, client := range []*sqlProcessClient{first, second} {
		wait.Add(1)
		go func(client *sqlProcessClient) {
			defer wait.Done()
			out, err := client.call(processRequest{Method: "WORKER"})
			if err != nil || out.Status != 200 {
				t.Errorf("worker: %v %+v", err, out)
			}
		}(client)
	}
	wait.Wait()
	database.QueryRow("SELECT count(*) FROM workspace_entries WHERE category='recurrence' AND record_id=?", task.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate recurrence markers: %d", count)
	}
	database.QueryRow("SELECT count(*) FROM tasks WHERE json_extract(data,'$.title')=? AND json_extract(data,'$.status')='todo'", task.Title).Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate or missing recurring tasks: %d", count)
	}
	if len(contacts) != 8 {
		t.Fatal("live process writes changed the parent bootstrap state")
	}
}
