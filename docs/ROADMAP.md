# Next priorities

Implementation status as of 2026-10-10: SQL runtime isolation, application
handoffs, searchable application/invoice pickers and staff observation recording
are implemented. Separate-process and browser verification cover the software.
Live providers and actual staff participation are still pending. See
[Daily work](DAILY_WORK.md), [integration acceptance](INTEGRATION_ACCEPTANCE.md)
and [staff sessions](STAFF_USABILITY.md).

| Priority | Improvement | Why it matters | Status |
| --- | --- | --- | --- |
| 1 | Finish the SQL migration | Committed SQL state prevents stale process data; large-workspace validation remains expensive. | SQL runtime migration and separate-process tests complete; costly projections remain. |
| 2 | Test real integrations end to end | A successful connection check does not prove email delivery, WhatsApp replies, payment callbacks, or AI responses work. | Runner and checklist ready; live providers pending. |
| 3 | Make application handoffs explicit | Show the current owner, next action, due date, and who takes over next. | Implemented; backend and browser checks pass. |
| 4 | Add searchable record pickers | Staff should find any accessible student, contact, or application without relying on loaded dropdown options. | Implemented in application/invoice forms; search and keyboard checks pass. |
| 5 | Run adviser and finance usability sessions | Automated tests establish functionality; staff feedback reveals what is confusing or slow. | Observation form and task guide ready; actual participants pending. |

## 1. Finish the SQL migration

Move remaining CRM mutations, validation, account/security handlers, workflow
reads, and background workers to scoped SQL repositories. Preserve ownership,
workspace isolation, related-record visibility, revision checks, audit history,
and transaction rollback behavior.

Acceptance criteria:

- Runtime requests and workers use committed SQL rather than a shared mutable
  memory cache; bootstrap/import code is separate from normal operation.
- Tests cover concurrent edits, worker execution, account/session revocation,
  startup, and workspace boundaries across separate backend processes.
- Repeat the documented realistic workload benchmarks and record read latency,
  write latency, and allocations before and after the migration.
- Remove the one-process restriction only after the implementation and tests
  demonstrate that separate processes cannot lose updates or duplicate work.

Separate-process regression checks now pass. SQLite still serializes writers,
and large writes retain costly workspace projections. Measure deployment-specific
load and share secrets/file storage before scaling; this is not a production
capacity claim.

## 2. Test real integrations end to end

Use a designated acceptance environment, configured provider accounts, approved
test recipients, and Stripe test mode. Connection checks remain an initial setup
step, not the acceptance result.

Acceptance criteria:

- Email: confirm delivery to a test inbox, reply ingestion and association with
  the intended record, plus understandable failure and retry states.
- WhatsApp: confirm an outbound message and inbound reply are attached to the
  correct conversation, including repeated webhook delivery and provider errors.
- Payments: exercise checkout, signed callbacks, invoice/payment updates, replay
  protection, and failed or cancelled checkout without duplicate payments.
- AI: verify an actual response, accessible source links, saved conversation
  history, explicit confirmation for actions, viewer restrictions, and failure
  handling for unavailable providers or exhausted limits.
- Record the environment, date, expected result, observed result, and sanitized
  provider references for each scenario. Never put credentials in test reports.

Live execution depends on configured accounts and a supplied testing environment
and recipients. Do not mark these checks passed based on mocked tests alone.

## 3. Make application handoffs explicit

Add a visible handoff state to the application and student journey: current owner,
next action, due date, proposed receiving staff member, and handoff status.

Acceptance criteria:

- Staff can propose a handoff with its reason and expected next action; the
  receiving staff member can accept or decline it.
- The current owner remains responsible until an authorized handoff is accepted.
- Pending, accepted, declined, and cancelled handoffs appear in record history.
- Receiving staff and owners see pending or overdue handoffs in their daily work.
- Permissions, workspace boundaries, and stale changes are checked on the server.

## 4. Add searchable record pickers

Replace limited student, contact, and application dropdowns with server-backed
search and pagination, starting with application and invoice forms.

Acceptance criteria:

- Staff can find accessible records beyond the first loaded page.
- Keep the selected record visible while searching, loading, or editing an
  existing record; opening a form must not silently clear a stored relationship.
- Show useful distinguishing details for similarly named records, plus loading,
  empty, and retry states. Ignore responses to superseded searches.
- Support keyboard selection, visible focus, labels, and mobile touch controls.
- Search and saves enforce access rules without exposing inaccessible records.

## 5. Run adviser and finance usability sessions

Ask actual advisers and finance staff to complete everyday tasks in a test
workspace. Prepare the sessions without contacting participants automatically.

Suggested tasks:

- Adviser: identify today's highest-priority application, find its missing
  documents, update the next action, and hand it to another adviser.
- Adviser: find a student in a large dataset and review their application and
  portal response without losing context.
- Finance: find an application, create its invoice, record a test receipt, and
  explain the remaining balance and any pending approval.
- Both: recover from a failed integration or stale edit, then repeat a key task
  on a phone using keyboard access where applicable.

Acceptance criteria:

- Record completion, time, errors, requests for help, and participants' comments
  for each task. Keep participant information and customer data private.
- Turn observed obstacles into reproducible issues, ordered by their impact on
  task completion rather than assumed importance.
- Implement the highest-impact fixes and repeat the affected tasks with staff.
- Report actual findings separately from automated test results. Sessions remain
  pending until advisers and finance staff participate.
