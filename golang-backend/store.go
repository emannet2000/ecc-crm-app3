package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	LastTOTPStep     int64    `json:"-"`
	OrgID            string   `json:"orgId"`
	TeamID           string   `json:"teamId"`
	Role             string   `json:"role"`
	Disabled         bool     `json:"disabled"`
	TwoFactorEnabled bool     `json:"twoFactorEnabled"`
	TOTPSecret       string   `json:"-"`
	RecoveryHashes   []string `json:"-"`
	SessionVersion   int      `json:"-"`
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Email            string   `json:"email"`
	Password         string   `json:"-"`
}

type Contact struct {
	RecordScope
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Company     string   `json:"company"`
	Title       string   `json:"title"`
	Phone       string   `json:"phone"`
	Location    string   `json:"location"`
	Stage       string   `json:"stage"`
	LastContact string   `json:"lastContact"`
	Owner       string   `json:"owner"`
	Tags        []string `json:"tags"`
	Notes       string   `json:"notes"`
	CreatedAt   string   `json:"createdAt"`
}

type Deal struct {
	Currency     string        `json:"currency"`
	Probability  *float64      `json:"probability"`
	LineItems    []LineItem    `json:"lineItems"`
	StageHistory []StageChange `json:"stageHistory"`
	RecordScope
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	ContactID   string  `json:"contactId"`
	ContactName string  `json:"contactName"`
	Value       float64 `json:"value"`
	Stage       string  `json:"stage"`
	CloseDate   string  `json:"closeDate"`
	Owner       string  `json:"owner"`
	Notes       string  `json:"notes"`
	CreatedAt   string  `json:"createdAt"`
}

type Activity struct {
	RecordScope
	ID         string `json:"id"`
	ContactID  string `json:"contactId"`
	DealID     string `json:"dealId"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	OccurredAt string `json:"occurredAt"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  string `json:"createdAt"`
}

type Task struct {
	Recurrence   string `json:"recurrence"`
	ReminderDate string `json:"reminderDate"`
	RecordScope
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"dueDate"`
	ContactID   string `json:"contactId"`
	ContactName string `json:"contactName"`
	Owner       string `json:"owner"`
	CreatedAt   string `json:"createdAt"`
}

type School struct {
	Programs []Program `json:"programs"`
	RecordScope
	ID               string `json:"id"`
	Name             string `json:"name"`
	CountryCode      string `json:"countryCode"`
	CommissionRate   string `json:"commissionRate"`
	ContractStatus   string `json:"contractStatus"`
	StudentsEnrolled int    `json:"studentsEnrolled"`
	ContactPerson    string `json:"contactPerson"`
	Website          string `json:"website"`
	Notes            string `json:"notes"`
	CreatedBy        string `json:"createdBy"`
	CreatedAt        string `json:"createdAt"`
}

type Student struct {
	Intake           string          `json:"intake"`
	TestType         string          `json:"testType"`
	TestScore        float64         `json:"testScore"`
	Accommodation    string          `json:"accommodation"`
	TravelDate       string          `json:"travelDate"`
	ApplicationStage string          `json:"applicationStage"`
	PreDeparture     []ChecklistItem `json:"preDeparture"`
	RecordScope
	ID               string `json:"id"`
	Name             string `json:"name"`
	StudentCode      string `json:"studentCode"`
	CountryCode      string `json:"countryCode"`
	SchoolID         string `json:"schoolId"`
	SchoolName       string `json:"schoolName"`
	AgentID          string `json:"agentId"`
	AgentName        string `json:"agentName"`
	Program          string `json:"program"`
	AcceptanceStatus string `json:"acceptanceStatus"`
	VisaStatus       string `json:"visaStatus"`
	InvoiceStatus    string `json:"invoiceStatus"`
	Notes            string `json:"notes"`
	CreatedBy        string `json:"createdBy"`
	CreatedAt        string `json:"createdAt"`
}

type Agent struct {
	CommissionRate float64 `json:"commissionRatePercent"`
	RecordScope
	ID               string `json:"id"`
	Name             string `json:"name"`
	AgentCode        string `json:"agentCode"`
	CountryCode      string `json:"countryCode"`
	ContractStatus   string `json:"contractStatus"`
	AgentStatus      string `json:"agentStatus"`
	StudentsReferred int    `json:"studentsReferred"`
	Notes            string `json:"notes"`
	CreatedBy        string `json:"createdBy"`
	CreatedAt        string `json:"createdAt"`
}

type Lead struct {
	ConvertedContactID string  `json:"convertedContactId"`
	Score              int     `json:"score"`
	SourceCost         float64 `json:"sourceCost"`
	RecordScope
	ID                string `json:"id"`
	LeadNumber        string `json:"leadNumber"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	Nationality       string `json:"nationality"`
	CurrentCountry    string `json:"currentCountry"`
	InterestedCountry string `json:"interestedCountry"`
	InterestedService string `json:"interestedService"`
	Source            string `json:"source"`
	AssignedTo        string `json:"assignedTo"`
	Status            string `json:"status"`
	FollowUpDate      string `json:"followUpDate"`
	Notes             string `json:"notes"`
	CreatedBy         string `json:"createdBy"`
	CreatedAt         string `json:"createdAt"`
}

