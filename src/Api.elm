module Api exposing (..)

import Http
import Json.Decode as D
import Json.Encode as E
import Types exposing (..)
import Url



-- DECODERS ----------------------------------------------------------------
-- ========== CASES ==========


caseDecoder : D.Decoder Case
caseDecoder =
    D.succeed Case
        |> andMap (D.field "id" D.string)
        |> andMap (optString "caseNumber")
        |> andMap (optString "clientId")
        |> andMap (optString "clientName")
        |> andMap (optString "studentId")
        |> andMap (optString "studentName")
        |> andMap (optString "serviceCategory")
        |> andMap (optString "destinationCountry")
        |> andMap (optString "visaType")
        |> andMap (optString "schoolOrEmployer")
        |> andMap (optString "assignedOfficer")
        |> andMap (optString "externalAdviser")
        |> andMap (optString "dateOpened")
        |> andMap (optString "targetSubmission")
        |> andMap (optString "actualSubmission")
        |> andMap (optString "governmentRef")
        |> andMap (D.oneOf [ D.field "currentStage" D.string, D.succeed "Assessment" ])
        |> andMap (D.oneOf [ D.field "priority" D.string, D.succeed "Medium" ])
        |> andMap (optString "nextAction")
        |> andMap (optString "nextDeadline")
        |> andMap (optString "result")
        |> andMap (optString "closureDate")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


documentDecoder : D.Decoder Document
documentDecoder =
    D.succeed Document
        |> andMap (D.field "id" D.string)
        |> andMap (optString "caseId")
        |> andMap (optString "caseNumber")
        |> andMap (D.field "docName" D.string)
        |> andMap (D.oneOf [ D.field "required" D.bool, D.succeed False ])
        |> andMap (optString "dateRequested")
        |> andMap (optString "dateReceived")
        |> andMap (optString "expiryDate")
        |> andMap (optString "verifiedBy")
        |> andMap (optString "verificationDate")
        |> andMap (D.oneOf [ D.field "status" D.string, D.succeed "Not Requested" ])
        |> andMap (optString "rejectionReason")
        |> andMap (D.oneOf [ D.field "latestVersion" D.int, D.succeed 1 ])
        |> andMap (D.oneOf [ D.field "translationRequired" D.bool, D.succeed False ])
        |> andMap (D.oneOf [ D.field "legalizationRequired" D.bool, D.succeed False ])
        |> andMap (optString "filePath")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


documentsDecoder : D.Decoder (List Document)
documentsDecoder =
    D.field "documents" (D.list documentDecoder)


invoiceDecoder : D.Decoder Invoice
invoiceDecoder =
    D.succeed Invoice
        |> andMap (optString "currency")
        |> andMap (D.field "id" D.string)
        |> andMap (optString "invoiceNumber")
        |> andMap (optString "clientId")
        |> andMap (optString "clientName")
        |> andMap (optString "caseId")
        |> andMap (optString "caseNumber")
        |> andMap (D.oneOf [ D.field "totalFee" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "governmentFee" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "schoolPartnerFee" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "amountReceived" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "balance" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "paymentMilestone" D.string, D.succeed "Quotation Issued" ])
        |> andMap (optString "paymentMethod")
        |> andMap (optString "officialReceiptNumber")
        |> andMap (optString "refundStatus")
        |> andMap (D.oneOf [ D.field "referralCommission" D.float, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "partnerPayable" D.float, D.succeed 0 ])
        |> andMap (optString "paymentApproval")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


paymentDecoder : D.Decoder Payment
paymentDecoder =
    D.succeed Payment
        |> andMap (optString "currency")
        |> andMap (D.field "id" D.string)
        |> andMap (optString "invoiceId")
        |> andMap (D.oneOf [ D.field "amount" D.float, D.succeed 0 ])
        |> andMap (optString "paidOn")
        |> andMap (optString "method")
        |> andMap (optString "reference")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")
        |> andMap (D.oneOf [ D.field "status" D.string, D.succeed "posted" ])
        |> andMap (optString "reviewedBy")


paymentsDecoder : D.Decoder (List Payment)
paymentsDecoder =
    D.field "payments" (D.list paymentDecoder)


partnerDecoder : D.Decoder Partner
partnerDecoder =
    D.succeed Partner
        |> andMap (D.field "id" D.string)
        |> andMap (D.oneOf [ D.field "type" D.string, D.succeed "School" ])
        |> andMap (D.field "legalCompanyName" D.string)
        |> andMap (optString "country")
        |> andMap (optString "licenseNumber")
        |> andMap (optString "licenseExpiry")
        |> andMap (optString "verificationSource")
        |> andMap (optString "contactPerson")
        |> andMap (optString "contactEmail")
        |> andMap (optString "contactPhone")
        |> andMap (optString "agreementStart")
        |> andMap (optString "agreementExpiry")
        |> andMap (optString "servicesPermitted")
        |> andMap (optString "commissionStructure")
        |> andMap (optString "paymentTerms")
        |> andMap (D.oneOf [ D.field "casesReferred" D.int, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "casesConverted" D.int, D.succeed 0 ])
        |> andMap (D.oneOf [ D.field "amountPayable" D.float, D.succeed 0 ])
        |> andMap (optString "complianceNotes")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


caseFormPayload : CaseForm -> E.Value
caseFormPayload cf =
    E.object
        [ ( "caseNumber", E.string cf.caseNumber )
        , ( "clientId", E.string cf.clientId )
        , ( "studentId", E.string cf.studentId )
        , ( "serviceCategory", E.string cf.serviceCategory )
        , ( "destinationCountry", E.string cf.destinationCountry )
        , ( "visaType", E.string cf.visaType )
        , ( "schoolOrEmployer", E.string cf.schoolOrEmployer )
        , ( "assignedOfficer", E.string cf.assignedOfficer )
        , ( "externalAdviser", E.string cf.externalAdviser )
        , ( "dateOpened", E.string cf.dateOpened )
        , ( "targetSubmission", E.string cf.targetSubmission )
        , ( "actualSubmission", E.string cf.actualSubmission )
        , ( "governmentRef", E.string cf.governmentRef )
        , ( "currentStage", E.string cf.currentStage )
        , ( "priority", E.string cf.priority )
        , ( "nextAction", E.string cf.nextAction )
        , ( "nextDeadline", E.string cf.nextDeadline )
        , ( "result", E.string cf.result )
        , ( "closureDate", E.string cf.closureDate )
        , ( "notes", E.string cf.notes )
        ]


invoiceFormPayload : InvoiceForm -> E.Value
invoiceFormPayload inv =
    E.object
        [ ( "invoiceNumber", E.string inv.invoiceNumber )
        , ( "clientId", E.string inv.clientId )
        , ( "caseId", E.string inv.caseId )
        , ( "totalFee", E.float (Maybe.withDefault 0 (String.toFloat inv.totalFee)) )
        , ( "governmentFee", E.float (Maybe.withDefault 0 (String.toFloat inv.governmentFee)) )
        , ( "schoolPartnerFee", E.float (Maybe.withDefault 0 (String.toFloat inv.schoolPartnerFee)) )
        , ( "amountReceived", E.float (Maybe.withDefault 0 (String.toFloat inv.amountReceived)) )
        , ( "paymentMilestone", E.string inv.paymentMilestone )
        , ( "paymentMethod", E.string inv.paymentMethod )
        , ( "officialReceiptNumber", E.string inv.officialReceiptNumber )
        , ( "refundStatus", E.string inv.refundStatus )
        , ( "referralCommission", E.float (Maybe.withDefault 0 (String.toFloat inv.referralCommission)) )
        , ( "partnerPayable", E.float (Maybe.withDefault 0 (String.toFloat inv.partnerPayable)) )
        , ( "paymentApproval", E.string inv.paymentApproval )
        , ( "notes", E.string inv.notes )
        ]


