# ECC CRM

Elm frontend and Go API for contacts, deals, activities, tasks, schools,
students, agents, leads, cases, document tracking, invoices, payments, and partners.

## Features

- Conversational **Ask ECC AI** adviser throughout the workspace: ask follow-up
  questions, get advice on the current record, and plan your day. Saved conversations, source links and explicit staff confirmation for proposed
  actions. Optional realtime voice endpoints require separate configuration. Requires AI provider configuration and
  workspace activation; see [the AI guide](docs/EXPANSION.md#conversational-ai-adviser).

- Light and dark workspace themes, saved in the browser; grouped navigation,
  accessible contact links, readable tables, and consistent form labels.

- SQLite database with separate tables for all CRM modules, accounts, revoked
  sessions, and change history. Writes commit in a transaction; failed writes
  roll back the in-memory state too.
- Global search in the top bar: search names, emails, companies, phone numbers,
  and reference numbers across the full dataset. Results link to their records.
- Reports: live totals, CSV exports for every CRM module, and paginated change
  history showing the actor, record, action, and time.
- Contacts and activities, deals kanban, tasks, education and case workflows,
  invoices and payments, partner management, and profile/password settings.
- Browser session restore, deep links, and logout across all sessions.

## Run

Requires Elm 0.19.2, Go 1.26 or newer, and a C compiler (such as GCC) for the
[SQLite driver](https://github.com/mattn/go-sqlite3#installation).

```bash
./build.sh
./build/ecc-crm
```

Open http://localhost:7000. The first launch seeds demo records and an account:
`demo@northwind.dev` / `Demo1234`.

The same app supports phones and tablets with bottom navigation, record cards,
larger touch controls and mobile forms. See [docs/MOBILE.md](docs/MOBILE.md) for
phone access and verification details.

The maintained application is `src/`, `public/`, and `golang-backend/`.
Earlier implementations and unused style backups are preserved in `archive/`; see [archive/README.md](archive/README.md). They are excluded from the application build and Docker context.

## Database and migration

The default database is `data/crm.sqlite3`. On a new database, existing
`data/crm.json` records are imported automatically, including password hashes
and session versions. The JSON file is left untouched. Migration happens only
once: restarting uses the database, and empty tables are never reseeded.
Malformed legacy data stops startup rather than replacing it with demo data.

- `DATABASE_PATH`: SQLite database path.
- `LEGACY_DATA_FILE`: optional JSON source to import into a new database.
- `DATA_FILE`: legacy configuration compatibility; when `DATABASE_PATH` is
  unset, imports this JSON file into `crm.sqlite3` in the same directory.
- `PORT`: HTTP port, default `7000`.
- `JWT_SECRET`: set a long random secret for deployment. When unset, a private random secret is persisted in `data/auth-secret`.

Schema migrations use SQLite `user_version`. Record tables store one row per
record, with an ID, ordering, and JSON data; account credentials have dedicated
columns. Runtime handlers and workers load committed SQL into private request
state and persist guarded row changes. Separate-process regression tests cover
concurrent writes, startup, account changes and recurring workers. SQLite still
serializes writers; measure your workload before scaling. Processes sharing a
database must also share the deployment secrets and file storage. Make data
changes through the API.

Back up the database using SQLite's backup command so committed WAL data is included:

```bash
sqlite3 data/crm.sqlite3 ".backup 'data/crm-backup.sqlite3'"
```

Keep backups private: they include CRM records and password hashes. Database
files are excluded from Git and cannot be served by the app's static routes.
Existing accounts and records migrate into the default organization. New registrations
create isolated organizations. Administrators invite users into their own workspace.
Document files are stored privately beside the database and accessed through authenticated
version downloads. Serve behind an HTTPS reverse proxy for deployment.

## New API endpoints

These endpoints require an authenticated HttpOnly cookie session. Existing API
clients with a valid bearer token are also supported.

- `GET /api/search?q=...`: up to 30 results, with a truncation flag. Requires at
  least two characters. Search treats SQL wildcard characters as literal text.
- `GET /api/audit?limit=25&offset=0`: paginated change history; optional `actor` filter.
- `GET /api/exports/{entity}`: all accessible module records as CSV, Excel (`?format=xlsx`), or PDF (`?format=pdf`), with proper quoting and
  spreadsheet formula protection. Account credentials cannot be exported.

Change history starts with changes made after migration. Existing demo data is
not presented as a user action. Deleted record labels remain in the history;
passwords and tokens are never included in history entries.

## Verify

```bash
elm make src/Main.elm --output=/tmp/ecc-crm-check.js
cd golang-backend
go test -race ./...
go vet ./...
```

Workspace styles live in `public/workspace.css`, scoped to the app shell.
The login branding remains in `public/styles.css`. Icons and charts use Elm SVG.


## Real-use workspace tools

Administrators can open **System administration** in the sidebar to manage users,
access levels, teams, invitations, account recovery and workspace security.
See [docs/ADMINISTRATION.md](docs/ADMINISTRATION.md) for the role matrix and steps.

Open **Workspace tools** in the sidebar for user invitations, roles/teams, record
visibility, sessions and 2FA, document versions, SMTP/outbox, imports, saved views,
calendar, workflows, commissions, and integration delivery history. Existing detail
pages link to the relevant tools; invoice pages also offer a PDF download.

The full feature and activation guide is [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md).
[.env.example](.env.example) lists deployment settings. The server reads process
environment variables and does not automatically load `.env` files.

Back up `uploads/` beside the database and the generated `auth-secret` (or preserve
`JWT_SECRET`) along with SQLite. Losing the encryption secret makes saved SMTP and
TOTP secrets unreadable. Restore these together. External SMTP, Stripe, VAPID,
OIDC and SAML integrations require configuration before live use.

## Daily workflow expansion

Record pages now embed native tools, with a separate client portal, cohort reports,
inbound conversations, optional AI drafts, version conflicts, and recoverable deletion.
Docker/Compose, CI, metrics and backup/restore scripts are included.
See [docs/EXPANSION.md](docs/EXPANSION.md) for setup, verification and remaining
provider/scaling limits. The older implementation checklist describes the previous
feature set; it is not a production acceptance record for this expansion.

## Daily work

See [docs/DAILY_WORK.md](docs/DAILY_WORK.md) for My work today, the unified student journey, navigation presets, inline form warnings, integration connection checks, SQL repository changes and verification.

## Next priorities

See [docs/ROADMAP.md](docs/ROADMAP.md) for the prioritized SQL migration,
live integration acceptance tests, application handoffs, searchable record
pickers, and adviser/finance usability sessions, including completion criteria
and dependencies. These are planned work, not completed features.

See [real integration acceptance](docs/INTEGRATION_ACCEPTANCE.md) for the runnable
provider checks and [staff usability](docs/STAFF_USABILITY.md) for participant
tasks and observation recording. Live provider acceptance and actual staff
sessions remain pending.

See [operational improvements](docs/OPERATIONAL_IMPROVEMENTS.md) for faster
contact saves, application requirements, handoff reminders, duplicate receipt
checks and optional manager approval.
