# Saves, application requirements, handoffs and receipts

## Faster saves

Ordinary contact edits read only the selected contact and an accessible matching
email, using a workspace/email SQL index. They retain normalization, ownership,
revision checks, transactional audit entries and notifications. Nested AI edits
keep their existing permission checks. Other module mutations still use broader
workspace projections.

Stage tracking now reads the latest recorded transition only for changed
applications. It skips unrelated saves and records actual stage changes once.

The existing benchmark with 5,000 contacts, applications and documents (20% in
another workspace) measured a contact update at **1.19 ms / 91 KB allocated**, over
three iterations on the same local machine. The previous measurement was
1.48 s / 408 MB. This is a short local measurement, not a production capacity
estimate. Run it with:

```bash
cd golang-backend
go test -run '^$' -bench 'BenchmarkRepositoryWorkload/validated-write$' -benchtime=3x -benchmem
```

## Application requirements

Applications and linked applications in the student journey have an **Application
requirements** panel. It identifies required documents, expiry/review needs,
outstanding invoice balances, pending receipt approvals, refunds awaiting action
and ownership handoffs. Action links open the relevant document, invoice or handoff.

The panel distinguishes documents that prevent automatic submission readiness
from items needing staff attention. It does not introduce a new rule that invoices
or handoffs prohibit stage changes. Results respect record access; an empty panel
means no issues were found in accessible records, not that hidden records were
reviewed or all submission requirements are complete.

## Handoff reminders

Overdue handoffs appear as urgent daily work, with a separate overdue count.
Owners and receiving staff get an in-app reminder after 24 hours without
acceptance, or when the handoff due date has passed. Each receives at most one
reminder per workspace day. Disabled accounts and staff who lost application
access are excluded.

Resolving a handoff stops new reminders and marks its outstanding notifications
read. History is retained. The current owner remains responsible until acceptance.
Reminder emails use the existing opt-in notification preference; this change
does not enable email for staff automatically.

## Receipt safeguards and approvals

Recording a payment checks the complete linked ledger, including restricted
receipts, without revealing their identities:

- A nonempty reference already used by an active receipt on the same invoice is
  rejected, ignoring surrounding whitespace and case. Confirmation cannot
  override this check.
- Matching amount, date and method produce a possible-duplicate warning. Staff
  must check history, enter a distinct receipt reference and explicitly confirm
  a separate receipt. The confirmation is recorded in its notes.
- Normal receipts keep the existing posting behavior. **Request manager approval**
  explicitly saves a pending receipt instead; it does not reduce the posted
  balance while pending.

The invoice's **Balance and receipt approvals** panel separates posted receipts,
outstanding balance, credit balance and amounts awaiting approval. Finance daily
work shows pending approval totals using the same accessible invoice aggregates.
Review pending receipts before requesting another payment.

Managers and administrators can approve or reject a pending receipt once.
Approval posts it; rejection retains it as void. Both preserve reviewer identity,
time and review history. Pending, rejected and approved receipts cannot be deleted
to bypass review/history; use the refund workflow for an approved receipt reversal.

Existing free-text invoice approval values are labeled **Approval note (legacy)**.
Those notes are not proof of a receipt review. The optional receipt approval
checkbox does not change the organization's default posting policy.

## Verification

Backend checks cover scoped saves and stale edits, hidden ledger duplicate checks,
approval permissions and balances, review history, private requirements, daily
reminder deduplication and stage transitions. The browser journey exercises the
document action, pending approval, manager approval, duplicate warning and
explicit separate-receipt confirmation alongside the existing workflow.

On 2026-10-10, the build, complete Go regression suite, targeted race checks,
`go vet`, all 16 live Chromium checks and the DOM/administration/mobile suites
passed. These results use test data and do not represent actual staff sessions
or real provider acceptance.
