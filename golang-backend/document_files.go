package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func registerDocumentRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/documents/{id}/files", authMiddleware(handleFileVersions))
	m.HandleFunc("POST /api/documents/{id}/files", authMiddleware(handleFileUpload))
	m.HandleFunc("GET /api/documents/{id}/files/{version}", authMiddleware(handleFileDownload))
}
func (state *runtimeStore) visibleDocument(id string) (Document, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, d := range state.documents {
		if d.ID == id {
			return d, true
		}
	}
	return Document{}, false
}

func visibleDocument(id string) (Document, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.visibleDocument(id)
}
func uploadRoot() string { return filepath.Join(filepath.Dir(dataPath), "uploads") }
func (state *runtimeStore) handleFileVersions(w http.ResponseWriter, r *http.Request) {
	if _, ok := state.visibleDocument(r.PathValue("id")); !ok {
		writeError(w, 404, "Document not found")
		return
	}
	rows, err := queryObjects(r, "SELECT version,filename,mime,size,sha256,uploaded_by AS uploadedBy,uploaded_at AS uploadedAt FROM file_versions WHERE org_id=? AND document_id=? ORDER BY version DESC", currentUser(r).OrgID, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "Could not load versions")
		return
	}
	writeJSON(w, 200, map[string]any{"versions": rows})
}

func handleFileVersions(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleFileVersions(w, r)
}
func validatedFile(name string, data []byte) (string, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	detected := http.DetectContentType(data)
	switch ext {
	case ".pdf":
		return "application/pdf", bytes.HasPrefix(data, []byte("%PDF-"))
	case ".png":
		return "image/png", detected == "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg", detected == "image/jpeg"
	case ".txt", ".csv":
		return "text/plain; charset=utf-8", strings.HasPrefix(detected, "text/plain") && !bytes.Contains(data, []byte{0})
	case ".docx", ".xlsx":
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", false
		}
		total := uint64(0)
		found := false
		target := "word/document.xml"
		kind := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		if ext == ".xlsx" {
			target = "xl/workbook.xml"
			kind = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		for _, f := range z.File {
			total += f.UncompressedSize64
			if total > 100<<20 || strings.Contains(f.Name, "..") || strings.HasSuffix(strings.ToLower(f.Name), "vbaproject.bin") {
				return "", false
			}
			if f.Name == target {
				found = true
			}
		}
		return kind, found
	}
	return "", false
}
func (state *runtimeStore) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	doc, ok := state.visibleDocument(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Document not found")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, 400, "Choose a file smaller than 10 MB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, h, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "Choose a file")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (10<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 {
		writeError(w, 400, "File must be between 1 byte and 10 MB")
		return
	}
	name := filepath.Base(strings.ReplaceAll(h.Filename, "\\", "/"))
	if len(name) > 200 || strings.ContainsAny(name, "\r\n\x00") {
		writeError(w, 400, "Invalid filename")
		return
	}
	kind, valid := validatedFile(name, data)
	if !valid {
		writeError(w, 400, "Use a PDF, PNG, JPEG, DOCX, XLSX, TXT, or CSV matching its file type")
		return
	}
	dir := filepath.Join(uploadRoot(), doc.OrgID, doc.ID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		writeError(w, 500, "Could not store file")
		return
	}
	path := filepath.Join(dir, randomSecret())
	if err = os.WriteFile(path, data, 0600); err != nil {
		writeError(w, 500, "Could not store file")
		return
	}
	if meta := metadata(r); meta != nil {
		meta.Rollback = append(meta.Rollback, func() { os.Remove(path) })
	}
	version := 0
	err = storeDB(r).QueryRow("SELECT coalesce(max(version),0)+1 FROM file_versions WHERE document_id=?", doc.ID).Scan(&version)
	if err != nil {
		writeError(w, 500, "Could not save version")
		return
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	_, err = storeDB(r).Exec("INSERT INTO file_versions VALUES(?,?,?,?,?,?,?,?,?,?,?)", newID("file"), doc.OrgID, doc.ID, version, name, path, kind, len(data), sum, currentUser(r).ID, utcNow())
	if err != nil {
		writeError(w, 500, "Could not save version")
		return
	}
	state.mu.Lock()
	for i := range state.documents {
		if state.documents[i].ID == doc.ID {
			state.documents[i].LatestVersion = version
			state.documents[i].FilePath = fmt.Sprintf("/api/documents/%s/files/%d", doc.ID, version)
			state.documents[i].Status = "Received"
			state.documents[i].DateReceived = organizationToday(r)
			state.documents[i].VerifiedBy = ""
			state.documents[i].VerificationDate = ""
			doc = state.documents[i]
		}
	}
	state.mu.Unlock()
	writeJSON(w, 201, map[string]any{"document": doc, "version": version})
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleFileUpload(w, r)
}
func (state *runtimeStore) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	doc, ok := state.visibleDocument(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Document not found")
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		writeError(w, 404, "Version not found")
		return
	}
	var path, name, kind string
	err = storeDB(r).QueryRow("SELECT storage_path,filename,mime FROM file_versions WHERE document_id=? AND org_id=? AND version=?", doc.ID, doc.OrgID, version).Scan(&path, &name, &kind)
	if err != nil {
		writeError(w, 404, "Version not found")
		return
	}
	rel, err := filepath.Rel(uploadRoot(), path)
	if err != nil || strings.HasPrefix(rel, "..") {
		writeError(w, 404, "File not found")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, 404, "Stored file is unavailable")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	io.Copy(w, f)
}

func handleFileDownload(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleFileDownload(w, r)
}
