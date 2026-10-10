# Real integration acceptance

Connection probes verify setup, not delivery. Run acceptance in a designated test
workspace with dedicated contacts and recipient accounts. Current live-provider
acceptance is **pending**; no provider credentials or dedicated recipients have
been supplied for this work.

The runner uses the app's real HTTP endpoints and providers. Its default is a
read-only configuration assessment after login:

```bash
export ECC_ACCEPTANCE_URL=https://your-test-app.example
export ECC_ACCEPTANCE_EMAIL=acceptance-admin@example.com
# Set ECC_ACCEPTANCE_PASSWORD securely in your shell or CI secret store.
npm run test:integrations
```

To send test messages and make provider calls, set `ECC_ACCEPTANCE_SEND=true`.
Set `ECC_ACCEPTANCE_CONTACT_ID` to a dedicated contact and
`ECC_ACCEPTANCE_RECIPIENT` to that contact's test email address. Set
`ECC_ACCEPTANCE_WHATSAPP=true` only for a contact with a dedicated WhatsApp test
number; send an inbound message from that number first to open the 24-hour reply
window. The runner sends a unique marker and waits for a matching inbound reply.

For payments, set `ECC_ACCEPTANCE_INVOICE_ID` to an unpaid test invoice. The
runner refuses checkout unless the server reports a Stripe test key. Complete
the printed Stripe test checkout while it polls the invoice. It never fills a
payment form or authorizes a payment automatically.

`ECC_ACCEPTANCE_SECONDS` controls each polling deadline (default 180, maximum
3600). Set `ECC_ACCEPTANCE_MARKER` to reproduce a known marker, and
`ECC_ACCEPTANCE_REPORT` to save a private JSON report. Report statuses distinguish
`passed`, `provider_accepted`, `callback_observed`, `response_observed`, `pending`,
`not_configured`, and `failed`. Partial evidence is not full acceptance. A zero
exit code means the runner encountered no failed checks; pending checks still
require work. Credentials and CRM response bodies are not included in reports.

Complete the following companion checks before approving live integrations:

| Integration | Evidence required beyond the runner |
| --- | --- |
| Email | Check the actual inbox and spam folder, reply with the marker, verify association with the intended contact/application, then exercise bad SMTP settings and a successful retry. SMTP `sent` means provider acceptance. |
| WhatsApp | Verify delivered/read callbacks, a matching inbound reply and correct contact association. Replay the same real signed webhook and verify one message/activity. Exercise rejected credentials and the closed reply window. |
| Payments | Complete real test checkout and verify one ledger payment and the expected balance. Replay the signed provider event; verify no second payment. Cancel another checkout and exercise provider failure; neither should settle an invoice. |
| AI | Inspect the actual answer and accessible source links, then reopen the saved thread. Test a proposed change: before confirmation the record must stay unchanged. Confirm as its authorized owner; reject confirmation as another staff member or viewer, and reject a stale proposal. Exercise provider unavailability and limits. |

Record environment, date, scenario, expected result, actual result and sanitized
provider references. Keep real customer data and credentials out of evidence.
Backend tests cover signed callback replay, error paths, workspace isolation and
AI permission boundaries with controlled fixtures. Those tests do not establish
live provider acceptance.

Multiple workers claim queued deliveries atomically. SMTP interruption produces
`delivery_unknown`; verify provider evidence before retrying. WhatsApp transport
failure produces `uncertain`; a crash can leave `sending`. Push delivery reserves
a unique claim before dispatch and leaves uncertainty for manual review on
failure. External delivery cannot be made exactly once by a database transaction.
Webhook retries retain the delivery ID so receivers can deduplicate.
