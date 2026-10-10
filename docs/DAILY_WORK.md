# Daily work and student journey

The home page starts with **My work today**, followed by the optional AI work
brief, analytics, and recent committed change history. Daily work does not require
AI credentials. It includes open tasks, active applications, due/missing lead
follow-ups, required documents needing review, and approaching/overdue deadlines.

**Mine and unassigned** uses the current staff assignment and includes unassigned
records. **Unassigned** isolates that queue. **All accessible work** includes other
staff assignments without widening record permissions. Cards link to the record.
The queue pages 25 items at a time; counts cover the whole matching dataset. It
has no AI worklist's 30-item cap. Deadlines use the workspace timezone. Finance
focus also adds outstanding invoices without combining different currencies.

The top-bar badge counts unread notifications returned by `/api/clock`, not
loaded record previews. Dashboard statistics read committed SQL with workspace
scope; recent changes use paginated scoped audit history. Empty/loading/error
states never masquerade as successful zero-count results.

## Student journey

Each student's **Student journey** combines linked applications, owner, stage,
next action/deadline, required-document progress, documents, invoices, recorded
payments, and recent messages. Documents and payments remain subject to their
parent's visibility. Only explicitly case-linked messages appear; messages on a
shared client without a case link do not belong to a specific student. The latest
25 messages are shown, with links to the individual application conversation.
Document, invoice and payment sections expand on demand. Currency is shown per
invoice/payment. Refresh journey reloads committed data.

## Navigation and form guidance

Administrators and managers initially see Management navigation; members and
viewers see Advising. **Navigation focus** offers Advising, Finance and Management
presets, saved per user in this browser. It changes prominent links and mobile
shortcuts, not security roles or permissions. **All modules** keeps every module
available. System administration remains administrator-only.

Student, lead and case forms check visible records after a short input debounce.
Warnings surface duplicate student codes/application identities, matching lead
email/phone, missing school/follow-up/next-action/deadline/student links, and
incomplete required documents for an edited case. The edited record is excluded
from duplicate matches. Warnings do not block legitimate saves or merge records.
Existing server validation still enforces required fields and record access.

## Integration connections

Workspace tools → Integrations now shows Email, WhatsApp, Payments and AI with
setup instructions, last check, and one of: needs configuration, disabled,
configured, connected, or failing. Administrators can run **Test connection**.

