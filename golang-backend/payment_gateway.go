package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func registerPaymentRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/invoices/{id}/checkout", authMiddleware(handleCheckout))
	m.HandleFunc("POST /api/payments/stripe/webhook", handleStripeWebhook)
	m.HandleFunc("POST /api/invoices/{id}/gateway-refund", authMiddleware(handleGatewayRefund))
}

var stripeHTTPClient = &http.Client{Timeout: 15 * time.Second}

func stripeRequest(path string, values url.Values, idempotency string) (map[string]any, error) {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		return nil, fmt.Errorf("Stripe is not configured")
	}
	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/"+path, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(key, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", idempotency)
	res, err := stripeHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Payment provider unavailable")
	}
	defer res.Body.Close()
	var data map[string]any
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) != nil {
		return nil, fmt.Errorf("Invalid payment response")
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("Payment provider rejected the request")
	}
	return data, nil
}
func currencyMinor(currency string) int64 {
	if currency == "JPY" || currency == "KRW" {
		return 1
	}
	return 100
}
func (state *runtimeStore) handleCheckout(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	var inv Invoice
	ok := false
	for _, i := range state.invoices {
		if i.ID == r.PathValue("id") {
			inv = i
			ok = true
		}
	}
	state.mu.RUnlock()
	if !ok {
		writeError(w, 404, "Invoice not found")
		return
	}
	if inv.Balance <= 0 {
		writeError(w, 400, "Invoice has no outstanding balance")
		return
	}
	currency := currencyOf(inv.Currency)
	amount := int64(inv.Balance*float64(currencyMinor(currency)) + .5)
	values := url.Values{"mode": {"payment"}, "success_url": {appURL() + "/invoices/" + inv.ID + "?payment=complete"}, "cancel_url": {appURL() + "/invoices/" + inv.ID}, "client_reference_id": {inv.ID}, "metadata[invoice_id]": {inv.ID}, "metadata[org_id]": {inv.OrgID}, "line_items[0][price_data][currency]": {strings.ToLower(currency)}, "line_items[0][price_data][unit_amount]": {strconv.FormatInt(amount, 10)}, "line_items[0][price_data][product_data][name]": {"Invoice " + inv.InvoiceNumber}, "line_items[0][quantity]": {"1"}}
	data, err := stripeRequest("checkout/sessions", values, "checkout:"+inv.ID+":"+fmt.Sprint(amount)+":"+strconv.FormatInt(time.Now().Unix()/300, 10))
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"url": data["url"], "id": data["id"]})
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleCheckout(w, r)
}
func stripeSignatureValid(header string, body []byte) bool {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		return false
	}
	stamp := ""
	signatures := []string{}
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			if key == "t" {
				stamp = value
			}
			if key == "v1" {
				signatures = append(signatures, value)
			}
		}
	}
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)) > 5*time.Minute || time.Until(time.Unix(seconds, 0)) > time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stamp + "."))
	mac.Write(body)
	sum := mac.Sum(nil)
	for _, sig := range signatures {
		decoded, e := hex.DecodeString(sig)
		if e == nil && hmac.Equal(decoded, sum) {
			return true
		}
	}
	return false
}
func (state *runtimeStore) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || !stripeSignatureValid(r.Header.Get("Stripe-Signature"), body) {
		writeError(w, 400, "Invalid payment signature")
		return
	}
	var event struct {
		ID, Type string
		Data     struct {
			Object struct {
				ID            string
				AmountTotal   int64 `json:"amount_total"`
				Currency      string
				PaymentStatus string `json:"payment_status"`
				PaymentIntent string `json:"payment_intent"`
				Metadata      map[string]string
			}
		}
	}
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		writeError(w, 400, "Invalid event")
		return
	}
	if event.Type != "checkout.session.completed" && event.Type != "checkout.session.async_payment_succeeded" {
		writeJSON(w, 200, statusResponse{"ignored"})
		return
	}
	o := event.Data.Object
	if o.PaymentIntent == "" && o.PaymentStatus == "paid" {
		writeError(w, 400, "Payment reference is missing")
		return
	}
	if o.PaymentStatus != "paid" {
		writeJSON(w, 200, statusResponse{"pending"})
		return
	}
	var received int
	storeDB(r).QueryRow("SELECT count(*) FROM payments WHERE json_extract(data,'$.reference')=? AND json_extract(data,'$.method')='Stripe'", o.PaymentIntent).Scan(&received)
	if received > 0 {
		writeJSON(w, 200, statusResponse{"already_processed"})
		return
	}
	var count int
	storeDB(r).QueryRow("SELECT count(*) FROM workspace_entries WHERE id=?", "stripe_event_"+event.ID).Scan(&count)
	if count > 0 {
		writeJSON(w, 200, statusResponse{"already_processed"})
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	index := -1
	for i, inv := range state.invoices {
		if inv.ID == o.Metadata["invoice_id"] && inv.OrgID == o.Metadata["org_id"] {
			index = i
		}
	}
	if index < 0 || !strings.EqualFold(currencyOf(state.invoices[index].Currency), o.Currency) || o.AmountTotal <= 0 {
		writeError(w, 400, "Invoice or currency mismatch")
		return
	}
	inv := state.invoices[index]
	state.recomputeInvoiceLocked(index)
	amount := float64(o.AmountTotal) / float64(currencyMinor(currencyOf(inv.Currency)))
	state.payments = append(state.payments, Payment{RecordScope: inv.RecordScope, ID: newID("pay"), InvoiceID: inv.ID, Amount: amount, Status: "posted", PaidOn: time.Now().Format("2006-01-02"), Method: "Stripe", Reference: o.PaymentIntent, CreatedBy: "Stripe webhook", CreatedAt: utcNow()})
	state.recomputeInvoiceLocked(index)
	_, err = storeDB(r).Exec("INSERT INTO workspace_entries VALUES(?,?, 'gateway_event','',?,?,?,?)", "stripe_event_"+event.ID, inv.OrgID, inv.ID, mustJSON(map[string]string{"paymentIntent": o.PaymentIntent, "checkoutId": o.ID}), utcNow(), utcNow())
	if err != nil {
		writeError(w, 500, "Could not record payment")
		return
	}
	metadata(r).Actor = "Stripe"
	metadata(r).OrgID = inv.OrgID
	writeJSON(w, 200, statusResponse{"processed"})
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleStripeWebhook(w, r)
}
func (state *runtimeStore) handleGatewayRefund(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager approval is required")
		return
	}
	var req struct{ RefundID string }
	if !decodeRequest(w, r, &req) {
		return
	}
	var payload, invoiceID string
	if storeDB(r).QueryRow("SELECT data,record_id FROM workspace_entries WHERE id=? AND org_id=? AND category='refund'", req.RefundID, currentUser(r).OrgID).Scan(&payload, &invoiceID) != nil || invoiceID != r.PathValue("id") {
		writeError(w, 404, "Refund not found")
		return
	}
	var refund map[string]any
	json.Unmarshal([]byte(payload), &refund)
	if refund["status"] != "approved" && refund["status"] != "provider_pending" {
		writeError(w, 409, "Approve the refund first")
		return
	}
	state.mu.RLock()
	var inv Invoice
	intent := ""
	for _, i := range state.invoices {
		if i.ID == invoiceID {
			inv = i
		}
	}
	for _, p := range state.payments {
		if p.InvoiceID == invoiceID && p.Method == "Stripe" {
			intent = p.Reference
		}
	}
	state.mu.RUnlock()
	amount, _ := refund["amount"].(float64)
	if inv.ID == "" || intent == "" || amount <= 0 || amount > inv.AmountReceived {
		writeError(w, 400, "No eligible Stripe payment for this refund")
		return
	}

	var data map[string]any
	var err error
	if refund["status"] == "provider_pending" {
		reference, _ := refund["reference"].(string)
		data, err = stripeGet("refunds/" + url.PathEscape(reference))
	} else {
		data, err = stripeRequest("refunds", url.Values{"payment_intent": {intent}, "amount": {strconv.FormatInt(int64(amount*float64(currencyMinor(currencyOf(inv.Currency)))+.5), 10)}}, "refund:"+req.RefundID)
	}
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	if data["status"] != "succeeded" {
		refundState := "provider_pending"
		if data["status"] == "failed" || data["status"] == "canceled" {
			refundState = "failed"
		}
		refund["status"] = refundState
		refund["reference"] = data["id"]
		if _, err = storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=?", mustJSON(refund), utcNow(), req.RefundID); err != nil {
			writeError(w, 500, "Could not record provider status")
			return
		}
		state.mu.Lock()
		for i := range state.invoices {
			if state.invoices[i].ID == inv.ID {
				state.invoices[i].RefundStatus = refundState
			}
		}
		state.mu.Unlock()
		writeJSON(w, 200, refund)
		return
	}

	state.mu.Lock()
	state.payments = append(state.payments, Payment{RecordScope: inv.RecordScope, ID: newID("pay"), InvoiceID: inv.ID, Amount: -amount, Status: "refund", Method: "Stripe refund", Reference: fmt.Sprint(data["id"]), PaidOn: organizationToday(r), CreatedBy: currentUser(r).Name, CreatedAt: utcNow()})
	for i := range state.invoices {
		if state.invoices[i].ID == inv.ID {
			state.recomputeInvoiceLocked(i)
			state.invoices[i].RefundStatus = "processed"
		}
	}
	state.mu.Unlock()
	refund["status"] = "processed"
	refund["reference"] = data["id"]
	storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=?", mustJSON(refund), utcNow(), req.RefundID)
	writeJSON(w, 200, refund)
}

func handleGatewayRefund(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleGatewayRefund(w, r)
}

func stripeGet(path string) (map[string]any, error) {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		return nil, fmt.Errorf("Stripe is not configured")
	}
	r, err := http.NewRequest("GET", "https://api.stripe.com/v1/"+path, nil)
	if err != nil {
		return nil, err
	}
	r.SetBasicAuth(key, "")
	res, err := stripeHTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Payment provider unavailable")
	}
	defer res.Body.Close()
	var data map[string]any
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) != nil || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Payment provider rejected the request")
	}
	return data, nil
}