paymentFormPayload : String -> PaymentForm -> E.Value
paymentFormPayload invoiceId pf =
    E.object
        [ ( "requireApproval", E.bool pf.requireApproval )
        , ( "confirmDuplicate", E.bool pf.confirmDuplicate )
        , ( "invoiceId", E.string invoiceId )
        , ( "amount", E.float (Maybe.withDefault 0 (String.toFloat pf.amount)) )
        , ( "paidOn", E.string pf.paidOn )
        , ( "method", E.string pf.method )
        , ( "reference", E.string pf.reference )
        , ( "notes", E.string pf.notes )
        ]


partnerFormPayload : PartnerForm -> E.Value
partnerFormPayload pf =
    E.object
        [ ( "type", E.string pf.type_ )
        , ( "legalCompanyName", E.string pf.legalCompanyName )
        , ( "country", E.string pf.country )
        , ( "licenseNumber", E.string pf.licenseNumber )
        , ( "licenseExpiry", E.string pf.licenseExpiry )
        , ( "verificationSource", E.string pf.verificationSource )
        , ( "contactPerson", E.string pf.contactPerson )
        , ( "contactEmail", E.string pf.contactEmail )
        , ( "contactPhone", E.string pf.contactPhone )
        , ( "agreementStart", E.string pf.agreementStart )
        , ( "agreementExpiry", E.string pf.agreementExpiry )
        , ( "servicesPermitted", E.string pf.servicesPermitted )
        , ( "commissionStructure", E.string pf.commissionStructure )
        , ( "paymentTerms", E.string pf.paymentTerms )
        , ( "casesReferred", E.int (Maybe.withDefault 0 (String.toInt pf.casesReferred)) )
        , ( "casesConverted", E.int (Maybe.withDefault 0 (String.toInt pf.casesConverted)) )
        , ( "amountPayable", E.float (Maybe.withDefault 0 (String.toFloat pf.amountPayable)) )
        , ( "complianceNotes", E.string pf.complianceNotes )
        , ( "notes", E.string pf.notes )
        ]