type Case struct {
	PartnerID    string `json:"partnerId"`
	StudentID    string `json:"studentId"`
	ParentCaseID string `json:"parentCaseId"`
	TemplateID   string `json:"templateId"`
	SLADays      int    `json:"slaDays"`
	AutoAdvance  bool   `json:"autoAdvance"`
	RecordScope
	ID                 string `json:"id"`
	CaseNumber         string `json:"caseNumber"`
	ClientID           string `json:"clientId"`
	ClientName         string `json:"clientName"`
	ServiceCategory    string `json:"serviceCategory"`
	DestinationCountry string `json:"destinationCountry"`
	VisaType           string `json:"visaType"`
	SchoolOrEmployer   string `json:"schoolOrEmployer"`
	AssignedOfficer    string `json:"assignedOfficer"`
	ExternalAdviser    string `json:"externalAdviser"`
	DateOpened         string `json:"dateOpened"`
	TargetSubmission   string `json:"targetSubmission"`
	ActualSubmission   string `json:"actualSubmission"`
	GovernmentRef      string `json:"governmentRef"`
	CurrentStage       string `json:"currentStage"`
	Priority           string `json:"priority"`
	NextAction         string `json:"nextAction"`
	NextDeadline       string `json:"nextDeadline"`
	Result             string `json:"result"`
	ClosureDate        string `json:"closureDate"`
	Notes              string `json:"notes"`
	CreatedBy          string `json:"createdBy"`
	CreatedAt          string `json:"createdAt"`
}

type Document struct {
	RecordScope
	ID                   string `json:"id"`
	CaseID               string `json:"caseId"`
	CaseNumber           string `json:"caseNumber"`
	DocName              string `json:"docName"`
	Required             bool   `json:"required"`
	DateRequested        string `json:"dateRequested"`
	DateReceived         string `json:"dateReceived"`
	ExpiryDate           string `json:"expiryDate"`
	VerifiedBy           string `json:"verifiedBy"`
	VerificationDate     string `json:"verificationDate"`
	Status               string `json:"status"`
	RejectionReason      string `json:"rejectionReason"`
	LatestVersion        int    `json:"latestVersion"`
	TranslationRequired  bool   `json:"translationRequired"`
	LegalizationRequired bool   `json:"legalizationRequired"`
	FilePath             string `json:"filePath"`
	Notes                string `json:"notes"`
	CreatedBy            string `json:"createdBy"`
	CreatedAt            string `json:"createdAt"`
}

type Invoice struct {
	Currency          string  `json:"currency"`
	DueDate           string  `json:"dueDate"`
	TaxAmount         float64 `json:"taxAmount"`
	Discount          float64 `json:"discount"`
	CreditAmount      float64 `json:"creditAmount"`
	OpeningReceived   float64 `json:"openingReceived"`
	LedgerInitialized bool    `json:"ledgerInitialized"`
	Dunning           bool    `json:"dunning"`
	RecordScope
	ID                    string  `json:"id"`
	InvoiceNumber         string  `json:"invoiceNumber"`
	ClientID              string  `json:"clientId"`
	ClientName            string  `json:"clientName"`
	CaseID                string  `json:"caseId"`
	CaseNumber            string  `json:"caseNumber"`
	TotalFee              float64 `json:"totalFee"`
	GovernmentFee         float64 `json:"governmentFee"`
	SchoolPartnerFee      float64 `json:"schoolPartnerFee"`
	AmountReceived        float64 `json:"amountReceived"`
	Balance               float64 `json:"balance"`
	PaymentMilestone      string  `json:"paymentMilestone"`
	PaymentMethod         string  `json:"paymentMethod"`
	OfficialReceiptNumber string  `json:"officialReceiptNumber"`
	RefundStatus          string  `json:"refundStatus"`
	ReferralCommission    float64 `json:"referralCommission"`
	PartnerPayable        float64 `json:"partnerPayable"`
	PaymentApproval       string  `json:"paymentApproval"`
	Notes                 string  `json:"notes"`
	CreatedBy             string  `json:"createdBy"`
	CreatedAt             string  `json:"createdAt"`
}

