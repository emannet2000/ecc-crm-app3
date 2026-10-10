# Real-use feature delivery

This is the delivery checklist for the requested CRM expansion. Status is based on
working code and verification, not merely on a route or model being present.

## Existing and to be retained

- [x] SQLite persistence and legacy JSON migration
- [x] Global cross-module search
- [x] CSV exports for CRM modules
- [x] Live reports
- [x] Record change history
- [x] Light/dark themes and responsive workspace

## Identity and access

- [x] Organizations and teams; admin user creation and invitations
- [x] Admin, manager, member, and viewer roles
- [x] Organization/team/private record visibility; edit permissions
- [x] HttpOnly sessions, refresh rotation, session list/revoke-one
- [x] Password recovery; login audit; throttling and lockout
- [x] TOTP with recovery codes; password strength feedback
- [x] IP restrictions and configurable SSO

## Documents, communication, and integrations

- [x] Protected uploads/downloads and document version history
- [x] SMTP outbox, invoice delivery, client email history
- [x] In-app/email/push notifications and reminders
- [x] Invoice PDFs, Excel export, validated CSV imports
- [x] Payment checkout, signed payment callbacks, refunds/credit notes
- [x] Signed webhooks and delivery history

## Workflows and data quality

- [x] Correct currency handling, rates, deal probabilities and line items
- [x] Deal stage history, owner filters, bulk moves
- [x] Ledger-based invoice balances, tax/discount, partial refunds and dunning
- [x] Case templates, verified-document advancement, deadlines/SLA, linked cases
- [x] Student application/pre-departure details; direct case creation
- [x] Computed referral/enrolment counts, commissions, payout history
- [x] School programs/fees; partner commissions
- [x] Lead conversion/scoring/source costs; contact dedup/merge/bulk actions
- [x] Task pagination, recurrence, reminders, calendar
- [x] Sorting, saved filters/views, column preferences
- [x] Timezones and relative dates; country/phone/URL validation
- [x] Retryable errors, pending-action feedback, keyboard/focus accessibility

## External activation

SMTP, payment gateways, push delivery, and identity providers need configuration.
The implementation will expose clear setup states and will not report messages or
payments as delivered while their provider is unconfigured.


## How to use the new features

- **Team & access:** create/invite users, set admin/manager/member/viewer roles,
  create teams, and choose organization/team/private record visibility. Managers
  can manage all records in their organization; members edit records they own;
  viewers read accessible records. At least one active administrator is retained.
- **Security:** list/revoke sessions, enable TOTP, save one-use recovery codes,
  inspect login/security events, and configure timezone, reporting currency and IP
  restrictions. Browser session secrets are HttpOnly and stored only as hashes
  server-side. Access expires after 15 minutes; refresh credentials rotate. Remembered
  sessions last up to 30 days; other sessions last up to 24 hours. Five failed logins
  lock the account for 15 minutes; sensitive endpoints also have IP rate limits.
- **Files:** upload up to 10 MB per version. Allowed formats are PDF, PNG, JPEG,
  DOCX, XLSX, TXT and CSV. The server checks type signatures, gives storage files
  random names, records checksums, and requires access to both document and case.
  Verification requires an uploaded version and rejects expired documents.
- **Email:** compose client emails, queue invoice PDFs, inspect delivery states,
  and retry failed delivery. Queueing is distinct from successful SMTP delivery.
  Client emails appear in activities and update last-contact dates.
- **Workflows:** deal probabilities and products, lead conversion/scoring/follow-up
  tasks, task recurrence, case templates/SLA/family links, student tests/application/
  travel/accommodation/pre-departure checklists, program fees, and finance terms.
  Student pages link directly to student case creation. All-required-document
  verification advances opted-in collection/review cases to submission readiness.
- **Finance:** receipts are a ledger; deleting a manual receipt recomputes balance.
  Tax, discounts and credit notes affect invoice totals. Partial refunds move through
  requested, approved/rejected, processed and reversed states. Manual processing
  requires an actual bank/payment reference; Stripe processing uses its refund API.
  Gateway and refund ledger payments cannot be casually deleted.
- **Commissions:** calculate a commission from an eligible amount and percentage,
  and record payouts with currency and payment references. History is retained.
  Referral/enrolment counts derive from linked students and cases. Assign a referral
  partner on the case workflow to populate partner counts.
- **Data & views:** CSV imports preview row errors and reject duplicate emails.
  Commit is atomic and supports up to 5,000 rows. Export accessible data to CSV,
  Excel or PDF. Browse every module with pagination, clickable sort headers,
  search/status filters and column selection; save named views. Contact merges
  reassign relationships and are rejected if related records cannot be updated.