casesPageExpect : (Result String ( List Case, Int ) -> msg) -> Http.Expect msg
casesPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load cases (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "cases" (D.list caseDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse cases: " ++ D.errorToString err)


documentsExpect : (Result String (List Document) -> msg) -> Http.Expect msg
documentsExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load documents."

                Http.GoodStatus_ _ body ->
                    case D.decodeString documentsDecoder body of
                        Ok docs ->
                            Ok docs

                        Err err ->
                            Err ("Could not parse documents: " ++ D.errorToString err)


dossierDecoder : D.Decoder Dossier
dossierDecoder =
    D.map4 Dossier
        (D.field "student" studentDecoder)
        (D.field "cases" (D.list caseDecoder))
        (D.field "documents" (D.list documentDecoder))
        (D.field "invoices" (D.list invoiceDecoder))


dossierExpect : (Result String Dossier -> msg) -> Http.Expect msg
dossierExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load dossier."

                Http.GoodStatus_ _ body ->
                    case D.decodeString dossierDecoder body of
                        Ok d ->
                            Ok d

                        Err err ->
                            Err ("Could not parse dossier: " ++ D.errorToString err)


invoicesPageExpect : (Result String ( List Invoice, Int ) -> msg) -> Http.Expect msg
invoicesPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load invoices (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "invoices" (D.list invoiceDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse invoices: " ++ D.errorToString err)


paymentsExpect : (Result String (List Payment) -> msg) -> Http.Expect msg
paymentsExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load payments."

                Http.GoodStatus_ _ body ->
                    case D.decodeString paymentsDecoder body of
                        Ok ps ->
                            Ok ps

                        Err err ->
                            Err ("Could not parse payments: " ++ D.errorToString err)


partnersPageExpect : (Result String ( List Partner, Int ) -> msg) -> Http.Expect msg
partnersPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load partners (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "partners" (D.list partnerDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse partners: " ++ D.errorToString err)


savedCaseExpect : (Result ApiError Case -> msg) -> Http.Expect msg
savedCaseExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save case.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "case" caseDecoder) body of
                        Ok c ->
                            Ok c

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedCaseExpect : (Result String String -> msg) -> Http.Expect msg
deletedCaseExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete case."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


caseFetchExpect : (Result String Case -> msg) -> Http.Expect msg
caseFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load case."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "case" caseDecoder) body of
                        Ok c ->
                            Ok c

                        Err err ->
                            Err ("Could not parse case: " ++ D.errorToString err)


savedDocumentExpect : (Result ApiError Document -> msg) -> Http.Expect msg
savedDocumentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save document.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "document" documentDecoder) body of
                        Ok d ->
                            Ok d

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedDocumentExpect : (Result String String -> msg) -> Http.Expect msg
deletedDocumentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete document."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedInvoiceExpect : (Result ApiError Invoice -> msg) -> Http.Expect msg
savedInvoiceExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save invoice.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "invoice" invoiceDecoder) body of
                        Ok inv ->
                            Ok inv

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedInvoiceExpect : (Result String String -> msg) -> Http.Expect msg
deletedInvoiceExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete invoice."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


invoiceFetchExpect : (Result String Invoice -> msg) -> Http.Expect msg
invoiceFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load invoice."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "invoice" invoiceDecoder) body of
                        Ok inv ->
                            Ok inv

                        Err err ->
                            Err ("Could not parse invoice: " ++ D.errorToString err)


savedPaymentExpect : (Result ApiError Payment -> msg) -> Http.Expect msg
savedPaymentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save payment.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "payment" paymentDecoder) body of
                        Ok p ->
                            Ok p

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedPaymentExpect : (Result String String -> msg) -> Http.Expect msg
deletedPaymentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete payment."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedPartnerExpect : (Result ApiError Partner -> msg) -> Http.Expect msg
savedPartnerExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save partner.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "partner" partnerDecoder) body of
                        Ok p ->
                            Ok p

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedPartnerExpect : (Result String String -> msg) -> Http.Expect msg
deletedPartnerExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete partner."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


partnerFetchExpect : (Result String Partner -> msg) -> Http.Expect msg
partnerFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load partner."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "partner" partnerDecoder) body of
                        Ok p ->
                            Ok p

                        Err err ->
                            Err ("Could not parse partner: " ++ D.errorToString err)


fetchCases : String -> String -> String -> Int -> Int -> (Result String ( List Case, Int ) -> msg) -> Cmd msg
fetchCases token query stage limit offset toMsg =
    let
        parts =
            List.filter (\part -> not (String.isEmpty part))
                [ "q=" ++ Url.percentEncode query
                , if String.isEmpty stage then
                    ""

                  else
                    "stage=" ++ Url.percentEncode stage
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/cases?" ++ String.join "&" parts
        , body = Http.emptyBody
        , expect = casesPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchCase : String -> String -> (Result String Case -> msg) -> Cmd msg
fetchCase token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/cases/" ++ id
        , body = Http.emptyBody
        , expect = caseFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createCase : String -> CaseForm -> (Result ApiError Case -> msg) -> Cmd msg
createCase token cf toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/cases"
        , body = Http.jsonBody (caseFormPayload cf)
        , expect = savedCaseExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateCase : String -> String -> CaseForm -> (Result ApiError Case -> msg) -> Cmd msg
updateCase token id cf toMsg =
    Http.request
        { method = "PUT"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/cases/" ++ id
        , body = Http.jsonBody (caseFormPayload cf)
        , expect = savedCaseExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteCase : String -> String -> (Result String String -> msg) -> Cmd msg
deleteCase token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/cases/" ++ id
        , body = Http.emptyBody
        , expect = deletedCaseExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchCaseDocuments : String -> String -> (Result String (List Document) -> msg) -> Cmd msg
fetchCaseDocuments token caseId toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/documents?caseId=" ++ Url.percentEncode caseId ++ "&limit=200"
        , body = Http.emptyBody
        , expect = documentsExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createDocument : String -> String -> DocumentForm -> (Result ApiError Document -> msg) -> Cmd msg
createDocument token caseId df toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/documents"
        , body =
            Http.jsonBody
                (E.object
                    [ ( "caseId", E.string caseId )
                    , ( "docName", E.string df.docName )
                    , ( "required", E.bool df.required )
                    , ( "dateRequested", E.string df.dateRequested )
                    , ( "dateReceived", E.string df.dateReceived )
                    , ( "expiryDate", E.string df.expiryDate )
                    , ( "verifiedBy", E.string df.verifiedBy )
                    , ( "verificationDate", E.string df.verificationDate )
                    , ( "status", E.string df.status )
                    , ( "rejectionReason", E.string df.rejectionReason )
                    , ( "translationRequired", E.bool df.translationRequired )
                    , ( "legalizationRequired", E.bool df.legalizationRequired )
                    , ( "filePath", E.string df.filePath )
                    , ( "notes", E.string df.notes )
                    ]
                )
        , expect = savedDocumentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateDocument : String -> String -> String -> DocumentForm -> (Result ApiError Document -> msg) -> Cmd msg
updateDocument token caseId id df toMsg =
    Http.request
        { method = "PUT"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/documents/" ++ id
        , body =
            Http.jsonBody
                (E.object
                    [ ( "caseId", E.string caseId )
                    , ( "docName", E.string df.docName )
                    , ( "required", E.bool df.required )
                    , ( "dateRequested", E.string df.dateRequested )
                    , ( "dateReceived", E.string df.dateReceived )
                    , ( "expiryDate", E.string df.expiryDate )
                    , ( "verifiedBy", E.string df.verifiedBy )
                    , ( "verificationDate", E.string df.verificationDate )
                    , ( "status", E.string df.status )
                    , ( "rejectionReason", E.string df.rejectionReason )
                    , ( "translationRequired", E.bool df.translationRequired )
                    , ( "legalizationRequired", E.bool df.legalizationRequired )
                    , ( "filePath", E.string df.filePath )
                    , ( "notes", E.string df.notes )
                    ]
                )
        , expect = savedDocumentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateDocumentStatus : String -> String -> String -> (Result ApiError Document -> msg) -> Cmd msg
updateDocumentStatus token id newStatus toMsg =
    Http.request
        { method = "PATCH"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/documents/" ++ id ++ "/status"
        , body = Http.jsonBody (E.object [ ( "status", E.string newStatus ), ( "rejectionReason", E.string "" ) ])
        , expect = savedDocumentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteDocument : String -> String -> (Result String String -> msg) -> Cmd msg
deleteDocument token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/documents/" ++ id
        , body = Http.emptyBody
        , expect = deletedDocumentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchInvoices : String -> String -> Int -> Int -> (Result String ( List Invoice, Int ) -> msg) -> Cmd msg
fetchInvoices token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/invoices?" ++ qs
        , body = Http.emptyBody
        , expect = invoicesPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchInvoice : String -> String -> (Result String Invoice -> msg) -> Cmd msg
fetchInvoice token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/invoices/" ++ id
        , body = Http.emptyBody
        , expect = invoiceFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createInvoice : String -> InvoiceForm -> (Result ApiError Invoice -> msg) -> Cmd msg
createInvoice token inv toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/invoices"
        , body = Http.jsonBody (invoiceFormPayload inv)
        , expect = savedInvoiceExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateInvoice : String -> String -> InvoiceForm -> (Result ApiError Invoice -> msg) -> Cmd msg
updateInvoice token id inv toMsg =
    Http.request
        { method = "PUT"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/invoices/" ++ id
        , body = Http.jsonBody (invoiceFormPayload inv)
        , expect = savedInvoiceExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


refundInvoice : String -> String -> String -> (Result ApiError Invoice -> msg) -> Cmd msg
refundInvoice token id reason toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/invoices/" ++ id ++ "/refund"
        , body = Http.jsonBody (E.object [ ( "reason", E.string reason ) ])
        , expect = savedInvoiceExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteInvoice : String -> String -> (Result String String -> msg) -> Cmd msg
deleteInvoice token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/invoices/" ++ id
        , body = Http.emptyBody
        , expect = deletedInvoiceExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchInvoicePayments : String -> String -> (Result String (List Payment) -> msg) -> Cmd msg
fetchInvoicePayments token invoiceId toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/payments?invoiceId=" ++ Url.percentEncode invoiceId ++ "&limit=200"
        , body = Http.emptyBody
        , expect = paymentsExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createPayment : String -> String -> PaymentForm -> (Result ApiError Payment -> msg) -> Cmd msg
createPayment token invoiceId pf toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/payments"
        , body = Http.jsonBody (paymentFormPayload invoiceId pf)
        , expect = savedPaymentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deletePayment : String -> String -> (Result String String -> msg) -> Cmd msg
deletePayment token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/payments/" ++ id
        , body = Http.emptyBody
        , expect = deletedPaymentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchPartners : String -> String -> String -> String -> Int -> Int -> (Result String ( List Partner, Int ) -> msg) -> Cmd msg
fetchPartners token query ptype country limit offset toMsg =
    let
        parts =
            List.filter (\part -> not (String.isEmpty part))
                [ "q=" ++ Url.percentEncode query
                , if String.isEmpty ptype then
                    ""

                  else
                    "type=" ++ Url.percentEncode ptype
                , if String.isEmpty country then
                    ""

                  else
                    "country=" ++ Url.percentEncode country
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/partners?" ++ String.join "&" parts
        , body = Http.emptyBody
        , expect = partnersPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchPartner : String -> String -> (Result String Partner -> msg) -> Cmd msg
fetchPartner token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/partners/" ++ id
        , body = Http.emptyBody
        , expect = partnerFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createPartner : String -> PartnerForm -> (Result ApiError Partner -> msg) -> Cmd msg
createPartner token pf toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/partners"
        , body = Http.jsonBody (partnerFormPayload pf)
        , expect = savedPartnerExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updatePartner : String -> String -> PartnerForm -> (Result ApiError Partner -> msg) -> Cmd msg
updatePartner token id pf toMsg =
    Http.request
        { method = "PUT"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token), Http.header "Content-Type" "application/json" ]
        , url = "/api/partners/" ++ id
        , body = Http.jsonBody (partnerFormPayload pf)
        , expect = savedPartnerExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deletePartner : String -> String -> (Result String String -> msg) -> Cmd msg
deletePartner token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/partners/" ++ id
        , body = Http.emptyBody
        , expect = deletedPartnerExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


userDecoder : D.Decoder User
userDecoder =
    D.map4 User
        (D.oneOf [ D.field "role" D.string, D.succeed "member" ])
        (D.field "id" D.string)
        (D.field "name" D.string)
        (D.field "email" D.string)


optString : String -> D.Decoder String
optString field =
    D.oneOf
        [ D.field field D.string
        , D.succeed ""
        ]


optStringList : String -> D.Decoder (List String)
optStringList field =
    D.oneOf
        [ D.field field (D.list D.string)
        , D.succeed []
        ]


andMap : D.Decoder a -> D.Decoder (a -> b) -> D.Decoder b
andMap =
    D.map2 (|>)


type alias ContactCore =
    { id : String
    , name : String
    , email : String
    , company : String
    , title : String
    , phone : String
    , location : String
    , stage : String
    }


type alias ContactExtras =
    { lastContact : String
    , owner : String
    , tags : List String
    , notes : String
    , createdAt : String
    }


contactCoreDecoder : D.Decoder ContactCore
contactCoreDecoder =
    D.map8 ContactCore
        (D.field "id" D.string)
        (D.field "name" D.string)
        (D.field "email" D.string)
        (D.field "company" D.string)
        (optString "title")
        (optString "phone")
        (optString "location")
        (D.oneOf [ D.field "stage" D.string, D.succeed "Lead" ])


contactExtrasDecoder : D.Decoder ContactExtras
contactExtrasDecoder =
    D.map5 ContactExtras
        (optString "lastContact")
        (optString "owner")
        (optStringList "tags")
        (optString "notes")
        (optString "createdAt")


contactDecoder : D.Decoder Contact
contactDecoder =
    D.map2
        (\core extras ->
            Contact
                core.id
                core.name
                core.email
                core.company
                core.title
                core.phone
                core.location
                core.stage
                extras.lastContact
                extras.owner
                extras.tags
                extras.notes
                extras.createdAt
        )
        contactCoreDecoder
        contactExtrasDecoder


contactsDecoder : D.Decoder (List Contact)
contactsDecoder =
    D.field "contacts" (D.list contactDecoder)


schoolDecoder : D.Decoder School
schoolDecoder =
    D.succeed School
        |> andMap (D.field "id" D.string)
        |> andMap (D.field "name" D.string)
        |> andMap (optString "countryCode")
        |> andMap (optString "commissionRate")
        |> andMap (D.oneOf [ D.field "contractStatus" D.string, D.succeed "Pending" ])
        |> andMap (D.oneOf [ D.field "studentsEnrolled" D.int, D.succeed 0 ])
        |> andMap (optString "contactPerson")
        |> andMap (optString "website")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


studentDecoder : D.Decoder Student
studentDecoder =
    D.succeed Student
        |> andMap (D.field "id" D.string)
        |> andMap (D.field "name" D.string)
        |> andMap (optString "studentCode")
        |> andMap (optString "countryCode")
        |> andMap (optString "schoolId")
        |> andMap (optString "schoolName")
        |> andMap (optString "agentId")
        |> andMap (optString "agentName")
        |> andMap (optString "program")
        |> andMap (D.oneOf [ D.field "acceptanceStatus" D.string, D.succeed "Pending" ])
        |> andMap (D.oneOf [ D.field "visaStatus" D.string, D.succeed "Not Started" ])
        |> andMap (D.oneOf [ D.field "invoiceStatus" D.string, D.succeed "Not Issued" ])
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


agentDecoder : D.Decoder Agent
agentDecoder =
    D.succeed Agent
        |> andMap (D.field "id" D.string)
        |> andMap (D.field "name" D.string)
        |> andMap (optString "agentCode")
        |> andMap (optString "countryCode")
        |> andMap (D.oneOf [ D.field "contractStatus" D.string, D.succeed "Pending" ])
        |> andMap (D.oneOf [ D.field "agentStatus" D.string, D.succeed "Active" ])
        |> andMap (D.oneOf [ D.field "studentsReferred" D.int, D.succeed 0 ])
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


leadDecoder : D.Decoder Lead
leadDecoder =
    D.succeed Lead
        |> andMap (D.field "id" D.string)
        |> andMap (optString "leadNumber")
        |> andMap (D.field "name" D.string)
        |> andMap (optString "email")
        |> andMap (optString "phone")
        |> andMap (optString "nationality")
        |> andMap (optString "currentCountry")
        |> andMap (optString "interestedCountry")
        |> andMap (optString "interestedService")
        |> andMap (D.oneOf [ D.field "source" D.string, D.succeed "Website" ])
        |> andMap (optString "assignedTo")
        |> andMap (D.oneOf [ D.field "status" D.string, D.succeed "New" ])
        |> andMap (optString "followUpDate")
        |> andMap (optString "notes")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


activityDecoder : D.Decoder Activity
activityDecoder =
    D.map Activity
        (D.field "id" D.string)
        |> andMap (D.field "contactId" D.string)
        |> andMap (optString "dealId")
        |> andMap (D.field "kind" D.string)
        |> andMap (D.field "title" D.string)
        |> andMap (optString "body")
        |> andMap (optString "occurredAt")
        |> andMap (optString "createdBy")
        |> andMap (optString "createdAt")


activitiesDecoder : D.Decoder (List Activity)
activitiesDecoder =
    D.field "activities" (D.list activityDecoder)


authResponseDecoder : D.Decoder AuthResponse
authResponseDecoder =
    D.map2 AuthResponse
        (D.field "token" D.string)
        (D.field "user" userDecoder)


profileUpdateDecoder : D.Decoder ProfileUpdateResponse
profileUpdateDecoder =
    D.map2 ProfileUpdateResponse
        (D.field "user" userDecoder)
        (D.field "token" D.string)


errorDecoder : D.Decoder String
errorDecoder =
    D.field "error" D.string


fieldErrorDecoder : D.Decoder (List ( String, String ))
fieldErrorDecoder =
    D.field "fields" (D.keyValuePairs D.string)



-- ENCODERS ----------------------------------------------------------------


loginPayload : String -> String -> E.Value
loginPayload email password =
    E.object
        [ ( "email", E.string email )
        , ( "password", E.string password )
        ]


registerPayload : String -> String -> String -> E.Value
registerPayload name email password =
    E.object
        [ ( "name", E.string name )
        , ( "email", E.string email )
        , ( "password", E.string password )
        ]


contactFormPayload : ContactForm -> E.Value
contactFormPayload cf =
    E.object
        [ ( "name", E.string cf.name )
        , ( "email", E.string cf.email )
        , ( "company", E.string cf.company )
        , ( "title", E.string cf.title )
        , ( "phone", E.string cf.phone )
        , ( "location", E.string cf.location )
        , ( "stage", E.string cf.stage )
        , ( "tags", E.list E.string cf.tags )
        , ( "notes", E.string cf.notes )
        ]


dealFormPayload : DealForm -> E.Value
dealFormPayload df =
    E.object
        [ ( "title", E.string df.title )
        , ( "contactId", E.string df.contactId )
        , ( "value", E.float (Maybe.withDefault 0 (String.toFloat df.value)) )
        , ( "currency", E.string df.currency )
        , ( "stage", E.string df.stage )
        , ( "closeDate", E.string df.closeDate )
        , ( "owner", E.string df.owner )
        , ( "notes", E.string df.notes )
        ]


schoolFormPayload : SchoolForm -> E.Value
schoolFormPayload sf =
    E.object
        [ ( "name", E.string sf.name )
        , ( "countryCode", E.string sf.countryCode )
        , ( "commissionRate", E.string sf.commissionRate )
        , ( "contractStatus", E.string sf.contractStatus )
        , ( "contactPerson", E.string sf.contactPerson )
        , ( "website", E.string sf.website )
        , ( "notes", E.string sf.notes )
        ]


studentFormPayload : StudentForm -> E.Value
studentFormPayload sf =
    E.object
        [ ( "name", E.string sf.name )
        , ( "studentCode", E.string sf.studentCode )
        , ( "countryCode", E.string sf.countryCode )
        , ( "schoolId", E.string sf.schoolId )
        , ( "agentId", E.string sf.agentId )
        , ( "program", E.string sf.program )
        , ( "acceptanceStatus", E.string sf.acceptanceStatus )
        , ( "visaStatus", E.string sf.visaStatus )
        , ( "invoiceStatus", E.string sf.invoiceStatus )
        , ( "notes", E.string sf.notes )
        ]


agentFormPayload : AgentForm -> E.Value
agentFormPayload af =
    E.object
        [ ( "name", E.string af.name )
        , ( "agentCode", E.string af.agentCode )
        , ( "countryCode", E.string af.countryCode )
        , ( "contractStatus", E.string af.contractStatus )
        , ( "agentStatus", E.string af.agentStatus )
        , ( "notes", E.string af.notes )
        ]


leadFormPayload : LeadForm -> E.Value
leadFormPayload lf =
    E.object
        [ ( "leadNumber", E.string lf.leadNumber )
        , ( "name", E.string lf.name )
        , ( "email", E.string lf.email )
        , ( "phone", E.string lf.phone )
        , ( "nationality", E.string lf.nationality )
        , ( "currentCountry", E.string lf.currentCountry )
        , ( "interestedCountry", E.string lf.interestedCountry )
        , ( "interestedService", E.string lf.interestedService )
        , ( "source", E.string lf.source )
        , ( "assignedTo", E.string lf.assignedTo )
        , ( "status", E.string lf.status )
        , ( "followUpDate", E.string lf.followUpDate )
        , ( "notes", E.string lf.notes )
        ]


activityFormPayload : ActivityForm -> E.Value
activityFormPayload af =
    E.object
        [ ( "kind", E.string af.kind )
        , ( "title", E.string af.title )
        , ( "body", E.string af.body )
        , ( "occurredAt", E.string af.occurredAt )
        , ( "dealId", E.string "" )
        ]


type alias DealCore =
    { id : String
    , title : String
    , contactId : String
    , contactName : String
    , value : Float
    , currency : String
    }


type alias DealExtras =
    { stage : String
    , closeDate : String
    , owner : String
    , notes : String
    , createdAt : String
    }


dealCoreDecoder : D.Decoder DealCore
dealCoreDecoder =
    D.map6 DealCore
        (D.field "id" D.string)
        (D.field "title" D.string)
        (optString "contactId")
        (optString "contactName")
        (D.oneOf [ D.field "value" D.float, D.succeed 0 ])
        (D.oneOf [ D.field "currency" D.string, D.succeed "USD" ])


dealExtrasDecoder : D.Decoder DealExtras
dealExtrasDecoder =
    D.map5 DealExtras
        (D.oneOf [ D.field "stage" D.string, D.succeed "Lead" ])
        (optString "closeDate")
        (optString "owner")
        (optString "notes")
        (optString "createdAt")


dealDecoder : D.Decoder Deal
dealDecoder =
    D.map4
        (\core extras probability ownerId ->
            Deal
                core.id
                core.title
                core.contactId
                core.contactName
                core.value
                probability
                core.currency
                extras.stage
                extras.closeDate
                ownerId
                extras.owner
                extras.notes
                extras.createdAt
        )
        dealCoreDecoder
        dealExtrasDecoder
        (D.maybe (D.field "probability" D.float))
        (optString "ownerId")


dealsDecoder : D.Decoder (List Deal)
dealsDecoder =
    D.field "deals" (D.list dealDecoder)



-- EXPECTS -----------------------------------------------------------------


authExpect : (Result String AuthResponse -> msg) -> Http.Expect msg
authExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error. Check your connection."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Request failed (" ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case D.decodeString authResponseDecoder body of
                        Ok res ->
                            Ok res

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


userExpect : (Result String User -> msg) -> Http.Expect msg
userExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Not authorized."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "user" userDecoder) body of
                        Ok user ->
                            Ok user

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


activitiesExpect : (Result String (List Activity) -> msg) -> Http.Expect msg
activitiesExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load activity."

                Http.GoodStatus_ _ body ->
                    case D.decodeString activitiesDecoder body of
                        Ok as_ ->
                            Ok as_

                        Err err ->
                            Err ("Could not parse activity: " ++ D.errorToString err)


savedContactExpect : (Result ApiError Contact -> msg) -> Http.Expect msg
savedContactExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error. Check your connection.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save contact.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "contact" contactDecoder) body of
                        Ok c ->
                            Ok c

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedExpect : (Result String String -> msg) -> Http.Expect msg
deletedExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error. Check your connection."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete contact."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedDealExpect : (Result ApiError Deal -> msg) -> Http.Expect msg
savedDealExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error. Check your connection.")

                Http.BadStatus_ metadata body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err
                                        (GenericError
                                            ("Could not save deal (HTTP "
                                                ++ String.fromInt metadata.statusCode
                                                ++ ")."
                                            )
                                        )

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deal" dealDecoder) body of
                        Ok d ->
                            Ok d

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedDealExpect : (Result String String -> msg) -> Http.Expect msg
deletedDealExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error. Check your connection."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err
                                ("Could not delete deal (HTTP "
                                    ++ String.fromInt metadata.statusCode
                                    ++ ")."
                                )

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedActivityExpect : (Result ApiError Activity -> msg) -> Http.Expect msg
savedActivityExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error. Check your connection.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save activity.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "activity" activityDecoder) body of
                        Ok a ->
                            Ok a

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedActivityExpect : (Result String String -> msg) -> Http.Expect msg
deletedActivityExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error. Check your connection."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete activity."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


profileUpdateExpect : (Result ApiError ProfileUpdateResponse -> msg) -> Http.Expect msg
profileUpdateExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error. Check your connection.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not update profile.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString profileUpdateDecoder body of
                        Ok res ->
                            Ok res

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


statusExpect : (Result String String -> msg) -> Http.Expect msg
statusExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error. Check your connection."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Request failed."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "status" D.string) body of
                        Ok s ->
                            Ok s

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


contactsPageExpect : (Result String ( List Contact, Int ) -> msg) -> Http.Expect msg
contactsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load contacts."

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "contacts" (D.list contactDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse contacts: " ++ D.errorToString err)


dealsPageExpect : (Result String ( List Deal, Int ) -> msg) -> Http.Expect msg
dealsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load deals (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "deals" (D.list dealDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse deals: " ++ D.errorToString err)


schoolsPageExpect : (Result String ( List School, Int ) -> msg) -> Http.Expect msg
schoolsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load schools (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "schools" (D.list schoolDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse schools: " ++ D.errorToString err)


studentsPageExpect : (Result String ( List Student, Int ) -> msg) -> Http.Expect msg
studentsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load students (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "students" (D.list studentDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse students: " ++ D.errorToString err)


agentsPageExpect : (Result String ( List Agent, Int ) -> msg) -> Http.Expect msg
agentsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load agents (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "agents" (D.list agentDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse agents: " ++ D.errorToString err)


leadsPageExpect : (Result String ( List Lead, Int ) -> msg) -> Http.Expect msg
leadsPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load leads (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "leads" (D.list leadDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse leads: " ++ D.errorToString err)


savedSchoolExpect : (Result ApiError School -> msg) -> Http.Expect msg
savedSchoolExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save school.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "school" schoolDecoder) body of
                        Ok s ->
                            Ok s

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedSchoolExpect : (Result String String -> msg) -> Http.Expect msg
deletedSchoolExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete school."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedStudentExpect : (Result ApiError Student -> msg) -> Http.Expect msg
savedStudentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save student.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "student" studentDecoder) body of
                        Ok s ->
                            Ok s

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedStudentExpect : (Result String String -> msg) -> Http.Expect msg
deletedStudentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete student."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedAgentExpect : (Result ApiError Agent -> msg) -> Http.Expect msg
savedAgentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save agent.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "agent" agentDecoder) body of
                        Ok a ->
                            Ok a

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedAgentExpect : (Result String String -> msg) -> Http.Expect msg
deletedAgentExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete agent."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


savedLeadExpect : (Result ApiError Lead -> msg) -> Http.Expect msg
savedLeadExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save lead.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "lead" leadDecoder) body of
                        Ok l ->
                            Ok l

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedLeadExpect : (Result String String -> msg) -> Http.Expect msg
deletedLeadExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete lead."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)


taskDecoder : D.Decoder Task
taskDecoder =
    D.map Task (D.field "id" D.string)
        |> andMap (D.field "title" D.string)
        |> andMap (optString "description")
        |> andMap (D.oneOf [ D.field "status" D.string, D.succeed "todo" ])
        |> andMap (optString "dueDate")
        |> andMap (optString "contactId")
        |> andMap (optString "contactName")
        |> andMap (optString "owner")
        |> andMap (optString "createdAt")


taskFormPayload : TaskForm -> E.Value
taskFormPayload tf =
    E.object
        [ ( "title", E.string tf.title )
        , ( "description", E.string tf.description )
        , ( "status", E.string tf.status )
        , ( "dueDate", E.string tf.dueDate )
        , ( "contactId", E.string tf.contactId )
        , ( "owner", E.string tf.owner )
        ]


tasksPageExpect : (Result String ( List Task, Int ) -> msg) -> Http.Expect msg
tasksPageExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ metadata body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err ("Could not load tasks (HTTP " ++ String.fromInt metadata.statusCode ++ ").")

                Http.GoodStatus_ _ body ->
                    case
                        D.decodeString
                            (D.map2 Tuple.pair
                                (D.field "tasks" (D.list taskDecoder))
                                (D.oneOf [ D.field "total" D.int, D.succeed 0 ])
                            )
                            body
                    of
                        Ok pair ->
                            Ok pair

                        Err err ->
                            Err ("Could not parse tasks: " ++ D.errorToString err)


savedTaskExpect : (Result ApiError Task -> msg) -> Http.Expect msg
savedTaskExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err (GenericError ("Bad URL: " ++ url))

                Http.Timeout_ ->
                    Err (GenericError "Request timed out.")

                Http.NetworkError_ ->
                    Err (GenericError "Network error.")

                Http.BadStatus_ _ body ->
                    case D.decodeString fieldErrorDecoder body of
                        Ok fields ->
                            Err (FieldErrors fields)

                        Err _ ->
                            case D.decodeString errorDecoder body of
                                Ok msg ->
                                    Err (GenericError msg)

                                Err _ ->
                                    Err (GenericError "Could not save task.")

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "task" taskDecoder) body of
                        Ok task ->
                            Ok task

                        Err err ->
                            Err (GenericError ("Could not parse response: " ++ D.errorToString err))


deletedTaskExpect : (Result String String -> msg) -> Http.Expect msg
deletedTaskExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not delete task."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deleted" D.string) body of
                        Ok id ->
                            Ok id

                        Err err ->
                            Err ("Could not parse response: " ++ D.errorToString err)



-- SINGLE-RECORD FETCH EXPECTS --------------------------------------------


contactFetchExpect : (Result String Contact -> msg) -> Http.Expect msg
contactFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load contact."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "contact" contactDecoder) body of
                        Ok c ->
                            Ok c

                        Err err ->
                            Err ("Could not parse contact: " ++ D.errorToString err)


dealFetchExpect : (Result String Deal -> msg) -> Http.Expect msg
dealFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load deal."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "deal" dealDecoder) body of
                        Ok d ->
                            Ok d

                        Err err ->
                            Err ("Could not parse deal: " ++ D.errorToString err)


schoolFetchExpect : (Result String School -> msg) -> Http.Expect msg
schoolFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load school."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "school" schoolDecoder) body of
                        Ok s ->
                            Ok s

                        Err err ->
                            Err ("Could not parse school: " ++ D.errorToString err)


studentFetchExpect : (Result String Student -> msg) -> Http.Expect msg
studentFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load student."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "student" studentDecoder) body of
                        Ok s ->
                            Ok s

                        Err err ->
                            Err ("Could not parse student: " ++ D.errorToString err)


agentFetchExpect : (Result String Agent -> msg) -> Http.Expect msg
agentFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load agent."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "agent" agentDecoder) body of
                        Ok a ->
                            Ok a

                        Err err ->
                            Err ("Could not parse agent: " ++ D.errorToString err)


leadFetchExpect : (Result String Lead -> msg) -> Http.Expect msg
leadFetchExpect toMsg =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.BadUrl_ url ->
                    Err ("Bad URL: " ++ url)

                Http.Timeout_ ->
                    Err "Request timed out."

                Http.NetworkError_ ->
                    Err "Network error."

                Http.BadStatus_ _ body ->
                    case D.decodeString errorDecoder body of
                        Ok msg ->
                            Err msg

                        Err _ ->
                            Err "Could not load lead."

                Http.GoodStatus_ _ body ->
                    case D.decodeString (D.field "lead" leadDecoder) body of
                        Ok l ->
                            Ok l

                        Err err ->
                            Err ("Could not parse lead: " ++ D.errorToString err)



-- REQUESTS ----------------------------------------------------------------


login : String -> String -> String -> Bool -> (Result String AuthResponse -> msg) -> Cmd msg
login email password otp remember toMsg =
    Http.post
        { url = "/api/login"
        , body = Http.jsonBody (E.object [ ( "email", E.string email ), ( "password", E.string password ), ( "otp", E.string otp ), ( "remember", E.bool remember ) ])
        , expect = authExpect toMsg
        }


register : String -> String -> String -> (Result String AuthResponse -> msg) -> Cmd msg
register name email password toMsg =
    Http.post
        { url = "/api/register"
        , body = Http.jsonBody (registerPayload name email password)
        , expect = authExpect toMsg
        }


me : String -> (Result String User -> msg) -> Cmd msg
me token toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/me"
        , body = Http.emptyBody
        , expect = userExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchContact : String -> String -> (Result String Contact -> msg) -> Cmd msg
fetchContact token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/contacts/" ++ id
        , body = Http.emptyBody
        , expect = contactFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchDeal : String -> String -> (Result String Deal -> msg) -> Cmd msg
fetchDeal token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/deals/" ++ id
        , body = Http.emptyBody
        , expect = dealFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchSchool : String -> String -> (Result String School -> msg) -> Cmd msg
fetchSchool token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/schools/" ++ id
        , body = Http.emptyBody
        , expect = schoolFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchStudent : String -> String -> (Result String Student -> msg) -> Cmd msg
fetchStudent token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/students/" ++ id
        , body = Http.emptyBody
        , expect = studentFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchStudentDossier : String -> String -> (Result String Dossier -> msg) -> Cmd msg
fetchStudentDossier token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/students/" ++ id ++ "/dossier"
        , body = Http.emptyBody
        , expect = dossierExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchAgent : String -> String -> (Result String Agent -> msg) -> Cmd msg
fetchAgent token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/agents/" ++ id
        , body = Http.emptyBody
        , expect = agentFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchLead : String -> String -> (Result String Lead -> msg) -> Cmd msg
fetchLead token id toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/leads/" ++ id
        , body = Http.emptyBody
        , expect = leadFetchExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchContacts : String -> String -> Int -> Int -> (Result String ( List Contact, Int ) -> msg) -> Cmd msg
fetchContacts token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/contacts?" ++ qs
        , body = Http.emptyBody
        , expect = contactsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createContact : String -> ContactForm -> (Result ApiError Contact -> msg) -> Cmd msg
createContact token cf toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/contacts"
        , body = Http.jsonBody (contactFormPayload cf)
        , expect = savedContactExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateContact : String -> String -> ContactForm -> (Result ApiError Contact -> msg) -> Cmd msg
updateContact token id cf toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/contacts/" ++ id
        , body = Http.jsonBody (contactFormPayload cf)
        , expect = savedContactExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteContact : String -> String -> (Result String String -> msg) -> Cmd msg
deleteContact token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/contacts/" ++ id
        , body = Http.emptyBody
        , expect = deletedExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchActivities : String -> String -> (Result String (List Activity) -> msg) -> Cmd msg
fetchActivities token contactId toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/contacts/" ++ contactId ++ "/activities"
        , body = Http.emptyBody
        , expect = activitiesExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createActivity : String -> String -> ActivityForm -> (Result ApiError Activity -> msg) -> Cmd msg
createActivity token contactId af toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/contacts/" ++ contactId ++ "/activities"
        , body = Http.jsonBody (activityFormPayload af)
        , expect = savedActivityExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteActivity : String -> String -> (Result String String -> msg) -> Cmd msg
deleteActivity token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/activities/" ++ id
        , body = Http.emptyBody
        , expect = deletedActivityExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchDeals : String -> String -> Int -> Int -> (Result String ( List Deal, Int ) -> msg) -> Cmd msg
fetchDeals token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/deals?" ++ qs
        , body = Http.emptyBody
        , expect = dealsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createDeal : String -> DealForm -> (Result ApiError Deal -> msg) -> Cmd msg
createDeal token df toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/deals"
        , body = Http.jsonBody (dealFormPayload df)
        , expect = savedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateDeal : String -> String -> DealForm -> (Result ApiError Deal -> msg) -> Cmd msg
updateDeal token id df toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/deals/" ++ id
        , body = Http.jsonBody (dealFormPayload df)
        , expect = savedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateDealStage : String -> String -> String -> (Result ApiError Deal -> msg) -> Cmd msg
updateDealStage token id newStage toMsg =
    Http.request
        { method = "PATCH"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/deals/" ++ id ++ "/stage"
        , body = Http.jsonBody (E.object [ ( "stage", E.string newStage ) ])
        , expect = savedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteDeal : String -> String -> (Result String String -> msg) -> Cmd msg
deleteDeal token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/deals/" ++ id
        , body = Http.emptyBody
        , expect = deletedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchSchools : String -> String -> Int -> Int -> (Result String ( List School, Int ) -> msg) -> Cmd msg
fetchSchools token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/schools?" ++ qs
        , body = Http.emptyBody
        , expect = schoolsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createSchool : String -> SchoolForm -> (Result ApiError School -> msg) -> Cmd msg
createSchool token sf toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/schools"
        , body = Http.jsonBody (schoolFormPayload sf)
        , expect = savedSchoolExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateSchool : String -> String -> SchoolForm -> (Result ApiError School -> msg) -> Cmd msg
updateSchool token id sf toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/schools/" ++ id
        , body = Http.jsonBody (schoolFormPayload sf)
        , expect = savedSchoolExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteSchool : String -> String -> (Result String String -> msg) -> Cmd msg
deleteSchool token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/schools/" ++ id
        , body = Http.emptyBody
        , expect = deletedSchoolExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchStudents : String -> String -> Int -> Int -> (Result String ( List Student, Int ) -> msg) -> Cmd msg
fetchStudents token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/students?" ++ qs
        , body = Http.emptyBody
        , expect = studentsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchLinkedStudents : String -> String -> String -> (Result String ( List Student, Int ) -> msg) -> Cmd msg
fetchLinkedStudents token relation recordId toMsg =
    let
        qs =
            relation ++ "Id=" ++ Url.percentEncode recordId ++ "&limit=200&offset=0"
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/students?" ++ qs
        , body = Http.emptyBody
        , expect = studentsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createStudent : String -> StudentForm -> (Result ApiError Student -> msg) -> Cmd msg
createStudent token sf toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/students"
        , body = Http.jsonBody (studentFormPayload sf)
        , expect = savedStudentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateStudent : String -> String -> StudentForm -> (Result ApiError Student -> msg) -> Cmd msg
updateStudent token id sf toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/students/" ++ id
        , body = Http.jsonBody (studentFormPayload sf)
        , expect = savedStudentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteStudent : String -> String -> (Result String String -> msg) -> Cmd msg
deleteStudent token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/students/" ++ id
        , body = Http.emptyBody
        , expect = deletedStudentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchAgents : String -> String -> Int -> Int -> (Result String ( List Agent, Int ) -> msg) -> Cmd msg
fetchAgents token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/agents?" ++ qs
        , body = Http.emptyBody
        , expect = agentsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createAgent : String -> AgentForm -> (Result ApiError Agent -> msg) -> Cmd msg
createAgent token af toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/agents"
        , body = Http.jsonBody (agentFormPayload af)
        , expect = savedAgentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateAgent : String -> String -> AgentForm -> (Result ApiError Agent -> msg) -> Cmd msg
updateAgent token id af toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/agents/" ++ id
        , body = Http.jsonBody (agentFormPayload af)
        , expect = savedAgentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteAgent : String -> String -> (Result String String -> msg) -> Cmd msg
deleteAgent token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/agents/" ++ id
        , body = Http.emptyBody
        , expect = deletedAgentExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchLeads : String -> String -> Int -> Int -> (Result String ( List Lead, Int ) -> msg) -> Cmd msg
fetchLeads token query limit offset toMsg =
    let
        qs =
            String.join "&"
                [ "q=" ++ Url.percentEncode query
                , "limit=" ++ String.fromInt limit
                , "offset=" ++ String.fromInt offset
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/leads?" ++ qs
        , body = Http.emptyBody
        , expect = leadsPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createLead : String -> LeadForm -> (Result ApiError Lead -> msg) -> Cmd msg
createLead token lf toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/leads"
        , body = Http.jsonBody (leadFormPayload lf)
        , expect = savedLeadExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateLead : String -> String -> LeadForm -> (Result ApiError Lead -> msg) -> Cmd msg
updateLead token id lf toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/leads/" ++ id
        , body = Http.jsonBody (leadFormPayload lf)
        , expect = savedLeadExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteLead : String -> String -> (Result String String -> msg) -> Cmd msg
deleteLead token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/leads/" ++ id
        , body = Http.emptyBody
        , expect = deletedLeadExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateMe : String -> String -> String -> (Result ApiError ProfileUpdateResponse -> msg) -> Cmd msg
updateMe token name email toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/me"
        , body =
            Http.jsonBody
                (E.object
                    [ ( "name", E.string name )
                    , ( "email", E.string email )
                    ]
                )
        , expect = profileUpdateExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


changePassword : String -> String -> String -> (Result String String -> msg) -> Cmd msg
changePassword token current next toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/me/password"
        , body =
            Http.jsonBody
                (E.object
                    [ ( "currentPassword", E.string current )
                    , ( "newPassword", E.string next )
                    ]
                )
        , expect = statusExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


logoutAll : String -> (Result String String -> msg) -> Cmd msg
logoutAll token toMsg =
    Http.request
        { method = "POST"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/me/logout-all"
        , body = Http.emptyBody
        , expect = statusExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchTasks : String -> String -> String -> (Result String ( List Task, Int ) -> msg) -> Cmd msg
fetchTasks token query statusFilter toMsg =
    fetchTasksPage token query statusFilter 0 toMsg


fetchTasksPage : String -> String -> String -> Int -> (Result String ( List Task, Int ) -> msg) -> Cmd msg
fetchTasksPage token query statusFilter offset toMsg =
    let
        parts =
            List.filter (\part -> not (String.isEmpty part))
                [ "q=" ++ Url.percentEncode query
                , if String.isEmpty statusFilter then
                    ""

                  else
                    "status=" ++ Url.percentEncode statusFilter
                ]
    in
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/tasks?limit=25&offset=" ++ String.fromInt offset ++ "&" ++ String.join "&" parts
        , body = Http.emptyBody
        , expect = tasksPageExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


createTask : String -> TaskForm -> (Result ApiError Task -> msg) -> Cmd msg
createTask token tf toMsg =
    Http.request
        { method = "POST"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/tasks"
        , body = Http.jsonBody (taskFormPayload tf)
        , expect = savedTaskExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateTask : String -> String -> TaskForm -> (Result ApiError Task -> msg) -> Cmd msg
updateTask token id tf toMsg =
    Http.request
        { method = "PUT"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/tasks/" ++ id
        , body = Http.jsonBody (taskFormPayload tf)
        , expect = savedTaskExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


updateTaskStatus : String -> String -> String -> (Result ApiError Task -> msg) -> Cmd msg
updateTaskStatus token id newStatus toMsg =
    Http.request
        { method = "PATCH"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/tasks/" ++ id ++ "/status"
        , body = Http.jsonBody (E.object [ ( "status", E.string newStatus ) ])
        , expect = savedTaskExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteTask : String -> String -> (Result String String -> msg) -> Cmd msg
deleteTask token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/tasks/" ++ id
        , body = Http.emptyBody
        , expect = deletedTaskExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }



-- BULK / PARTIAL UPDATES --------------------------------------------------


updateDealStageOnly : String -> String -> String -> (Result ApiError Deal -> msg) -> Cmd msg
updateDealStageOnly token id newStage toMsg =
    Http.request
        { method = "PATCH"
        , headers =
            [ Http.header "Authorization" ("Bearer " ++ token)
            , Http.header "Content-Type" "application/json"
            ]
        , url = "/api/deals/" ++ id ++ "/stage"
        , body = Http.jsonBody (E.object [ ( "stage", E.string newStage ) ])
        , expect = savedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


deleteDealRaw : String -> String -> (Result String String -> msg) -> Cmd msg
deleteDealRaw token id toMsg =
    Http.request
        { method = "DELETE"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/deals/" ++ id
        , body = Http.emptyBody
        , expect = deletedDealExpect toMsg
        , timeout = Just 20000
        , tracker = Nothing
        }


fetchReports : String -> (Result String ReportSummary -> msg) -> Cmd msg
fetchReports token toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/reports"
        , body = Http.emptyBody
        , expect =
            Http.expectJson (Result.mapError httpErrorMessage >> toMsg)
                (D.succeed ReportSummary
                    |> andMap (optString "pipelineLabel")
                    |> andMap (optString "wonLabel")
                    |> andMap (optString "balanceLabel")
                    |> andMap (D.field "contacts" D.int)
                    |> andMap (D.field "students" D.int)
                    |> andMap (D.field "leads" D.int)
                    |> andMap (D.field "activeCases" D.int)
                    |> andMap (D.field "openTasks" D.int)
                    |> andMap (D.field "pipelineValue" D.float)
                    |> andMap (D.field "wonValue" D.float)
                    |> andMap (D.field "outstandingBalance" D.float)
                    |> andMap (D.field "stageCounts" (D.dict D.int))
                    |> andMap (D.field "stageValues" (D.dict D.float))
                    |> andMap (D.field "createdCounts" (D.dict D.int))
                    |> andMap (D.field "newLeads" D.int)
                    |> andMap (D.field "approvedStudents" D.int)
                    |> andMap (D.field "totalCases" D.int)
                )
        , timeout = Just 15000
        , tracker = Nothing
        }


workspaceGet : String -> String -> D.Decoder a -> (Result String a -> msg) -> Cmd msg
workspaceGet token path decoder toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = path
        , body = Http.emptyBody
        , expect = Http.expectJson (Result.mapError httpErrorMessage >> toMsg) decoder
        , timeout = Just 15000
        , tracker = Nothing
        }


fetchGlobalSearch : String -> String -> (Result String SearchData -> msg) -> Cmd msg
fetchGlobalSearch token query toMsg =
    workspaceGet token
        ("/api/search?q=" ++ Url.percentEncode query)
        (D.map2 SearchData
            (D.field "results" (D.list (D.map4 SearchResult (D.field "entity" D.string) (D.field "id" D.string) (D.field "title" D.string) (D.field "subtitle" D.string))))
            (D.field "truncated" D.bool)
        )
        toMsg


fetchAudit : String -> Int -> (Result String AuditData -> msg) -> Cmd msg
fetchAudit token offset toMsg =
    workspaceGet token
        ("/api/audit?limit=25&offset=" ++ String.fromInt offset)
        (D.map2 AuditData
            (D.field "events"
                (D.list
                    (D.map7 AuditEvent
                        (D.field "id" D.int)
                        (D.field "actor" D.string)
                        (D.field "action" D.string)
                        (D.field "entity" D.string)
                        (D.field "recordId" D.string)
                        (D.field "label" D.string)
                        (D.field "occurredAt" D.string)
                    )
                )
            )
            (D.field "total" D.int)
        )
        toMsg


fetchExport : String -> String -> (Result String String -> msg) -> Cmd msg
fetchExport token entity toMsg =
    Http.request
        { method = "GET"
        , headers = [ Http.header "Authorization" ("Bearer " ++ token) ]
        , url = "/api/exports/" ++ Url.percentEncode entity
        , body = Http.emptyBody
        , expect = Http.expectString (Result.mapError httpErrorMessage >> toMsg)
        , timeout = Just 30000
        , tracker = Nothing
        }


refreshSession : (Result String AuthResponse -> msg) -> Cmd msg
refreshSession toMsg =
    Http.post { url = "/api/session/refresh", body = Http.emptyBody, expect = authExpect toMsg }


httpErrorMessage : Http.Error -> String
httpErrorMessage error =
    case error of
        Http.NetworkError ->
            "Network unavailable. Check your connection and retry."

        Http.Timeout ->
            "The request timed out. Please retry."

        Http.BadStatus status ->
            if status >= 500 then
                "The server could not complete the request. Please retry."

            else if status == 401 then
                "Your session expired. Please sign in again."

            else if status == 409 then
                "The record changed or has related records. Reload it and check before retrying."

            else if status == 403 then
                "Your role does not have permission for this action."

            else
                "The request was rejected. Check the details and retry."

        _ ->
            "Could not read the response. Please retry."


fetchWorkspaceClock : String -> (Result String ( String, Int ) -> msg) -> Cmd msg
fetchWorkspaceClock token toMsg =
    workspaceGet token "/api/clock" (D.map2 Tuple.pair (D.field "today" D.string) (D.field "unread" D.int)) toMsg