- Email checks SMTP connection, TLS, authentication and NOOP; it sends no email.
- WhatsApp checks access to the configured Meta phone number; it sends no message.
- Payments checks Stripe account access; it creates no payment.
- AI checks credentials and access to the selected model through the official
  [Retrieve model endpoint](https://developers.openai.com/api/reference/resources/models/methods/retrieve);
  it sends no CRM data and does not test generation or token quota.

Tests save sanitized results per workspace. Changing credentials/model/settings
invalidates the previous result; successful checks older than 24 hours require
retesting. A connection check does not replace acceptance testing of delivery,
checkout, callbacks or AI generation. Credentials remain private on the server.

## SQL transition and limits

Standard module list/detail URLs now use scoped SQL repositories with SQL paging,
counting, filtering, literal search and ordering. Related records check parent
access; school and agent counts use all visible linked students. Reports, daily
work, journey and audit history read committed scoped data.

Runtime mutations, account/security handlers and workers load committed SQL into
private request state. They write changed rows in the same transaction as history
and audit entries. Updates/deletes require the previous payload and organization
to match. SQL transactions reserve the write lock before reading; concurrent
startup also acquires this lock. Package-level records are limited to bootstrap,
legacy import and explicit test fixtures, not live request synchronization.

Separate-process tests cover concurrent startup and writes, stale revisions,
account/session changes, and recurring worker execution without duplicate tasks.
Queued email, WhatsApp and webhook workers claim jobs atomically. Review uncertain
external delivery before retrying; see [integration acceptance](INTEGRATION_ACCEPTANCE.md).
Processes must share the database, secrets and uploaded-file storage. SQLite
still serializes writers, and mutation validation still loads a whole workspace
projection. This migration removes dependence on the shared mutable cache; it
does not establish production capacity or make those expensive projections cheap.

Run `go test -run '^$' -bench 'Benchmark(RecordReads|RepositoryWorkload)' -benchmem`
for repeatable local measurements. Browser coverage includes the live student
workflow and navigation/form/mobile accessibility checks. See package scripts.

## Measured local baseline

Measured on 2026-10-10 with Go 1.26 on an Intel i5-2415M, three measured
iterations per operation. The workload adds 5,000 each of contacts, applications
and documents, with 20% belonging to another workspace, plus demo records.
Single-record comparisons use the separate `BenchmarkRecordReads` dataset.
These short local measurements are not production capacity estimates.

| Operation | Mean time | Allocated bytes per operation |
| --- | ---: | ---: |
| Scoped SQL single-record read | 0.62 ms | 40.5 KB |
| Legacy single-record read | 90.7 ms | 25.2 MB |
| Scoped paginated list | 14.4 ms | 354 KB |
| Full daily-work totals and first page | 347 ms | 90.6 MB |
| Validated contact update | 1.53 s | 447 MB |

Removing whole-store JSON decoding from record indexing reduced validated-write
allocations from the earlier 768 MB baseline. Daily-work projection and mutation validation remain substantial costs. Replace
workspace-wide projections with targeted SQL validation before increasing load.

## Mobile and keyboard checks

The live browser workflow checks 320, 390 and 768 CSS-pixel widths, horizontal
fit, keyboard operation and visible focus of expandable journey sections, and
44-pixel navigation/summary targets. The new controls use 44-pixel targets where
practical; [WCAG 2.2 target-size guidance](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum)
sets a 24-by-24 CSS-pixel minimum with defined exceptions. This focused check is
not a full WCAG conformance audit. Navigation and optional journey sections use
[progressive disclosure](https://www.nngroup.com/articles/progressive-disclosure/).

## Application handoffs, pickers and staff observations

Applications and student journeys show current responsibility, next action and
handoff history. Owners/managers can propose a handoff to staff who already have
access. The current owner stays responsible until the recipient accepts.
Acceptance checks revision and permissions, transfers the application and its
old-owner documents, and records history. Recipients can decline; owners/managers
can cancel. Pending handoffs appear in daily work.

Application and invoice forms now search accessible students, contacts and cases
through paginated SQL endpoints. Existing selections stay visible during search
or failure. Record IDs distinguish similar names; keyboard arrows, Enter, Escape,
visible focus and touch controls are supported.

Workspace tools includes Staff usability to record actual adviser/finance task
observations with workspace and owner privacy. Preparation instructions are in
[Staff usability](STAFF_USABILITY.md). Actual sessions remain pending.

## SQL migration measurement

The subsequent [operational improvements](OPERATIONAL_IMPROVEMENTS.md) reduce
ordinary contact saves to targeted SQL reads and remove the full-case stage
scan. Their latest contact-save benchmark is 1.19 ms / 91 KB allocated. The
measurements below describe the earlier migration baseline.

A repeat on the same machine and dataset, three iterations, measured SQL record
reads at 1.13 ms / 42.6 KB, paginated lists at 29.0 ms / 366 KB, daily work at
878 ms / 93.1 MB, and validated writes at 1.48 s / 408 MB. Browser verification
ran concurrently, so these timings are indicative rather than a controlled
before/after comparison. The legacy-style projected read path measured 448 ms /
45.7 MB. Allocation reductions do not remove the large-workspace write bottleneck.
Targeted SQL validation remains the next performance priority.

## Final verification of this change

On 2026-10-10, the build, full Go suite with race detection, `go vet`, 16
Chromium checks, DOM/administration/mobile suites, and three backup tests passed.
New coverage includes separate backend processes sharing SQL, handoff permissions
and stale acceptance, observation privacy, and picker pagination, selection
preservation, keyboard use and stale search responses. The real-provider runner
was exercised against the temporary unconfigured app and reported all four
integrations as not configured. No real provider delivery or actual staff
participation is included in these results.
