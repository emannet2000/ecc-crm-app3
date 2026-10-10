module Types exposing (..)

import Browser
import Dict exposing (Dict)
import Browser.Navigation as Nav
import Set exposing (Set)
import Url


type Mode
    = Login
    | Register


type Route
    = Home
    | Contacts
    | ContactDetail String
    | Deals
    | DealDetail String
    | Tasks
    | Reports
    | Workspace
    | Administration
    | Settings
    | Schools
    | SchoolDetail String
    | Students
    | StudentDetail String
    | Agents
    | AgentDetail String
    | Leads
    | LeadDetail String
    | Cases
    | CaseDetail String
    | Invoices
    | InvoiceDetail String
    | Partners
    | PartnerDetail String


type AlertType
    = AlertError
    | AlertSuccess


type alias Alert =
    { kind : AlertType
    , message : String
    }


type alias User =
    { role : String
    , id : String
    , name : String
    , email : String
    }


type alias Contact =
    { id : String
    , name : String
    , email : String
    , company : String
    , title : String
    , phone : String
    , location : String
    , stage : String
    , lastContact : String
    , owner : String
    , tags : List String
    , notes : String
    , createdAt : String
    }


type RemoteData a
    = NotAsked
    | Loading
    | Success a
    | Failure String


type alias ContactsData =
    { items : List Contact
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias ContactForm =
    { name : String
    , email : String
    , company : String
    , title : String
    , phone : String
    , location : String
    , stage : String
    , tags : List String
    , tagInput : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyContactForm : ContactForm
emptyContactForm =
    { name = ""
    , email = ""
    , company = ""
    , title = ""
    , phone = ""
    , location = ""
    , stage = "Lead"
    , tags = []
    , tagInput = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


contactToForm : Contact -> ContactForm
contactToForm c =
    { name = c.name
    , email = c.email
    , company = c.company
    , title = c.title
    , phone = c.phone
    , location = c.location
    , stage = c.stage
    , tags = c.tags
    , tagInput = ""
    , notes = c.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


type alias Deal =
    { id : String
    , title : String
    , contactId : String
    , contactName : String
    , value : Float
    , probability : Maybe Float
    , currency : String
    , stage : String
    , closeDate : String
    , ownerId : String
    , owner : String
    , notes : String
    , createdAt : String
    }


type alias DealsData =
    { items : List Deal
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias DealForm =
    { title : String
    , contactId : String
    , value : String
    , currency : String
    , stage : String
    , closeDate : String
    , owner : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyDealForm : DealForm
emptyDealForm =
    { title = ""
    , contactId = ""
    , value = ""
    , currency = "USD"
    , stage = "Lead"
    , closeDate = ""
    , owner = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


dealToForm : Deal -> DealForm
dealToForm d =
    { title = d.title
    , contactId = d.contactId
    , value = String.fromFloat d.value
    , currency = d.currency
    , stage = d.stage
    , closeDate = d.closeDate
    , owner = d.owner
    , notes = d.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


type alias Activity =
    { id : String
    , contactId : String
    , dealId : String
    , kind : String
    , title : String
    , body : String
    , occurredAt : String
    , createdBy : String
    , createdAt : String
    }


type alias Task =
    { id : String
    , title : String
    , description : String
    , status : String
    , dueDate : String
    , contactId : String
    , contactName : String
    , owner : String
    , createdAt : String
    }


type alias TasksData =
    { items : List Task
    , query : String
    , statusFilter : String
    , total : Int
    }


type alias TaskForm =
    { title : String
    , description : String
    , status : String
    , dueDate : String
    , contactId : String
    , owner : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyTaskForm : TaskForm
emptyTaskForm =
    { title = ""
    , description = ""
    , status = "todo"
    , dueDate = ""
    , contactId = ""
    , owner = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


taskToForm : Task -> TaskForm
taskToForm task =
    { title = task.title
    , description = task.description
    , status = task.status
    , dueDate = task.dueDate
    , contactId = task.contactId
    , owner = task.owner
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


taskStatuses : List String
taskStatuses =
    [ "todo", "in_progress", "done" ]


type alias ActivityForm =
    { kind : String
    , title : String
    , body : String
    , occurredAt : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyActivityForm : ActivityForm
emptyActivityForm =
    { kind = "note"
    , title = ""
    , body = ""
    , occurredAt = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


activityKinds : List String
activityKinds =
    [ "note", "call", "email", "whatsapp", "meeting", "task" ]


type alias ProfileForm =
    { name : String
    , email : String
    , errors : List ( String, String )
    , submitting : Bool
    , success : Maybe String
    }


emptyProfileForm : ProfileForm
emptyProfileForm =
    { name = ""
    , email = ""
    , errors = []
    , submitting = False
    , success = Nothing
    }


profileFromUser : User -> ProfileForm
profileFromUser u =
    { name = u.name
    , email = u.email
    , errors = []
    , submitting = False
    , success = Nothing
    }


type alias PasswordForm =
    { current : String
    , next : String
    , confirm : String
    , errors : List ( String, String )
    , submitting : Bool
    , success : Maybe String
    }


emptyPasswordForm : PasswordForm
emptyPasswordForm =
    { current = ""
    , next = ""
    , confirm = ""
    , errors = []
    , submitting = False
    , success = Nothing
    }


type alias ProfileUpdateResponse =
    { user : User
    , token : String
    }


type ApiError
    = FieldErrors (List ( String, String ))
    | GenericError String


type alias Form =
    { name : String
    , email : String
    , password : String
    , remember : Bool
    , otp : String
    }


type alias AuthResponse =
    { token : String
    , user : User
    }


type Field
    = NameField
    | EmailField
    | PasswordField



-- ========== SCHOOLS ==========


type alias School =
    { id : String
    , name : String
    , countryCode : String
    , commissionRate : String
    , contractStatus : String
    , studentsEnrolled : Int
    , contactPerson : String
    , website : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias SchoolsData =
    { items : List School
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias SchoolForm =
    { name : String
    , countryCode : String
    , commissionRate : String
    , contractStatus : String
    , studentsEnrolled : String
    , contactPerson : String
    , website : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptySchoolForm : SchoolForm
emptySchoolForm =
    { name = ""
    , countryCode = ""
    , commissionRate = ""
    , contractStatus = "Pending"
    , studentsEnrolled = "0"
    , contactPerson = ""
    , website = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


schoolToForm : School -> SchoolForm
schoolToForm s =
    { name = s.name
    , countryCode = s.countryCode
    , commissionRate = s.commissionRate
    , contractStatus = s.contractStatus
    , studentsEnrolled = String.fromInt s.studentsEnrolled
    , contactPerson = s.contactPerson
    , website = s.website
    , notes = s.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }



-- ========== STUDENTS ==========


type alias Student =
    { id : String
    , name : String
    , studentCode : String
    , countryCode : String
    , schoolId : String
    , schoolName : String
    , agentId : String
    , agentName : String
    , program : String
    , acceptanceStatus : String
    , visaStatus : String
    , invoiceStatus : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias StudentsData =
    { items : List Student
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias LinkedStudentsData =
    { parentId : String
    , items : List Student
    , total : Int
    }


type alias StudentForm =
    { name : String
    , studentCode : String
    , countryCode : String
    , schoolId : String
    , agentId : String
    , program : String
    , acceptanceStatus : String
    , visaStatus : String
    , invoiceStatus : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyStudentForm : StudentForm
emptyStudentForm =
    { name = ""
    , studentCode = ""
    , countryCode = ""
    , schoolId = ""
    , agentId = ""
    , program = ""
    , acceptanceStatus = "Pending"
    , visaStatus = "Not Started"
    , invoiceStatus = "Not Issued"
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


studentToForm : Student -> StudentForm
studentToForm s =
    { name = s.name
    , studentCode = s.studentCode
    , countryCode = s.countryCode
    , schoolId = s.schoolId
    , agentId = s.agentId
    , program = s.program
    , acceptanceStatus = s.acceptanceStatus
    , visaStatus = s.visaStatus
    , invoiceStatus = s.invoiceStatus
    , notes = s.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


acceptanceStatuses : List String
acceptanceStatuses =
    [ "Pending", "Accepted", "Rejected", "Waitlisted" ]


visaStatuses : List String
visaStatuses =
    [ "Not Started", "Pending", "Approved", "Denied" ]


invoiceStatuses : List String
invoiceStatuses =
    [ "Not Issued", "Issued", "Paid", "Overdue" ]



-- ========== AGENTS ==========


type alias Agent =
    { id : String
    , name : String
    , agentCode : String
    , countryCode : String
    , contractStatus : String
    , agentStatus : String
    , studentsReferred : Int
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias AgentsData =
    { items : List Agent
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias AgentForm =
    { name : String
    , agentCode : String
    , countryCode : String
    , contractStatus : String
    , agentStatus : String
    , studentsReferred : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyAgentForm : AgentForm
emptyAgentForm =
    { name = ""
    , agentCode = ""
    , countryCode = ""
    , contractStatus = "Pending"
    , agentStatus = "Active"
    , studentsReferred = "0"
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


agentToForm : Agent -> AgentForm
agentToForm a =
    { name = a.name
    , agentCode = a.agentCode
    , countryCode = a.countryCode
    , contractStatus = a.contractStatus
    , agentStatus = a.agentStatus
    , studentsReferred = String.fromInt a.studentsReferred
    , notes = a.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


agentContractStatuses : List String
agentContractStatuses =
    [ "Signed", "Not Signed", "Pending" ]


agentStatuses : List String
agentStatuses =
    [ "Active", "Inactive" ]



-- ========== LEADS ==========


type alias Lead =
    { id : String
    , leadNumber : String
    , name : String
    , email : String
    , phone : String
    , nationality : String
    , currentCountry : String
    , interestedCountry : String
    , interestedService : String
    , source : String
    , assignedTo : String
    , status : String
    , followUpDate : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias LeadsData =
    { items : List Lead
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias LeadForm =
    { leadNumber : String
    , name : String
    , email : String
    , phone : String
    , nationality : String
    , currentCountry : String
    , interestedCountry : String
    , interestedService : String
    , source : String
    , assignedTo : String
    , status : String
    , followUpDate : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyLeadForm : LeadForm
emptyLeadForm =
    { leadNumber = ""
    , name = ""
    , email = ""
    , phone = ""
    , nationality = ""
    , currentCountry = ""
    , interestedCountry = ""
    , interestedService = ""
    , source = "Website"
    , assignedTo = ""
    , status = "New"
    , followUpDate = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


leadToForm : Lead -> LeadForm
leadToForm l =
    { leadNumber = l.leadNumber
    , name = l.name
    , email = l.email
    , phone = l.phone
    , nationality = l.nationality
    , currentCountry = l.currentCountry
    , interestedCountry = l.interestedCountry
    , interestedService = l.interestedService
    , source = l.source
    , assignedTo = l.assignedTo
    , status = l.status
    , followUpDate = l.followUpDate
    , notes = l.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


leadSources : List String
leadSources =
    [ "Website"
    , "Facebook"
    , "Instagram"
    , "WhatsApp"
    , "Referral"
    , "School Partner"
    , "Event"
    , "Walk-in"
    , "Other"
    ]


leadStatuses : List String
leadStatuses =
    [ "New"
    , "Contacted"
    , "Qualified"
    , "Consultation Booked"
    , "Proposal Sent"
    , "Converted"
    , "Closed/Lost"
    ]



-- ========== CASES ==========


type alias Case =
    { id : String
    , caseNumber : String
    , clientId : String
    , clientName : String
    , studentId : String
    , studentName : String
    , serviceCategory : String
    , destinationCountry : String
    , visaType : String
    , schoolOrEmployer : String
    , assignedOfficer : String
    , externalAdviser : String
    , dateOpened : String
    , targetSubmission : String
    , actualSubmission : String
    , governmentRef : String
    , currentStage : String
    , priority : String
    , nextAction : String
    , nextDeadline : String
    , result : String
    , closureDate : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias CasesData =
    { items : List Case
    , query : String
    , stageFilter : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias CaseForm =
    { caseNumber : String
    , clientId : String
    , studentId : String
    , serviceCategory : String
    , destinationCountry : String
    , visaType : String
    , schoolOrEmployer : String
    , assignedOfficer : String
    , externalAdviser : String
    , dateOpened : String
    , targetSubmission : String
    , actualSubmission : String
    , governmentRef : String
    , currentStage : String
    , priority : String
    , nextAction : String
    , nextDeadline : String
    , result : String
    , closureDate : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyCaseForm : CaseForm
emptyCaseForm =
    { caseNumber = ""
    , clientId = ""
    , studentId = ""
    , serviceCategory = ""
    , destinationCountry = ""
    , visaType = ""
    , schoolOrEmployer = ""
    , assignedOfficer = ""
    , externalAdviser = ""
    , dateOpened = ""
    , targetSubmission = ""
    , actualSubmission = ""
    , governmentRef = ""
    , currentStage = "Assessment"
    , priority = "Medium"
    , nextAction = ""
    , nextDeadline = ""
    , result = ""
    , closureDate = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


caseToForm : Case -> CaseForm
caseToForm c =
    { caseNumber = c.caseNumber
    , clientId = c.clientId
    , studentId = c.studentId
    , serviceCategory = c.serviceCategory
    , destinationCountry = c.destinationCountry
    , visaType = c.visaType
    , schoolOrEmployer = c.schoolOrEmployer
    , assignedOfficer = c.assignedOfficer
    , externalAdviser = c.externalAdviser
    , dateOpened = c.dateOpened
    , targetSubmission = c.targetSubmission
    , actualSubmission = c.actualSubmission
    , governmentRef = c.governmentRef
    , currentStage = c.currentStage
    , priority =
        if c.priority == "Normal" then
            "Medium"
        else
            c.priority
    , nextAction = c.nextAction
    , nextDeadline = c.nextDeadline
    , result = c.result
    , closureDate = c.closureDate
    , notes = c.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


caseStages : List String
caseStages =
    [ "Assessment"
    , "Eligibility Review"
    , "Agreement Pending"
    , "Documents Pending"
    , "Documents Under Review"
    , "Application Preparation"
    , "Partner Review"
    , "Ready for Submission"
    , "Submitted"
    , "Additional Documents Requested"
    , "Decision Pending"
    , "Approved"
    , "Refused"
    , "Closed"
    ]


casePriorities : List String
casePriorities =
    [ "Low", "Medium", "High", "Urgent" ]



-- ========== DOCUMENTS ==========


type alias Document =
    { id : String
    , caseId : String
    , caseNumber : String
    , docName : String
    , required : Bool
    , dateRequested : String
    , dateReceived : String
    , expiryDate : String
    , verifiedBy : String
    , verificationDate : String
    , status : String
    , rejectionReason : String
    , latestVersion : Int
    , translationRequired : Bool
    , legalizationRequired : Bool
    , filePath : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias DocumentForm =
    { docName : String
    , required : Bool
    , dateRequested : String
    , dateReceived : String
    , expiryDate : String
    , verifiedBy : String
    , verificationDate : String
    , status : String
    , rejectionReason : String
    , translationRequired : Bool
    , legalizationRequired : Bool
    , filePath : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyDocumentForm : DocumentForm
emptyDocumentForm =
    { docName = ""
    , required = True
    , dateRequested = ""
    , dateReceived = ""
    , expiryDate = ""
    , verifiedBy = ""
    , verificationDate = ""
    , status = "Not Requested"
    , rejectionReason = ""
    , translationRequired = False
    , legalizationRequired = False
    , filePath = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


documentToForm : Document -> DocumentForm
documentToForm d =
    { docName = d.docName
    , required = d.required
    , dateRequested = d.dateRequested
    , dateReceived = d.dateReceived
    , expiryDate = d.expiryDate
    , verifiedBy = d.verifiedBy
    , verificationDate = d.verificationDate
    , status = d.status
    , rejectionReason = d.rejectionReason
    , translationRequired = d.translationRequired
    , legalizationRequired = d.legalizationRequired
    , filePath = d.filePath
    , notes = d.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


documentStatuses : List String
documentStatuses =
    [ "Not Requested"
    , "Requested"
    , "Received"
    , "Under Review"
    , "Correction Required"
    , "Verified"
    , "Expired"
    ]



-- ========== INVOICES ==========


type alias Invoice =
    { currency : String
    , id : String
    , invoiceNumber : String
    , clientId : String
    , clientName : String
    , caseId : String
    , caseNumber : String
    , totalFee : Float
    , governmentFee : Float
    , schoolPartnerFee : Float
    , amountReceived : Float
    , balance : Float
    , paymentMilestone : String
    , paymentMethod : String
    , officialReceiptNumber : String
    , refundStatus : String
    , referralCommission : Float
    , partnerPayable : Float
    , paymentApproval : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias InvoicesData =
    { items : List Invoice
    , query : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias InvoiceForm =
    { invoiceNumber : String
    , clientId : String
    , caseId : String
    , totalFee : String
    , governmentFee : String
    , schoolPartnerFee : String
    , amountReceived : String
    , paymentMilestone : String
    , paymentMethod : String
    , officialReceiptNumber : String
    , refundStatus : String
    , referralCommission : String
    , partnerPayable : String
    , paymentApproval : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyInvoiceForm : InvoiceForm
emptyInvoiceForm =
    { invoiceNumber = ""
    , clientId = ""
    , caseId = ""
    , totalFee = "0"
    , governmentFee = "0"
    , schoolPartnerFee = "0"
    , amountReceived = "0"
    , paymentMilestone = "Quotation Issued"
    , paymentMethod = ""
    , officialReceiptNumber = ""
    , refundStatus = ""
    , referralCommission = "0"
    , partnerPayable = "0"
    , paymentApproval = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


invoiceToForm : Invoice -> InvoiceForm
invoiceToForm inv =
    { invoiceNumber = inv.invoiceNumber
    , clientId = inv.clientId
    , caseId = inv.caseId
    , totalFee = String.fromFloat inv.totalFee
    , governmentFee = String.fromFloat inv.governmentFee
    , schoolPartnerFee = String.fromFloat inv.schoolPartnerFee
    , amountReceived = String.fromFloat inv.amountReceived
    , paymentMilestone = inv.paymentMilestone
    , paymentMethod = inv.paymentMethod
    , officialReceiptNumber = inv.officialReceiptNumber
    , refundStatus = inv.refundStatus
    , referralCommission = String.fromFloat inv.referralCommission
    , partnerPayable = String.fromFloat inv.partnerPayable
    , paymentApproval = inv.paymentApproval
    , notes = inv.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


paymentMilestones : List String
paymentMilestones =
    [ "Quotation Issued"
    , "Deposit Due"
    , "Deposit Paid"
    , "Instalment Due"
    , "Fully Paid"
    , "Refund Review"
    , "Refunded"
    ]



-- ========== PAYMENTS ==========


type alias Payment =
    { currency : String
    , id : String
    , invoiceId : String
    , amount : Float
    , paidOn : String
    , method : String
    , reference : String
    , notes : String
    , createdBy : String
    , createdAt : String
    , status : String
    , reviewedBy : String
    }


type alias PaymentForm =
    { requireApproval : Bool
    , confirmDuplicate : Bool
    , amount : String
    , paidOn : String
    , method : String
    , reference : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    }


emptyPaymentForm : PaymentForm
emptyPaymentForm =
    { requireApproval = False
    , confirmDuplicate = False
    , amount = ""
    , paidOn = ""
    , method = ""
    , reference = ""
    , notes = ""
    , errors = []
    , submitting = False
    }



-- ========== PARTNERS ==========


type alias Partner =
    { id : String
    , type_ : String
    , legalCompanyName : String
    , country : String
    , licenseNumber : String
    , licenseExpiry : String
    , verificationSource : String
    , contactPerson : String
    , contactEmail : String
    , contactPhone : String
    , agreementStart : String
    , agreementExpiry : String
    , servicesPermitted : String
    , commissionStructure : String
    , paymentTerms : String
    , casesReferred : Int
    , casesConverted : Int
    , amountPayable : Float
    , complianceNotes : String
    , notes : String
    , createdBy : String
    , createdAt : String
    }


type alias PartnersData =
    { items : List Partner
    , query : String
    , typeFilter : String
    , countryFilter : String
    , total : Int
    , offset : Int
    , limit : Int
    }


type alias PartnerForm =
    { type_ : String
    , legalCompanyName : String
    , country : String
    , licenseNumber : String
    , licenseExpiry : String
    , verificationSource : String
    , contactPerson : String
    , contactEmail : String
    , contactPhone : String
    , agreementStart : String
    , agreementExpiry : String
    , servicesPermitted : String
    , commissionStructure : String
    , paymentTerms : String
    , casesReferred : String
    , casesConverted : String
    , amountPayable : String
    , complianceNotes : String
    , notes : String
    , errors : List ( String, String )
    , submitting : Bool
    , dirty : Bool
    , confirmDiscard : Bool
    }


emptyPartnerForm : PartnerForm
emptyPartnerForm =
    { type_ = "School"
    , legalCompanyName = ""
    , country = ""
    , licenseNumber = ""
    , licenseExpiry = ""
    , verificationSource = ""
    , contactPerson = ""
    , contactEmail = ""
    , contactPhone = ""
    , agreementStart = ""
    , agreementExpiry = ""
    , servicesPermitted = ""
    , commissionStructure = ""
    , paymentTerms = ""
    , casesReferred = "0"
    , casesConverted = "0"
    , amountPayable = "0"
    , complianceNotes = ""
    , notes = ""
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


partnerToForm : Partner -> PartnerForm
partnerToForm p =
    { type_ = p.type_
    , legalCompanyName = p.legalCompanyName
    , country = p.country
    , licenseNumber = p.licenseNumber
    , licenseExpiry = p.licenseExpiry
    , verificationSource = p.verificationSource
    , contactPerson = p.contactPerson
    , contactEmail = p.contactEmail
    , contactPhone = p.contactPhone
    , agreementStart = p.agreementStart
    , agreementExpiry = p.agreementExpiry
    , servicesPermitted = p.servicesPermitted
    , commissionStructure = p.commissionStructure
    , paymentTerms = p.paymentTerms
    , casesReferred = String.fromInt p.casesReferred
    , casesConverted = String.fromInt p.casesConverted
    , amountPayable = String.fromFloat p.amountPayable
    , complianceNotes = p.complianceNotes
    , notes = p.notes
    , errors = []
    , submitting = False
    , dirty = False
    , confirmDiscard = False
    }


partnerTypes : List String
partnerTypes =
    [ "School"
    , "Immigration Lawyer"
    , "Licensed Consultant"
    , "Recruitment Agency"
    , "Employer"
    , "Travel Agency"
    , "Referral Agent"
    , "Language Provider"
    ]



-- ========== DOSSIER ==========


type alias Dossier =
    { student : Student
    , cases : List Case
    , documents : List Document
    , invoices : List Invoice
    }



-- ========== MODEL ==========


type alias SearchResult =
    { entity : String, id : String, title : String, subtitle : String }


type alias SearchData =
    { results : List SearchResult, truncated : Bool }


type alias AuditEvent =
    { id : Int, actor : String, action : String, entity : String, recordId : String, label : String, occurredAt : String }


type alias AuditData =
    { events : List AuditEvent, total : Int }


type alias ReportSummary =
    { pipelineLabel : String
    , wonLabel : String
    , balanceLabel : String
    , contacts : Int
    , students : Int
    , leads : Int
    , activeCases : Int
    , openTasks : Int
    , pipelineValue : Float
    , wonValue : Float
    , outstandingBalance : Float
    , stageCounts : Dict String Int
    , stageValues : Dict String Float
    , createdCounts : Dict String Int
    , newLeads : Int
    , approvedStudents : Int
    , totalCases : Int
    }


type Theme
    = LightTheme
    | DarkTheme


type alias Model =
    { mode : Mode
    , theme : Theme
    , reports : RemoteData ReportSummary
    , globalQuery : String
    , globalResults : RemoteData SearchData
    , audit : RemoteData AuditData
    , auditOffset : Int
    , exportEntity : String
    , exporting : Maybe String
    , form : Form
    , errors : List ( Field, String )
    , alert : Maybe Alert
    , submitting : Bool
    , token : Maybe String
    , user : Maybe User
    , showPassword : Bool
    , bootstrapping : Bool
    , route : Route
    , contacts : RemoteData ContactsData
    , viewingContact : Maybe Contact
    , contactForm : Maybe ContactForm
    , editingId : Maybe String
    , deletingContact : Maybe Contact
    , toast : Maybe String
    , deals : RemoteData DealsData
    , viewingDeal : Maybe Deal
    , dealForm : Maybe DealForm
    , editingDealId : Maybe String
    , deletingDeal : Maybe Deal
    , movingDealId : Maybe String
    , draggingDealId : Maybe String
    , dropTargetStage : Maybe String
    , selectedDeals : Set String
    , ownerFilter : String
    , dateFromFilter : String
    , dateToFilter : String
    , unreadNotifications : Int
    , today : String
    , bulkMoveStage : Maybe String
    , bulkDeleteConfirm : Bool
    , activities : RemoteData (List Activity)
    , activityForm : Maybe ActivityForm
    , deletingActivity : Maybe Activity
    , taskOffset : Int
    , tasks : RemoteData TasksData
    , taskForm : Maybe TaskForm
    , editingTaskId : Maybe String
    , deletingTask : Maybe Task
    , pendingContactsQuery : Maybe String
    , pendingDealsQuery : Maybe String
    , pendingTasksQuery : Maybe String
    , profileForm : ProfileForm
    , passwordForm : PasswordForm
    , logoutAllConfirm : Bool
    , schools : RemoteData SchoolsData
    , viewingSchool : Maybe School
    , schoolForm : Maybe SchoolForm
    , editingSchoolId : Maybe String
    , deletingSchool : Maybe School
    , pendingSchoolsQuery : Maybe String
    , students : RemoteData StudentsData
    , linkedStudents : RemoteData LinkedStudentsData
    , viewingStudent : Maybe Student
    , studentDossier : RemoteData Dossier
    , studentForm : Maybe StudentForm
    , editingStudentId : Maybe String
    , deletingStudent : Maybe Student
    , pendingStudentsQuery : Maybe String
    , agents : RemoteData AgentsData
    , viewingAgent : Maybe Agent
    , agentForm : Maybe AgentForm
    , editingAgentId : Maybe String
    , deletingAgent : Maybe Agent
    , pendingAgentsQuery : Maybe String
    , leads : RemoteData LeadsData
    , viewingLead : Maybe Lead
    , leadForm : Maybe LeadForm
    , editingLeadId : Maybe String
    , deletingLead : Maybe Lead
    , pendingLeadsQuery : Maybe String
    , cases : RemoteData CasesData
    , viewingCase : Maybe Case
    , caseForm : Maybe CaseForm
    , editingCaseId : Maybe String
    , deletingCase : Maybe Case
    , pendingCasesQuery : Maybe String
    , caseDocuments : RemoteData (List Document)
    , documentForm : Maybe DocumentForm
    , editingDocumentId : Maybe String
    , deletingDocument : Maybe Document
    , invoices : RemoteData InvoicesData
    , viewingInvoice : Maybe Invoice
    , invoiceForm : Maybe InvoiceForm
    , editingInvoiceId : Maybe String
    , deletingInvoice : Maybe Invoice
    , pendingInvoicesQuery : Maybe String
    , invoicePayments : RemoteData (List Payment)
    , paymentForm : Maybe PaymentForm
    , deletingPayment : Maybe Payment
    , refundConfirmInvoice : Maybe Invoice
    , partners : RemoteData PartnersData
    , viewingPartner : Maybe Partner
    , partnerForm : Maybe PartnerForm
    , editingPartnerId : Maybe String
    , deletingPartner : Maybe Partner
    , pendingPartnersQuery : Maybe String
    , sidebarOpen : Bool
    , navKey : Nav.Key
    }


emptyForm : Form
emptyForm =
    { name = ""
    , email = ""
    , password = ""
    , remember = True
    , otp = ""
    }



-- ========== MSG ==========


type Msg
    = UpdatedField Field String
    | ToggledTheme
    | ToggledRemember Bool
    | ToggledShowPassword
    | Submitted
    | SwitchedMode Mode
    | UpdatedGlobalQuery String
    | GlobalSearchReady String
    | GotGlobalSearch String (Result String SearchData)
    | ClosedGlobalSearch
    | RequestedAuditPage Int
    | GotAudit Int (Result String AuditData)
    | SelectedExportEntity String
    | RequestedExport
    | GotExport String (Result String String)
    | GotReports (Result String ReportSummary)
    | GotAuth (Result String AuthResponse)
    | PollWorkspaceClock
    | RefreshedWorkspaceClock (Result String ( String, Int ))
    | RefreshSession
    | RefreshedSession (Result String AuthResponse)
    | UpdatedOTP String
    | GotUser (Result String User)
    | LoggedOut
    | NavigatedTo Route
    | GotContacts (Result String ( List Contact, Int ))
    | UpdatedContactsQuery String
    | OpenedContactDetail Contact
    | OpenedAddContact
    | OpenedEditContact Contact
    | RequestedCloseContactForm
    | ConfirmedCloseContactForm
    | CancelledCloseContactForm
    | UpdatedContactFormField String String
    | AddedContactTag
    | RemovedContactTag String
    | SubmittedContactForm
    | GotSavedContact (Result ApiError Contact)
    | RequestedDeleteContact Contact
    | CancelledDeleteContact
    | ConfirmedDeleteContact
    | GotDeletedContact (Result String String)
    | GotDeals (Result String ( List Deal, Int ))
    | OpenedAddDeal
    | OpenedAddDealForContact Contact
    | OpenedAddDealWithStage String
    | OpenedEditDeal Deal
    | OpenedDealDetail Deal
    | RequestedCloseDealForm
    | ConfirmedCloseDealForm
    | CancelledCloseDealForm
    | UpdatedDealFormField String String
    | SubmittedDealForm
    | GotSavedDeal (Result ApiError Deal)
    | UpdatedDealsQuery String
    | MovedDeal Deal String
    | GotMovedDeal (Result ApiError Deal)
    | RequestedDeleteDeal Deal
    | CancelledDeleteDeal
    | ConfirmedDeleteDeal
    | GotDeletedDeal (Result String String)
    | DraggingDealStarted String
    | DraggingDealEnded
    | DropTargetEntered String
    | DropTargetLeft
    | DealDroppedOnStage String
    | ToggledDealSelection String
    | ToggledSelectAllDeals
    | ClearedDealSelection
    | UpdatedOwnerFilter String
    | UpdatedDateFromFilter String
    | UpdatedDateToFilter String
    | ClearedDealFilters
    | RequestedBulkMove String
    | CancelledBulkMove
    | ConfirmedBulkMove
    | RequestedBulkDelete
    | CancelledBulkDelete
    | ConfirmedBulkDelete
    | GotBulkMoved (List String) (Result ApiError Deal)
    | GotBulkDeleted (List String) (Result String String)
    | GotActivities (Result String (List Activity))
    | OpenedActivityForm
    | RequestedCloseActivityForm
    | ConfirmedCloseActivityForm
    | CancelledCloseActivityForm
    | UpdatedActivityFormField String String
    | SubmittedActivityForm
    | GotSavedActivity (Result ApiError Activity)
    | RequestedDeleteActivity Activity
    | CancelledDeleteActivity
    | ConfirmedDeleteActivity
    | GotDeletedActivity (Result String String)
    | UpdatedProfileField String String
    | SubmittedProfile
    | GotUpdatedProfile (Result ApiError ProfileUpdateResponse)
    | UpdatedPasswordField String String
    | SubmittedPassword
    | GotChangedPassword (Result String String)
    | RequestedLogoutAll
    | CancelledLogoutAll
    | ConfirmedLogoutAll
    | GotLogoutAll (Result String String)
    | ChangedTasksPage Int
    | GotTasks (Result String ( List Task, Int ))
    | UpdatedTasksQuery String
    | UpdatedTasksStatusFilter String
    | OpenedAddTask
    | OpenedAddTaskForContact Contact
    | OpenedEditTask Task
    | RequestedCloseTaskForm
    | ConfirmedCloseTaskForm
    | CancelledCloseTaskForm
    | UpdatedTaskFormField String String
    | SubmittedTaskForm
    | GotSavedTask (Result ApiError Task)
    | ToggledTaskStatus Task String
    | GotToggledTask (Result ApiError Task)
    | RequestedDeleteTask Task
    | CancelledDeleteTask
    | ConfirmedDeleteTask
    | GotDeletedTask (Result String String)
    | ContactsPageChanged Int
    | FlushContactsSearch
    | FlushDealsSearch
    | FlushTasksSearch
    | GotSchools (Result String ( List School, Int ))
    | UpdatedSchoolsQuery String
    | OpenedAddSchool
    | OpenedEditSchool School
    | OpenedSchoolDetail School
    | RequestedCloseSchoolForm
    | ConfirmedCloseSchoolForm
    | CancelledCloseSchoolForm
    | UpdatedSchoolFormField String String
    | SubmittedSchoolForm
    | GotSavedSchool (Result ApiError School)
    | RequestedDeleteSchool School
    | CancelledDeleteSchool
    | ConfirmedDeleteSchool
    | GotDeletedSchool (Result String String)
    | SchoolsPageChanged Int
    | FlushSchoolsSearch
    | GotStudents (Result String ( List Student, Int ))
    | GotLinkedStudents String (Result String ( List Student, Int ))
    | UpdatedStudentsQuery String
    | OpenedAddStudent
    | OpenedAddStudentForAgent Agent
    | OpenedAddStudentForSchool School
    | OpenedEditStudent Student
    | OpenedStudentDetail Student
    | RequestedCloseStudentForm
    | ConfirmedCloseStudentForm
    | CancelledCloseStudentForm
    | UpdatedStudentFormField String String
    | SubmittedStudentForm
    | GotSavedStudent (Result ApiError Student)
    | RequestedDeleteStudent Student
    | CancelledDeleteStudent
    | ConfirmedDeleteStudent
    | GotDeletedStudent (Result String String)
    | StudentsPageChanged Int
    | FlushStudentsSearch
    | GotAgents (Result String ( List Agent, Int ))
    | UpdatedAgentsQuery String
    | OpenedAddAgent
    | OpenedEditAgent Agent
    | OpenedAgentDetail Agent
    | RequestedCloseAgentForm
    | ConfirmedCloseAgentForm
    | CancelledCloseAgentForm
    | UpdatedAgentFormField String String
    | SubmittedAgentForm
    | GotSavedAgent (Result ApiError Agent)
    | RequestedDeleteAgent Agent
    | CancelledDeleteAgent
    | ConfirmedDeleteAgent
    | GotDeletedAgent (Result String String)
    | AgentsPageChanged Int
    | FlushAgentsSearch
    | GotLeads (Result String ( List Lead, Int ))
    | UpdatedLeadsQuery String
    | OpenedAddLead
    | OpenedEditLead Lead
    | OpenedLeadDetail Lead
    | RequestedCloseLeadForm
    | ConfirmedCloseLeadForm
    | CancelledCloseLeadForm
    | UpdatedLeadFormField String String
    | SubmittedLeadForm
    | GotSavedLead (Result ApiError Lead)
    | RequestedDeleteLead Lead
    | CancelledDeleteLead
    | ConfirmedDeleteLead
    | GotDeletedLead (Result String String)
    | LeadsPageChanged Int
    | FlushLeadsSearch
    | GotCases (Result String ( List Case, Int ))
    | UpdatedCasesQuery String
    | UpdatedCasesStageFilter String
    | OpenedAddCase
    | OpenedAddCaseForStudent Student
    | OpenedEditCase Case
    | OpenedCaseDetail Case
    | RequestedCloseCaseForm
    | ConfirmedCloseCaseForm
    | CancelledCloseCaseForm
    | UpdatedCaseFormField String String
    | SubmittedCaseForm
    | GotSavedCase (Result ApiError Case)
    | RequestedDeleteCase Case
    | CancelledDeleteCase
    | ConfirmedDeleteCase
    | GotDeletedCase (Result String String)
    | CasesPageChanged Int
    | FlushCasesSearch
    | GotCaseDocuments (Result String (List Document))
    | OpenedAddDocument
    | OpenedEditDocument Document
    | RequestedCloseDocumentForm
    | ConfirmedCloseDocumentForm
    | CancelledCloseDocumentForm
    | UpdatedDocumentFormField String String
    | ToggledDocumentBool String Bool
    | SubmittedDocumentForm
    | GotSavedDocument (Result ApiError Document)
    | RequestedDeleteDocument Document
    | CancelledDeleteDocument
    | ConfirmedDeleteDocument
    | GotDeletedDocument (Result String String)
    | UpdatedDocumentStatus Document String
    | GotUpdatedDocument (Result ApiError Document)
    | GotInvoices (Result String ( List Invoice, Int ))
    | UpdatedInvoicesQuery String
    | OpenedAddInvoice
    | OpenedEditInvoice Invoice
    | OpenedInvoiceDetail Invoice
    | RequestedCloseInvoiceForm
    | ConfirmedCloseInvoiceForm
    | CancelledCloseInvoiceForm
    | UpdatedInvoiceFormField String String
    | SubmittedInvoiceForm
    | GotSavedInvoice (Result ApiError Invoice)
    | RequestedDeleteInvoice Invoice
    | CancelledDeleteInvoice
    | ConfirmedDeleteInvoice
    | GotDeletedInvoice (Result String String)
    | InvoicesPageChanged Int
    | FlushInvoicesSearch
    | GotInvoicePayments (Result String (List Payment))
    | OpenedPaymentForm
    | ClosedPaymentForm
    | UpdatedPaymentFormField String String
    | SubmittedPaymentForm
    | GotSavedPayment (Result ApiError Payment)
    | RequestedDeletePayment Payment
    | CancelledDeletePayment
    | ConfirmedDeletePayment
    | GotDeletedPayment (Result String String)
    | RequestedRefund Invoice
    | CancelledRefund
    | ConfirmedRefund
    | GotRefunded (Result ApiError Invoice)
    | GotPartners (Result String ( List Partner, Int ))
    | UpdatedPartnersQuery String
    | UpdatedPartnersTypeFilter String
    | UpdatedPartnersCountryFilter String
    | OpenedAddPartner
    | OpenedEditPartner Partner
    | OpenedPartnerDetail Partner
    | RequestedClosePartnerForm
    | ConfirmedClosePartnerForm
    | CancelledClosePartnerForm
    | UpdatedPartnerFormField String String
    | SubmittedPartnerForm
    | GotSavedPartner (Result ApiError Partner)
    | RequestedDeletePartner Partner
    | CancelledDeletePartner
    | ConfirmedDeletePartner
    | GotDeletedPartner (Result String String)
    | PartnersPageChanged Int
    | FlushPartnersSearch
    | FetchedContact (Result String Contact)
    | FetchedDeal (Result String Deal)
    | FetchedSchool (Result String School)
    | FetchedStudent (Result String Student)
    | GotStudentDossier (Result String Dossier)
    | FetchedAgent (Result String Agent)
    | FetchedLead (Result String Lead)
    | FetchedCase (Result String Case)
    | FetchedInvoice (Result String Invoice)
    | FetchedPartner (Result String Partner)
    | FetchFailed String
    | LinkClicked Browser.UrlRequest
    | UrlChanged Url.Url
    | DismissedToast
    | EscapePressed
    | ToggledSideBar
    | NoOp