- **Notifications/calendar:** persistent in-app reminders, opt-in notification
  email, optional web push, case deadlines, expiring documents, overdue invoices,
  and recurring tasks. The worker runs every 15 seconds. Workspace dates follow
  the organization's IANA timezone; timestamp displays include relative dates.

Legacy financial currency values that were never saved by the previous backend
cannot be recovered; existing unspecified currency is interpreted as USD. New
currency values persist. Mixed totals are never added at face value: reports use
explicit dated exchange rates or show separate currency subtotals. The CSV/Excel/
PDF exports preserve the original currency fields.

## Activate external integrations

The environment variable names are in [.env.example](../.env.example). Configure
`APP_URL` to the actual public HTTPS origin and set `ALLOW_SIGNUP=false` when only
administrator provisioning should be available. Trust forwarded IP headers only
from explicitly configured `TRUSTED_PROXY_CIDRS`.

**SMTP:** configure host, port, sender, credentials and STARTTLS/implicit TLS in
Workspace tools → Integrations, or set the `SMTP_*` environment variables. A
workspace-specific setting overrides the environment fallback. Saved credentials
are encrypted. `local` TLS mode permits unencrypted delivery only to a loopback
SMTP server for testing. Delivery failures retry up to five times with increasing
delays; messages stay visibly queued/waiting/failed until delivery succeeds.
Password reset links expire after 30 minutes; invitations after 72 hours. Both are
single-use and stored as hashes. Administrator invitation links can be copied
when email has not yet been connected.

**Stripe:** set `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`. Register the
callback URL `APP_URL/api/payments/stripe/webhook` for
`checkout.session.completed` and `checkout.session.async_payment_succeeded`.
Only signed paid events with matching invoice/organization/currency become ledger
receipts. Event IDs and payment references prevent duplicate posting. Create a
checkout link from an invoice row in Data & views. A browser success redirect is
not proof of payment. Pending provider refunds stay pending without ledger posting; use **Check Stripe
refund** to fetch the provider status and post a successful refund once. Manual
processing is blocked while a provider refund is pending. Run a Stripe test-mode
payment and partial refund before
switching to live credentials.

**OpenID Connect:** set issuer URL, client ID and client secret; register
`APP_URL/api/sso/callback` as the redirect URI. Sign-in uses state, nonce, PKCE and
signed ID-token verification. A verified email must match an existing active CRM
account. Local 2FA, when enabled, still needs completion. New accounts and roles
are not created from identity-provider claims.

**SAML:** provide the IdP metadata URL and SP certificate/private key paths.
The SP metadata endpoint is `APP_URL/api/saml/metadata`, assertion consumer is
`APP_URL/api/saml/acs`, and start endpoint is `APP_URL/api/saml/start`. The signed
assertion must provide the configured email attribute (or email NameID) for an
existing active account. IdP-initiated login is disabled; assertions are tracked
and protected against replay. HTTPS is needed for the SameSite=None request
tracking cookies. Provider-specific SAML acceptance testing remains necessary;
no live IdP was supplied for this workspace.

**Push:** generate keys with `cd golang-backend && go run ./cmd/vapid`, set the
VAPID public/private key and subscriber address, and restart. Users explicitly
opt into browser notifications. Push subscriptions accept supported browser
service hosts; expired subscriptions are removed. Use HTTPS or localhost.

**Webhooks:** administrators connect public HTTPS endpoints and save the secret
shown once. Deliveries expose `X-ECC-Delivery`, `X-ECC-Timestamp`, and
`X-ECC-Signature`. Verify HMAC-SHA256 over `timestamp + "." + raw request body`,
reject stale timestamps, and deduplicate delivery IDs. Private-network destinations
and redirects are blocked. Delivery states, retries, and enable/disable controls
are available in Integrations.

## Verification and operational limits

Go tests exercise persistence/migration, organization and private-record isolation,
roles, cookie refresh/revocation, reset/invitation single-use behavior, TOTP replay
and recovery codes, throttling/CSRF, protected file versions, import atomicity,
Excel/PDF output, payment/refund/credit reconciliation, signed gateway idempotency,
case templates/verification/SLA, recurring tasks, and OIDC signature/nonce/verified
email/PKCE/replay checks using fixtures. Browser checks cover login/restoration,
keyboard navigation, modal focus, search, exports/history, themes, all workspace
tabs, saved views, and mobile layouts.

Provider credentials were not supplied. SMTP, Stripe, push and SAML should be
validated against their real providers before production activation. The app
now verifies shared-database operation with separate-process regression tests;
SQLite writers remain serialized and large writes remain expensive. See
[SQL runtime verification](DAILY_WORK.md#sql-transition-and-limits). Back up the database
using SQLite's backup API, plus the private uploads directory and encryption secret.
Protect the demo administrator account and disable public signup for a private
production workspace.
