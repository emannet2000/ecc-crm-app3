package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const maxRecordPhoto = 5 << 20

func registerRecordPhotoRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/records/{entity}/{id}/photo", authMiddleware(handleRecordPhoto))
	m.HandleFunc("PUT /api/records/{entity}/{id}/photo", authMiddleware(handleRecordPhotoUpload))
	m.HandleFunc("DELETE /api/records/{entity}/{id}/photo", authMiddleware(handleRecordPhotoDelete))
	m.HandleFunc("GET /api/records/{entity}/{id}/qr", authMiddleware(handleRecordQR))
	m.HandleFunc("POST /api/qr/decode", authMiddleware(handleQRDecode))
}

func handleQRDecode(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRecordPhoto+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Choose a QR image smaller than 5 MB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Choose a PNG or JPEG image")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRecordPhoto+1))
	if err != nil || len(data) == 0 || len(data) > maxRecordPhoto {
		writeError(w, http.StatusBadRequest, "Image must be between 1 byte and 5 MB")
		return
	}
	kind, valid := validatedFile(header.Filename, data)
	config, _, decodeErr := image.DecodeConfig(bytes.NewReader(data))
	if !valid || (kind != "image/png" && kind != "image/jpeg") || decodeErr != nil || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		writeError(w, http.StatusBadRequest, "Use a valid PNG or JPEG image up to 6000 × 6000 pixels")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "zbarimg", "--quiet", "--raw", "-")
	command.Stdin = bytes.NewReader(data)
	command.Stderr = io.Discard
	decoded, err := command.Output()
	if errors.Is(err, exec.ErrNotFound) {
		writeError(w, http.StatusServiceUnavailable, "QR image decoding is unavailable")
		return
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeError(w, http.StatusServiceUnavailable, "QR image decoding timed out")
		return
	}
	if err != nil || len(decoded) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "No QR code could be read from that image")
		return
	}
	value := strings.TrimSpace(strings.SplitN(string(decoded), "\n", 2)[0])
	writeJSON(w, http.StatusOK, map[string]string{"value": value})
}

func (state *runtimeStore) recordPhotoScope(r *http.Request, write bool) (RecordScope, bool) {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	if (entity != "contacts" && entity != "students") || id == "" {
		return RecordScope{}, false
	}
	payload, ok := recordsOf(state.snapshot())[entity][id]
	if !ok {
		return RecordScope{}, false
	}
	var record struct {
		RecordScope
	}
	if json.Unmarshal([]byte(payload), &record) != nil {
		return RecordScope{}, false
	}
	user := currentUser(r)
	if (write && !canWrite(record.RecordScope, user)) || (!write && !canRead(record.RecordScope, user)) {
		return RecordScope{}, false
	}
	return record.RecordScope, true
}

func recordPhotoScope(r *http.Request, write bool) (RecordScope, bool) {
	state, release := runtimeForReadCall(r)
	defer release()
	return state.recordPhotoScope(r, write)
}

func auditRecordPhoto(r *http.Request, scope RecordScope, action string) error {
	label := "Profile photo updated"
	if action == "photo_removed" {
		label = "Profile photo removed"
	}
	_, err := storeDB(r).Exec(`INSERT INTO audit_events(actor,action,entity,record_id,label,occurred_at,org_id,scope_data) VALUES(?,?,?,?,?,?,?,?)`, currentUser(r).Email, action, r.PathValue("entity"), r.PathValue("id"), label, utcNow(), scope.OrgID, mustJSON(scope))
	return err
}

func (state *runtimeStore) handleRecordQR(w http.ResponseWriter, r *http.Request) {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	if !validEntity(entity) || id == "" {
		writeError(w, http.StatusNotFound, "Record not found")
		return
	}
	payload, ok := recordsOf(state.snapshot())[entity][id]
	if !ok {
		writeError(w, http.StatusNotFound, "Record not found")
		return
	}
	var record struct{ RecordScope }
	if json.Unmarshal([]byte(payload), &record) != nil || !canRead(record.RecordScope, currentUser(r)) {
		writeError(w, http.StatusNotFound, "Record not found")
		return
	}
	target := strings.TrimRight(appURL(), "/") + "/" + entity + "/" + url.PathEscape(id)
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "qrencode", "-o", "-", "-t", "PNG", "-s", "8", "-m", "2")
	command.Stdin = strings.NewReader(target)
	png, err := command.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeError(w, http.StatusServiceUnavailable, "QR code generation timed out")
		} else {
			writeError(w, http.StatusServiceUnavailable, "QR code generator is unavailable")
		}
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.Header().Set("Content-Disposition", "inline; filename=record-qr.png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(png)
}

func handleRecordQR(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRecordQR(w, r)
}

func (state *runtimeStore) handleRecordPhoto(w http.ResponseWriter, r *http.Request) {
	scope, ok := state.recordPhotoScope(r, false)
	if !ok {
		writeError(w, http.StatusNotFound, "Record photo not found")
		return
	}
	var mime string
	var data []byte
	err := storeDB(r).QueryRow("SELECT mime,data FROM record_photos WHERE org_id=? AND entity=? AND record_id=?", scope.OrgID, r.PathValue("entity"), r.PathValue("id")).Scan(&mime, &data)
	if err != nil {
		writeError(w, http.StatusNotFound, "Record photo not found")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func handleRecordPhoto(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRecordPhoto(w, r)
}

func (state *runtimeStore) handleRecordPhotoUpload(w http.ResponseWriter, r *http.Request) {
	scope, ok := state.recordPhotoScope(r, true)
	if !ok {
		writeError(w, http.StatusNotFound, "Record not found or you cannot edit it")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRecordPhoto+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Choose an image smaller than 5 MB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Choose a PNG or JPEG image")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRecordPhoto+1))
	if err != nil || len(data) == 0 || len(data) > maxRecordPhoto {
		writeError(w, http.StatusBadRequest, "Image must be between 1 byte and 5 MB")
		return
	}
	kind, valid := validatedFile(header.Filename, data)
	if !valid || (kind != "image/png" && kind != "image/jpeg") {
		writeError(w, http.StatusBadRequest, "Use a PNG or JPEG image")
		return
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		writeError(w, http.StatusBadRequest, "Image dimensions must be at most 6000 × 6000 pixels")
		return
	}
	_, err = storeDB(r).Exec(`INSERT INTO record_photos(org_id,entity,record_id,mime,data,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(org_id,entity,record_id) DO UPDATE SET mime=excluded.mime,data=excluded.data,updated_at=excluded.updated_at`, scope.OrgID, r.PathValue("entity"), r.PathValue("id"), kind, data, utcNow())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save record photo")
		return
	}
	if err = auditRecordPhoto(r, scope, "photo_updated"); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not record photo change")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"message": "Photo updated", "updatedAt": utcNow()})
}

func handleRecordPhotoUpload(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRecordPhotoUpload(w, r)
}

func (state *runtimeStore) handleRecordPhotoDelete(w http.ResponseWriter, r *http.Request) {
	scope, ok := state.recordPhotoScope(r, true)
	if !ok {
		writeError(w, http.StatusNotFound, "Record not found or you cannot edit it")
		return
	}
	result, err := storeDB(r).Exec("DELETE FROM record_photos WHERE org_id=? AND entity=? AND record_id=?", scope.OrgID, r.PathValue("entity"), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not remove record photo")
		return
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		if err = auditRecordPhoto(r, scope, "photo_removed"); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not record photo change")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleRecordPhotoDelete(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleRecordPhotoDelete(w, r)
}