type Payment struct {
	ReviewedBy string `json:"reviewedBy,omitempty"`
	ReviewedAt string `json:"reviewedAt,omitempty"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
	RecordScope
	ID        string  `json:"id"`
	InvoiceID string  `json:"invoiceId"`
	Amount    float64 `json:"amount"`
	PaidOn    string  `json:"paidOn"`
	Method    string  `json:"method"`
	Reference string  `json:"reference"`
	Notes     string  `json:"notes"`
	CreatedBy string  `json:"createdBy"`
	CreatedAt string  `json:"createdAt"`
}

type Partner struct {
	CommissionRate float64 `json:"commissionRatePercent"`
	RecordScope
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	LegalCompanyName    string  `json:"legalCompanyName"`
	Country             string  `json:"country"`
	LicenseNumber       string  `json:"licenseNumber"`
	LicenseExpiry       string  `json:"licenseExpiry"`
	VerificationSource  string  `json:"verificationSource"`
	ContactPerson       string  `json:"contactPerson"`
	ContactEmail        string  `json:"contactEmail"`
	ContactPhone        string  `json:"contactPhone"`
	AgreementStart      string  `json:"agreementStart"`
	AgreementExpiry     string  `json:"agreementExpiry"`
	ServicesPermitted   string  `json:"servicesPermitted"`
	CommissionStructure string  `json:"commissionStructure"`
	PaymentTerms        string  `json:"paymentTerms"`
	CasesReferred       int     `json:"casesReferred"`
	CasesConverted      int     `json:"casesConverted"`
	AmountPayable       float64 `json:"amountPayable"`
	ComplianceNotes     string  `json:"complianceNotes"`
	Notes               string  `json:"notes"`
	CreatedBy           string  `json:"createdBy"`
	CreatedAt           string  `json:"createdAt"`
}

var (
	mu         sync.RWMutex
	users      = map[string]User{}
	contacts   = []Contact{}
	deals      = []Deal{}
	activities = []Activity{}
	tasks      = []Task{}
	schools    = []School{}
	students   = []Student{}
	agents     = []Agent{}
	leads      = []Lead{}
	cases      = []Case{}
	documents  = []Document{}
	invoices   = []Invoice{}
	payments   = []Payment{}
	partners   = []Partner{}
)

func newID(prefix string) string {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(entropy[:])
}

func parseLimitOffset(r *http.Request, defaultLimit, defaultOffset int) (limit, offset int) {
	limit, offset = defaultLimit, defaultOffset
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

func slicePage[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func seedData() {
	hash, _ := bcrypt.GenerateFromPassword([]byte("Demo1234"), bcrypt.DefaultCost)
	demo := User{
		ID:       "u_1",
		Name:     "Demo User",
		Email:    "demo@northwind.dev",
		Password: string(hash),
	}
	users[demo.Email] = demo
	log.Println("Seeded demo user: demo@northwind.dev / Demo1234")

	contacts = []Contact{
		{
			ID: "c_1", Name: "Ada Lovelace", Email: "ada@analytical.io",
			Company: "Analytical Engines", Title: "Chief Scientist",
			Phone: "+44 20 7946 0958", Location: "London, UK",
			Stage: "Customer", LastContact: "2026-09-08", Owner: "Demo User",
			Tags:      []string{"VIP", "Technical", "Q4-target"},
			Notes:     "Met at Q3 conference. Decision maker for technical purchases. Interested in the enterprise tier with SSO and audit logs.",
			CreatedAt: "2026-01-15",
		},
		{
			ID: "c_2", Name: "Grace Hopper", Email: "grace@navy.mil",
			Company: "US Navy", Title: "Rear Admiral",
			Phone: "+1 202 555 0142", Location: "Washington, DC",
			Stage: "Customer", LastContact: "2026-09-05", Owner: "Demo User",
			Tags:      []string{"VIP", "Government"},
			Notes:     "Long-time customer. Renewed for 3 years.",
			CreatedAt: "2025-06-02",
		},
		{
			ID: "c_3", Name: "Alan Turing", Email: "alan@bletchley.uk",
			Company: "Bletchley Park", Title: "Research Lead",
			Phone: "+44 1908 640404", Location: "Milton Keynes, UK",
			Stage: "Qualified", LastContact: "2026-09-01", Owner: "Demo User",
			Tags:      []string{"Technical", "Research"},
			Notes:     "Evaluating our cryptography features. Warm lead.",
			CreatedAt: "2026-08-12",
		},
		{
			ID: "c_4", Name: "Katherine Johnson", Email: "kj@nasa.gov",
			Company: "NASA", Title: "Research Mathematician",
			Phone: "+1 281 483 0121", Location: "Houston, TX",
			Stage: "Proposal", LastContact: "2026-08-28", Owner: "Demo User",
			Tags:      []string{"Aerospace", "Q4-target"},
			Notes:     "Proposal v2 sent. Waiting on procurement review.",
			CreatedAt: "2026-07-01",
		},
		{
			ID: "c_5", Name: "Linus Torvalds", Email: "linus@kernel.org",
			Company: "Linux Foundation", Title: "Principal Engineer",
			Phone: "+1 415 555 0198", Location: "Portland, OR",
			Stage: "Lead", LastContact: "2026-08-22", Owner: "Demo User",
			Tags:      []string{"Open Source"},
			Notes:     "Inbound from conference talk. Not yet qualified.",
			CreatedAt: "2026-08-22",
		},
		{
			ID: "c_6", Name: "Margaret Hamilton", Email: "mh@mit.edu",
			Company: "MIT", Title: "Professor",
			Phone: "+1 617 253 1000", Location: "Cambridge, MA",
			Stage: "Customer", LastContact: "2026-09-09", Owner: "Demo User",
			Tags:      []string{"Academic", "VIP"},
			Notes:     "Champion for the department-wide rollout.",
			CreatedAt: "2025-11-20",
		},
		{
			ID: "c_7", Name: "Donald Knuth", Email: "knuth@stanford.edu",
			Company: "Stanford", Title: "Professor Emeritus",
			Phone: "+1 650 723 2300", Location: "Stanford, CA",
			Stage: "Qualified", LastContact: "2026-08-30", Owner: "Demo User",
			Tags:      []string{"Academic"},
			Notes:     "",
			CreatedAt: "2026-08-25",
		},
		{
			ID: "c_8", Name: "Barbara Liskov", Email: "liskov@mit.edu",
			Company: "MIT", Title: "Institute Professor",
			Phone: "+1 617 253 1000", Location: "Cambridge, MA",
			Stage: "Proposal", LastContact: "2026-08-25", Owner: "Demo User",
			Tags:      []string{"Academic", "Technical"},
			Notes:     "Interested in the API integrations story.",
			CreatedAt: "2026-06-14",
		},
	}
	log.Println("Seeded 8 contacts")

	seedDeals()
	seedActivities()
	seedTasks()
	seedSchools()
	seedStudents()
	seedAgents()
	seedLeads()
	seedCases()
	seedDocuments()
	seedFinance()
	seedPartners()
}

func seedDeals() {
	deals = []Deal{
		{
			ID: "d_1", Title: "Analytical Engines — Enterprise tier", ContactID: "c_1", ContactName: "Ada Lovelace",
			Value: 48000, Stage: "Negotiation", CloseDate: "2026-10-15", Owner: "Demo User",
			Notes:     "SSO and audit logs are the blockers. Legal reviewing redlines.",
			CreatedAt: "2026-08-01",
		},
		{
			ID: "d_2", Title: "US Navy — 3yr renewal", ContactID: "c_2", ContactName: "Grace Hopper",
			Value: 96000, Stage: "Won", CloseDate: "2026-06-02", Owner: "Demo User",
			Notes:     "Renewed for 3 years.",
			CreatedAt: "2025-05-01",
		},
		{
			ID: "d_3", Title: "Bletchley Park — Cryptography add-on", ContactID: "c_3", ContactName: "Alan Turing",
			Value: 22000, Stage: "Qualified", CloseDate: "2026-11-01", Owner: "Demo User",
			Notes:     "Warm lead, evaluating cryptography features.",
			CreatedAt: "2026-08-12",
		},
		{
			ID: "d_4", Title: "NASA — Research suite", ContactID: "c_4", ContactName: "Katherine Johnson",
			Value: 61000, Stage: "Proposal", CloseDate: "2026-11-20", Owner: "Demo User",
			Notes:     "Proposal v2 sent. Waiting on procurement review.",
			CreatedAt: "2026-07-01",
		},
		{
			ID: "d_5", Title: "Linux Foundation — Trial", ContactID: "c_5", ContactName: "Linus Torvalds",
			Value: 8000, Stage: "Lead", CloseDate: "2026-12-01", Owner: "Demo User",
			Notes:     "Inbound from conference talk. Not yet qualified.",
			CreatedAt: "2026-08-22",
		},
		{
			ID: "d_6", Title: "MIT — Department rollout", ContactID: "c_6", ContactName: "Margaret Hamilton",
			Value: 120000, Stage: "Won", CloseDate: "2026-09-09", Owner: "Demo User",
			Notes:     "Champion for the department-wide rollout.",
			CreatedAt: "2025-11-20",
		},
		{
			ID: "d_7", Title: "Stanford — Evaluation", ContactID: "c_7", ContactName: "Donald Knuth",
			Value: 15000, Stage: "Lost", CloseDate: "2026-08-30", Owner: "Demo User",
			Notes:     "Went with an internal tool instead.",
			CreatedAt: "2026-08-25",
		},
	}
	log.Println("Seeded 7 deals")
}

func seedActivities() {
	activities = []Activity{
		{
			ID: "a_1", ContactID: "c_1", DealID: "d_1",
			Kind: "email", Title: "Proposal sent",
			Body:       "Sent the v2 pricing proposal — enterprise tier with SSO and audit logs.",
			OccurredAt: "2026-09-08", CreatedBy: "Demo User", CreatedAt: "2026-09-08",
		},
		{
			ID: "a_2", ContactID: "c_1", DealID: "d_1",
			Kind: "call", Title: "Discovery call",
			Body:       "Walked through current workflow. Pain points: reporting and permissions.",
			OccurredAt: "2026-09-05", CreatedBy: "Demo User", CreatedAt: "2026-09-05",
		},
		{
			ID: "a_3", ContactID: "c_1", DealID: "",
			Kind: "meeting", Title: "Product demo",
			Body:       "Showed the pipeline view and contact detail. Positive feedback.",
			OccurredAt: "2026-08-28", CreatedBy: "Demo User", CreatedAt: "2026-08-28",
		},
		{
			ID: "a_4", ContactID: "c_3", DealID: "d_3",
			Kind: "note", Title: "Evaluating cryptography features",
			Body:       "Sent whitepaper on AES-256 at rest and TLS 1.3.",
			OccurredAt: "2026-09-01", CreatedBy: "Demo User", CreatedAt: "2026-09-01",
		},
		{
			ID: "a_5", ContactID: "c_4", DealID: "d_4",
			Kind: "email", Title: "Proposal v2 sent",
			Body:       "Procurement requested updated terms. Waiting on legal review.",
			OccurredAt: "2026-08-28", CreatedBy: "Demo User", CreatedAt: "2026-08-28",
		},
		{
			ID: "a_6", ContactID: "c_6", DealID: "d_6",
			Kind: "meeting", Title: "Rollout kickoff",
			Body:       "Confirmed timeline and success metrics with department heads.",
			OccurredAt: "2026-09-09", CreatedBy: "Demo User", CreatedAt: "2026-09-09",
		},
		{
			ID: "a_7", ContactID: "c_2", DealID: "d_2",
			Kind: "note", Title: "Renewal signed",
			Body:       "3-year renewal closed. Contract on file.",
			OccurredAt: "2026-06-02", CreatedBy: "Demo User", CreatedAt: "2026-06-02",
		},
	}
	log.Println("Seeded 7 activities")
}

func seedTasks() {
	tasks = []Task{
		{ID: "t_1", Title: "Send revised SSO proposal", Description: "Include audit-log addendum Ada requested.", Status: "todo", DueDate: "2026-09-25", ContactID: "c_1", ContactName: "Ada Lovelace", Owner: "Demo User", CreatedAt: "2026-09-10"},
		{ID: "t_2", Title: "Schedule Navy QBR", Description: "Quarterly business review with Grace's team.", Status: "in_progress", DueDate: "2026-09-20", ContactID: "c_2", ContactName: "Grace Hopper", Owner: "Demo User", CreatedAt: "2026-09-08"},
		{ID: "t_3", Title: "Crypto whitepaper follow-up", Description: "Check if Alan finished the evaluation.", Status: "todo", DueDate: "2026-09-28", ContactID: "c_3", ContactName: "Alan Turing", Owner: "Demo User", CreatedAt: "2026-09-09"},
		{ID: "t_4", Title: "NASA procurement check-in", Description: "Ping legal on proposal v2 status.", Status: "todo", DueDate: "2026-09-18", ContactID: "c_4", ContactName: "Katherine Johnson", Owner: "Demo User", CreatedAt: "2026-09-05"},
		{ID: "t_5", Title: "Close MIT rollout checklist", Description: "Confirm success metrics with department heads.", Status: "done", DueDate: "2026-09-12", ContactID: "c_6", ContactName: "Margaret Hamilton", Owner: "Demo User", CreatedAt: "2026-09-01"},
		{ID: "t_6", Title: "Internal demo for new pipeline view", Description: "Prep slides for Friday all-hands.", Status: "in_progress", DueDate: "2026-09-26", ContactID: "", ContactName: "", Owner: "Demo User", CreatedAt: "2026-09-11"},
	}
	log.Println("Seeded 6 tasks")
}

func seedSchools() {
	schools = []School{
		{
			ID:               "s_1",
			Name:             "University of Toronto",
			CountryCode:      "CA",
			CommissionRate:   "15%",
			ContractStatus:   "Signed",
			StudentsEnrolled: 12,
			ContactPerson:    "Patricia Lam",
			Website:          "https://utoronto.ca",
			Notes:            "Top-tier partner. Prefers email contact.",
			CreatedBy:        "Demo User",
			CreatedAt:        "2026-03-10",
		},
		{
			ID:               "s_2",
			Name:             "University of Melbourne",
			CountryCode:      "AU",
			CommissionRate:   "12.5%",
			ContractStatus:   "Signed",
			StudentsEnrolled: 8,
			ContactPerson:    "James Whitford",
			Website:          "https://unimelb.edu.au",
			Notes:            "Strong business school. Quarterly review calls.",
			CreatedBy:        "Demo User",
			CreatedAt:        "2026-02-04",
		},
		{
			ID:               "s_3",
			Name:             "Auckland Institute of Studies",
			CountryCode:      "NZ",
			CommissionRate:   "10%",
			ContractStatus:   "Pending",
			StudentsEnrolled: 3,
			ContactPerson:    "Hine Ngata",
			Website:          "https://ais.ac.nz",
			Notes:            "Contract under legal review. Do not submit yet.",
			CreatedBy:        "Demo User",
			CreatedAt:        "2026-08-15",
		},
		{
			ID:               "s_4",
			Name:             "Seneca College",
			CountryCode:      "CA",
			CommissionRate:   "To be agreed",
			ContractStatus:   "Follow-up Required",
			StudentsEnrolled: 0,
			ContactPerson:    "",
			Website:          "https://senecacollege.ca",
			Notes:            "Initial outreach May 2026. No response yet.",
			CreatedBy:        "Demo User",
			CreatedAt:        "2026-05-22",
		},
	}
	log.Println("Seeded 4 schools")
}

func seedStudents() {
	students = []Student{
		{
			ID: "st_1", Name: "Maria Santos", StudentCode: "ECC-PH-2026-00001",
			CountryCode: "PH", SchoolID: "s_1", SchoolName: "University of Toronto",
			AgentID: "ag_1", AgentName: "Ravi Kumar",
			Program: "MSc Computer Science", AcceptanceStatus: "Accepted",
			VisaStatus: "Pending", InvoiceStatus: "Issued",
			Notes:     "Strong candidate. Visa appointment scheduled for October.",
			CreatedBy: "Demo User", CreatedAt: "2026-06-12",
		},
		{
			ID: "st_2", Name: "John Okonkwo", StudentCode: "ECC-NG-2026-00002",
			CountryCode: "NG", SchoolID: "s_2", SchoolName: "University of Melbourne",
			AgentID: "ag_2", AgentName: "Fatima Al-Zahra",
			Program: "MBA", AcceptanceStatus: "Accepted",
			VisaStatus: "Approved", InvoiceStatus: "Paid",
			Notes:     "Visa approved. Enrolled for the February intake.",
			CreatedBy: "Demo User", CreatedAt: "2026-04-03",
		},
		{
			ID: "st_3", Name: "Priya Sharma", StudentCode: "ECC-IN-2026-00003",
			CountryCode: "IN", SchoolID: "s_1", SchoolName: "University of Toronto",
			AgentID: "ag_1", AgentName: "Ravi Kumar",
			Program: "BSc Data Science", AcceptanceStatus: "Waitlisted",
			VisaStatus: "Not Started", InvoiceStatus: "Not Issued",
			Notes:     "On waitlist — awaiting university decision.",
			CreatedBy: "Demo User", CreatedAt: "2026-08-20",
		},
		{
			ID: "st_4", Name: "Ahmed Hassan", StudentCode: "ECC-EG-2026-00004",
			CountryCode: "EG", SchoolID: "s_3", SchoolName: "Auckland Institute of Studies",
			AgentID: "", AgentName: "",
			Program: "Diploma in IT", AcceptanceStatus: "Pending",
			VisaStatus: "Not Started", InvoiceStatus: "Not Issued",
			Notes:     "Application submitted. Awaiting school response.",
			CreatedBy: "Demo User", CreatedAt: "2026-09-01",
		},
	}
	log.Println("Seeded 4 students")
}

func seedAgents() {
	agents = []Agent{
		{
			ID: "ag_1", Name: "Ravi Kumar", AgentCode: "ECC-AG-00001",
			CountryCode: "IN", ContractStatus: "Signed", AgentStatus: "Active",
			StudentsReferred: 14,
			Notes:            "Top performer for Canada and Australia intakes.",
			CreatedBy:        "Demo User", CreatedAt: "2025-08-14",
		},
		{
			ID: "ag_2", Name: "Fatima Al-Zahra", AgentCode: "ECC-AG-00002",
			CountryCode: "AE", ContractStatus: "Signed", AgentStatus: "Active",
			StudentsReferred: 9,
			Notes:            "Strong in UAE and Gulf referrals.",
			CreatedBy:        "Demo User", CreatedAt: "2025-11-20",
		},
		{
			ID: "ag_3", Name: "Chen Wei", AgentCode: "ECC-AG-00003",
			CountryCode: "CN", ContractStatus: "Pending", AgentStatus: "Inactive",
			StudentsReferred: 0,
			Notes:            "Contract under negotiation. Not yet active.",
			CreatedBy:        "Demo User", CreatedAt: "2026-07-03",
		},
		{
			ID: "ag_4", Name: "Sofia Martinez", AgentCode: "ECC-AG-00004",
			CountryCode: "PH", ContractStatus: "Not Signed", AgentStatus: "Inactive",
			StudentsReferred: 0,
			Notes:            "Initial contact. Awaiting response.",
			CreatedBy:        "Demo User", CreatedAt: "2026-09-01",
		},
	}
	log.Println("Seeded 4 agents")
}

func seedLeads() {
	leads = []Lead{
		{
			ID: "l_1", LeadNumber: "ECC-LD-2026-00001",
			Name: "Carlos Reyes", Email: "carlos.reyes@example.com", Phone: "+63 917 555 0101",
			Nationality: "PH", CurrentCountry: "PH", InterestedCountry: "CA",
			InterestedService: "Canada student pathway",
			Source:            "Facebook", AssignedTo: "Demo User", Status: "Contacted",
			FollowUpDate: "2026-10-05",
			Notes:        "Downloaded the Canada guide. Interested in computer science programs.",
			CreatedBy:    "Demo User", CreatedAt: "2026-09-15",
		},
		{
			ID: "l_2", LeadNumber: "ECC-LD-2026-00002",
			Name: "Aisha Bello", Email: "aisha.bello@example.com", Phone: "+234 802 555 0202",
			Nationality: "NG", CurrentCountry: "NG", InterestedCountry: "AU",
			InterestedService: "Australia student application",
			Source:            "Instagram", AssignedTo: "Demo User", Status: "Qualified",
			FollowUpDate: "2026-09-30",
			Notes:        "Strong academic record. Ready for consultation.",
			CreatedBy:    "Demo User", CreatedAt: "2026-09-12",
		},
		{
			ID: "l_3", LeadNumber: "ECC-LD-2026-00003",
			Name: "Diego Fernandez", Email: "diego.f@example.com", Phone: "+34 600 555 0303",
			Nationality: "ES", CurrentCountry: "AE", InterestedCountry: "NZ",
			InterestedService: "New Zealand student pathway",
			Source:            "Referral", AssignedTo: "Demo User", Status: "New",
			FollowUpDate: "",
			Notes:        "Referred by an existing client. Awaiting first contact.",
			CreatedBy:    "Demo User", CreatedAt: "2026-09-22",
		},
		{
			ID: "l_4", LeadNumber: "ECC-LD-2026-00004",
			Name: "Mei Lin", Email: "mei.lin@example.com", Phone: "+86 138 5555 0404",
			Nationality: "CN", CurrentCountry: "CN", InterestedCountry: "GB",
			InterestedService: "UK student application",
			Source:            "Website", AssignedTo: "Demo User", Status: "Consultation Booked",
			FollowUpDate: "2026-10-01",
			Notes:        "Consultation scheduled for next week.",
			CreatedBy:    "Demo User", CreatedAt: "2026-09-18",
		},
		{
			ID: "l_5", LeadNumber: "ECC-LD-2026-00005",
			Name: "Fatou Diallo", Email: "fatou.d@example.com", Phone: "+221 77 555 0505",
			Nationality: "SN", CurrentCountry: "SN", InterestedCountry: "CA",
			InterestedService: "Canada student pathway",
			Source:            "Walk-in", AssignedTo: "Demo User", Status: "Converted",
			FollowUpDate: "",
			Notes:        "Signed agreement. Converted to student record.",
			CreatedBy:    "Demo User", CreatedAt: "2026-08-10",
		},
	}
	log.Println("Seeded 5 leads")
}

func seedCases() {
	cases = []Case{
		{
			ID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			ClientID: "c_1", ClientName: "Ada Lovelace",
			ServiceCategory:    "Canada student pathway",
			DestinationCountry: "CA", VisaType: "Study Permit",
			SchoolOrEmployer: "University of Toronto",
			AssignedOfficer:  "Demo User",
			DateOpened:       "2026-06-01", TargetSubmission: "2026-10-01",
			CurrentStage: "Documents Under Review", Priority: "High",
			NextAction: "Awaiting transcript verification", NextDeadline: "2026-09-30",
			Notes:     "Strong candidate. Visa appointment scheduled for October.",
			CreatedBy: "Demo User", CreatedAt: "2026-06-01",
		},
		{
			ID: "case_2", CaseNumber: "ECC-CASE-2026-00002",
			ClientID: "c_2", ClientName: "Grace Hopper",
			ServiceCategory:    "Australia student application",
			DestinationCountry: "AU", VisaType: "Subclass 500",
			SchoolOrEmployer: "University of Melbourne",
			AssignedOfficer:  "Demo User",
			DateOpened:       "2026-04-10", TargetSubmission: "2026-05-15",
			ActualSubmission: "2026-05-12", GovernmentRef: "AU-2026-884421",
			CurrentStage: "Approved", Priority: "Medium",
			NextAction: "Enrollment confirmation", NextDeadline: "2026-10-15",
			Result:    "Approved",
			Notes:     "Visa approved. Enrolled for February intake.",
			CreatedBy: "Demo User", CreatedAt: "2026-04-10",
		},
		{
			ID: "case_3", CaseNumber: "ECC-CASE-2026-00003",
			ClientID: "c_3", ClientName: "Alan Turing",
			ServiceCategory:    "New Zealand student pathway",
			DestinationCountry: "NZ", VisaType: "Fee-paying Student",
			SchoolOrEmployer: "Auckland Institute of Studies",
			AssignedOfficer:  "Demo User",
			DateOpened:       "2026-08-15", TargetSubmission: "2026-11-01",
			CurrentStage: "Assessment", Priority: "Medium",
			NextAction: "Eligibility review", NextDeadline: "2026-10-05",
			Notes:     "Initial eligibility check scheduled.",
			CreatedBy: "Demo User", CreatedAt: "2026-08-15",
		},
	}
	log.Println("Seeded 3 cases")
}

func seedDocuments() {
	documents = []Document{
		{ID: "doc_1", CaseID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			DocName: "Passport", Required: true,
			DateRequested: "2026-06-02", DateReceived: "2026-06-05",
			Status: "Verified", VerifiedBy: "Demo User", VerificationDate: "2026-06-06",
			LatestVersion: 1, CreatedBy: "Demo User", CreatedAt: "2026-06-02"},
		{ID: "doc_2", CaseID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			DocName: "Academic Transcripts", Required: true,
			DateRequested: "2026-06-02", DateReceived: "2026-06-10",
			Status: "Under Review", LatestVersion: 1,
			Notes:     "Awaiting university verification.",
			CreatedBy: "Demo User", CreatedAt: "2026-06-02"},
		{ID: "doc_3", CaseID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			DocName: "IELTS Score Report", Required: true,
			DateRequested: "2026-06-02", DateReceived: "2026-06-08",
			ExpiryDate: "2028-06-08", Status: "Verified",
			VerifiedBy: "Demo User", VerificationDate: "2026-06-09",
			LatestVersion: 1, CreatedBy: "Demo User", CreatedAt: "2026-06-02"},
		{ID: "doc_4", CaseID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			DocName: "Statement of Purpose", Required: true,
			DateRequested: "2026-06-02", Status: "Requested",
			LatestVersion: 1, CreatedBy: "Demo User", CreatedAt: "2026-06-02"},
		{ID: "doc_5", CaseID: "case_2", CaseNumber: "ECC-CASE-2026-00002",
			DocName: "Passport", Required: true,
			DateRequested: "2026-04-11", DateReceived: "2026-04-12",
			Status: "Verified", VerifiedBy: "Demo User", VerificationDate: "2026-04-13",
			LatestVersion: 1, CreatedBy: "Demo User", CreatedAt: "2026-04-11"},
		{ID: "doc_6", CaseID: "case_2", CaseNumber: "ECC-CASE-2026-00002",
			DocName: "Bank Statement", Required: true,
			DateRequested: "2026-04-11", DateReceived: "2026-04-15",
			Status: "Verified", VerifiedBy: "Demo User", VerificationDate: "2026-04-16",
			LatestVersion: 1, CreatedBy: "Demo User", CreatedAt: "2026-04-11"},
	}
	log.Println("Seeded 6 documents")
}

func seedFinance() {
	invoices = []Invoice{
		{ID: "inv_1", InvoiceNumber: "ECC-INV-2026-00001",
			ClientID: "c_1", ClientName: "Ada Lovelace",
			CaseID: "case_1", CaseNumber: "ECC-CASE-2026-00001",
			TotalFee: 4500, GovernmentFee: 235, SchoolPartnerFee: 0,
			AmountReceived: 2500, Balance: 2235,
			PaymentMilestone: "Instalment Due", PaymentMethod: "Bank Transfer",
			OfficialReceiptNumber: "ECC-RCP-2026-00001",
			PaymentApproval:       "Demo User", Notes: "Deposit received.",
			CreatedBy: "Demo User", CreatedAt: "2026-06-15"},
		{ID: "inv_2", InvoiceNumber: "ECC-INV-2026-00002",
			ClientID: "c_2", ClientName: "Grace Hopper",
			CaseID: "case_2", CaseNumber: "ECC-CASE-2026-00002",
			TotalFee: 5500, GovernmentFee: 620, SchoolPartnerFee: 500,
			AmountReceived: 6620, Balance: 0,
			PaymentMilestone: "Fully Paid", PaymentMethod: "Bank Transfer",
			OfficialReceiptNumber: "ECC-RCP-2026-00002",
			ReferralCommission:    900, PartnerPayable: 500,
			PaymentApproval: "Demo User", Notes: "Fully paid.",
			CreatedBy: "Demo User", CreatedAt: "2026-04-20"},
	}
	payments = []Payment{
		{ID: "pay_1", InvoiceID: "inv_1", Amount: 2500, PaidOn: "2026-06-20",
			Method: "Bank Transfer", Reference: "TRX-884421",
			Notes:     "Deposit.",
			CreatedBy: "Demo User", CreatedAt: "2026-06-20"},
		{ID: "pay_2", InvoiceID: "inv_2", Amount: 6620, PaidOn: "2026-04-25",
			Method: "Bank Transfer", Reference: "TRX-884422",
			Notes:     "Full payment.",
			CreatedBy: "Demo User", CreatedAt: "2026-04-25"},
	}
	log.Println("Seeded 2 invoices + 2 payments")
}

func seedPartners() {
	partners = []Partner{
		{ID: "p_1", Type: "Immigration Lawyer",
			LegalCompanyName: "Al Farsi Legal Consultancy",
			Country:          "AE", LicenseNumber: "DXB-LAW-2024-0421",
			LicenseExpiry: "2027-03-31", VerificationSource: "Dubai Legal Affairs",
			ContactPerson: "Ahmed Al Farsi", ContactEmail: "ahmed@alfarsi.ae",
			ContactPhone:   "+971 4 555 0101",
			AgreementStart: "2025-04-01", AgreementExpiry: "2027-03-31",
			ServicesPermitted:   "Immigration consultation, visa review",
			CommissionStructure: "Flat fee per case", PaymentTerms: "Net 30",
			CasesReferred: 24, CasesConverted: 18, AmountPayable: 12000,
			ComplianceNotes: "License verified.",
			CreatedBy:       "Demo User", CreatedAt: "2025-04-01"},
		{ID: "p_2", Type: "Recruitment Agency",
			LegalCompanyName: "Gulf Talent Partners",
			Country:          "AE", LicenseNumber: "DXB-EMP-2023-1188",
			LicenseExpiry: "2026-12-31", VerificationSource: "MOHRE",
			ContactPerson: "Sarah Khalid", ContactEmail: "sarah@gulftalent.ae",
			ContactPhone:   "+971 4 555 0202",
			AgreementStart: "2024-01-15", AgreementExpiry: "2026-12-31",
			ServicesPermitted:   "Employer referrals",
			CommissionStructure: "10% of first-year salary", PaymentTerms: "Net 45",
			CasesReferred: 15, CasesConverted: 9, AmountPayable: 8500,
			ComplianceNotes: "License renewal due Dec 2026.",
			CreatedBy:       "Demo User", CreatedAt: "2024-01-15"},
		{ID: "p_3", Type: "Travel Agency",
			LegalCompanyName: "Emirates Travel Hub",
			Country:          "AE", LicenseNumber: "DXB-TRV-2025-0044",
			LicenseExpiry: "2028-06-30", VerificationSource: "DET",
			ContactPerson: "Rashid Al Marri", ContactEmail: "rashid@emtravel.ae",
			ContactPhone:   "+971 4 555 0303",
			AgreementStart: "2025-07-01", AgreementExpiry: "2028-06-30",
			ServicesPermitted:   "Flight booking, travel insurance",
			CommissionStructure: "Tiered", PaymentTerms: "Net 15",
			CasesReferred: 8, CasesConverted: 6, AmountPayable: 2400,
			CreatedBy: "Demo User", CreatedAt: "2025-07-01"},
	}
	log.Println("Seeded 3 partners")
}

func (state *runtimeStore) findUserByEmail(email string) (User, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	u, ok := state.users[strings.ToLower(strings.TrimSpace(email))]
	return u, ok
}

func findUserByEmail(email string) (User, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.findUserByEmail(email)
}

// ================= SCHOOLS: ACCESSORS =================

func (state *runtimeStore) listSchools() []School {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]School, len(state.schools))
	copy(out, state.schools)
	return out
}

func listSchools() []School {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listSchools()
}

func (state *runtimeStore) getSchoolByID(id string) (School, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, s := range state.schools {
		if s.ID == id {
			return s, true
		}
	}
	return School{}, false
}

func getSchoolByID(id string) (School, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getSchoolByID(id)
}

// ================= STUDENTS: ACCESSORS =================

func (state *runtimeStore) listStudents() []Student {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Student, len(state.students))
	copy(out, state.students)
	return out
}

func listStudents() []Student {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listStudents()
}

func (state *runtimeStore) getStudentByID(id string) (Student, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, s := range state.students {
		if s.ID == id {
			return s, true
		}
	}
	return Student{}, false
}

func getStudentByID(id string) (Student, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getStudentByID(id)
}

// ================= AGENTS: ACCESSORS =================

func (state *runtimeStore) listAgents() []Agent {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Agent, len(state.agents))
	copy(out, state.agents)
	return out
}

func listAgents() []Agent {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listAgents()
}

func (state *runtimeStore) getAgentByID(id string) (Agent, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, a := range state.agents {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

func getAgentByID(id string) (Agent, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getAgentByID(id)
}

// ================= LEADS: ACCESSORS =================

func (state *runtimeStore) listLeads() []Lead {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Lead, len(state.leads))
	copy(out, state.leads)
	return out
}

func listLeads() []Lead {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listLeads()
}

func (state *runtimeStore) getLeadByID(id string) (Lead, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, l := range state.leads {
		if l.ID == id {
			return l, true
		}
	}
	return Lead{}, false
}

func getLeadByID(id string) (Lead, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getLeadByID(id)
}

// ================= CASES: ACCESSORS =================

func (state *runtimeStore) listCases() []Case {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Case, len(state.cases))
	copy(out, state.cases)
	return out
}

func listCases() []Case {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listCases()
}

func (state *runtimeStore) getCaseByID(id string) (Case, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, c := range state.cases {
		if c.ID == id {
			return c, true
		}
	}
	return Case{}, false
}

func getCaseByID(id string) (Case, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getCaseByID(id)
}

// ================= DOCUMENTS: ACCESSORS =================

func (state *runtimeStore) listDocuments() []Document {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Document, len(state.documents))
	copy(out, state.documents)
	return out
}

func listDocuments() []Document {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listDocuments()
}

func (state *runtimeStore) getDocumentByID(id string) (Document, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, d := range state.documents {
		if d.ID == id {
			return d, true
		}
	}
	return Document{}, false
}

func getDocumentByID(id string) (Document, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getDocumentByID(id)
}

// ================= INVOICES: ACCESSORS =================

func (state *runtimeStore) listInvoices() []Invoice {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Invoice, len(state.invoices))
	copy(out, state.invoices)
	return out
}

func listInvoices() []Invoice {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listInvoices()
}

func (state *runtimeStore) getInvoiceByID(id string) (Invoice, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, inv := range state.invoices {
		if inv.ID == id {
			return inv, true
		}
	}
	return Invoice{}, false
}

func getInvoiceByID(id string) (Invoice, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getInvoiceByID(id)
}

// ================= PAYMENTS: ACCESSORS =================

func (state *runtimeStore) listPayments() []Payment {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Payment, len(state.payments))
	copy(out, state.payments)
	return out
}

func listPayments() []Payment {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listPayments()
}

// ================= PARTNERS: ACCESSORS =================

func (state *runtimeStore) listPartners() []Partner {
	state.mu.RLock()
	defer state.mu.RUnlock()
	out := make([]Partner, len(state.partners))
	copy(out, state.partners)
	return out
}

func listPartners() []Partner {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.listPartners()
}

func (state *runtimeStore) getPartnerByID(id string) (Partner, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, p := range state.partners {
		if p.ID == id {
			return p, true
		}
	}
	return Partner{}, false
}

func getPartnerByID(id string) (Partner, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.getPartnerByID(id)
}
