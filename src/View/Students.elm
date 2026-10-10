module View.Students exposing (deleteStudentConfirmModal, linkedStudentsView, studentDetailView, studentFormModal, studentsView)

{-| Students: list, detail (with case/documents/invoice dossier), form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Router exposing (routeToPath)
import Svg
import Types exposing (..)
import View.Dashboard exposing (ActivityEntry, PriorityAlert, activityPanel, alertsPanel, caseEntry, documentCorrectionAlert, documentEntry, expiredDocumentAlert, invoiceEntry, outstandingInvoiceAlert)
import View.Format exposing (formatCurrency, formatCurrencyWith)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconMail, iconPin, iconSchool, iconStudent, iconTasks, iconTrash, iconUserTiny)


studentStatusBadge : String -> String -> Html Msg
studentStatusBadge category status =
    let
        cls =
            case ( category, String.toLower status ) of
                ( "acceptance", "accepted" ) ->
                    "badge badge--success"

                ( "acceptance", "rejected" ) ->
                    "badge badge--lost"

                ( "acceptance", "waitlisted" ) ->
                    "badge badge--info"

                ( "acceptance", _ ) ->
                    "badge badge--muted"

                ( "visa", "approved" ) ->
                    "badge badge--success"

                ( "visa", "denied" ) ->
                    "badge badge--lost"

                ( "visa", "pending" ) ->
                    "badge badge--info"

                ( "visa", _ ) ->
                    "badge badge--muted"

                ( "invoice", "paid" ) ->
                    "badge badge--success"

                ( "invoice", "overdue" ) ->
                    "badge badge--lost"

                ( "invoice", "issued" ) ->
                    "badge badge--info"

                ( "invoice", _ ) ->
                    "badge badge--muted"

                _ ->
                    "badge"
    in
    span [ class cls ] [ text status ]


caseStageBadge : String -> Html Msg
caseStageBadge s =
    let
        cls =
            case String.toLower s of
                "approved" ->
                    "badge badge--success"

                "refused" ->
                    "badge badge--lost"

                "closed" ->
                    "badge badge--muted"

                "submitted" ->
                    "badge badge--customer"

                _ ->
                    "badge badge--info"
    in
    span [ class cls ] [ text s ]


docStatusBadge : String -> Html Msg
docStatusBadge s =
    let
        cls =
            case String.toLower s of
                "verified" ->
                    "badge badge--success"

                "expired" ->
                    "badge badge--lost"

                "correction required" ->
                    "badge badge--proposal"

                "received" ->
                    "badge badge--info"

                "under review" ->
                    "badge badge--info"

                "requested" ->
                    "badge badge--qualified"

                _ ->
                    "badge badge--muted"
    in
    span [ class cls ] [ text s ]


milestoneBadge : String -> Html Msg
milestoneBadge m =
    let
        cls =
            case String.toLower m of
                "fully paid" ->
                    "badge badge--success"

                "refunded" ->
                    "badge badge--muted"

                "refund review" ->
                    "badge badge--proposal"

                "deposit paid" ->
                    "badge badge--info"

                _ ->
                    "badge badge--muted"
    in
    span [ class cls ] [ text m ]


studentRow : Student -> Html Msg
studentRow s =
    tr [ class "contact-row" ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ node "crm-record-photo"
                    [ class "contact-avatar"
                    , Attr.attribute "entity" "students"
                    , Attr.attribute "record-id" s.id
                    , Attr.attribute "initials" (initials s.name)
                    , Attr.attribute "person-name" s.name
                    ]
                    []
                , div [ class "contact-name-info" ]
                    [ a [ class "contact-name record-link", Attr.href (routeToPath (StudentDetail s.id)) ] [ text s.name ]
                    , span [ class "contact-email" ]
                        [ text
                            (if String.isEmpty s.studentCode then
                                s.program

                             else
                                s.studentCode
                            )
                        ]
                    ]
                ]
            ]
        , td [] [ text s.schoolName ]
        , td []
            [ text
                (if String.isEmpty s.agentName then
                    "—"

                 else
                    s.agentName
                )
            ]
        , td [] [ text s.program ]
        , td [] [ studentStatusBadge "acceptance" s.acceptanceStatus ]
        , td [] [ studentStatusBadge "visa" s.visaStatus ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ s.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditStudent s, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ s.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteStudent s, True ))
                ]
                [ iconTrash ]
            ]
        ]


linkedStudentsView : Model -> String -> Html Msg
linkedStudentsView model parentId =
    case model.linkedStudents of
        Loading ->
            div [ class "activity-loading" ] [ text "Loading linked students…" ]

        Failure message ->
            div [ class "activity-loading" ] [ text ("Could not load linked students: " ++ message) ]

        Success data ->
            if data.parentId /= parentId then
                div [ class "activity-loading" ] [ text "Loading linked students…" ]

            else if List.isEmpty data.items then
                p [ class "detail-muted" ] [ text "No linked students yet." ]

            else
                div []
                    ([ div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Student" ]
                                    , th [] [ text "School" ]
                                    , th [] [ text "Agent" ]
                                    , th [] [ text "Program" ]
                                    , th [] [ text "Acceptance" ]
                                    , th [] [ text "Visa" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map studentRow data.items)
                            ]
                        ]
                     ]
                        ++ (if data.total > List.length data.items then
                                [ p [ class "detail-muted" ]
                                    [ text ("Showing " ++ String.fromInt (List.length data.items) ++ " of " ++ String.fromInt data.total ++ " linked students. Open the Students list to browse all records.") ]
                                ]

                            else
                                []
                           )
                    )

        NotAsked ->
            div [ class "activity-loading" ] [ text "Linked students are not loaded." ]


studentsSkeleton : Html Msg
studentsSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search students…", disabled True ] [] ]
            ]
        , div [ class "table-wrap" ]
            (List.repeat 5
                (div [ class "skeleton-row" ]
                    [ div [ class "skeleton-line skeleton-line--medium" ] []
                    , div [ class "skeleton-line skeleton-line--short" ] []
                    ]
                )
            )
        ]


studentsView : Model -> Html Msg
studentsView model =
    case model.students of
        NotAsked ->
            studentsSkeleton

        Loading ->
            studentsSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load students: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            let
                filtered =
                    data.items

                isQueryEmpty =
                    String.isEmpty (String.trim data.query)
            in
            div []
                [ div [ class "page-toolbar" ]
                    [ div [ class "page-toolbar__search" ]
                        [ svgIcon
                            [ Attr.attribute "viewBox" "0 0 24 24"
                            , Attr.attribute "width" "16"
                            , Attr.attribute "height" "16"
                            , Attr.attribute "fill" "none"
                            , Attr.attribute "stroke" "currentColor"
                            , Attr.attribute "stroke-width" "1.8"
                            , Attr.attribute "stroke-linecap" "round"
                            , Attr.attribute "stroke-linejoin" "round"
                            ]
                            [ Svg.node "circle"
                                [ Attr.attribute "cx" "11"
                                , Attr.attribute "cy" "11"
                                , Attr.attribute "r" "8"
                                ]
                                []
                            , svgPath "M21 21l-4.35-4.35"
                            ]
                        , input
                            [ type_ "search"
                            , Attr.attribute "aria-label" "Search students"
                            , placeholder "Search students…"
                            , value data.query
                            , onInput UpdatedStudentsQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddStudent
                        ]
                        [ text "Add student" ]
                    ]
                , if List.isEmpty filtered then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No students yet"

                                 else
                                    "No students match your search"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty then
                                    "Add your first student to start tracking applications."

                                 else
                                    "Try a different search term."
                                )
                            ]
                        , if isQueryEmpty then
                            div [ class "empty-state__action" ]
                                [ button
                                    [ class "ecc-btn ecc-btn--inline"
                                    , type_ "button"
                                    , onClick OpenedAddStudent
                                    ]
                                    [ text "Add your first student" ]
                                ]

                          else
                            text ""
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Student" ]
                                    , th [] [ text "School" ]
                                    , th [] [ text "Agent" ]
                                    , th [] [ text "Program" ]
                                    , th [] [ text "Acceptance" ]
                                    , th [] [ text "Visa" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map studentRow filtered)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit StudentsPageChanged
                ]


caseDossierRow : Case -> Html Msg
caseDossierRow c =
    div
        [ class "activity-item"
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , onClick (OpenedCaseDetail c)
        ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text c.caseNumber ]
                , caseStageBadge c.currentStage
                ]
            , div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ]
                    [ text (c.destinationCountry ++ " · " ++ c.visaType) ]
                , span [ class "activity-item__time" ]
                    [ text
                        (if String.isEmpty c.nextDeadline then
                            "No deadline"

                         else
                            "Deadline " ++ c.nextDeadline
                        )
                    ]
                ]
            ]
        ]


documentDossierRow : Document -> Html Msg
documentDossierRow d =
    div [ class "activity-item" ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text d.docName ]
                , docStatusBadge d.status
                ]
            , div [ class "activity-item__meta" ]
                [ if String.isEmpty d.caseNumber then
                    text ""

                  else
                    span [ class "activity-item__kind" ]
                        [ text d.caseNumber ]
                , if String.isEmpty d.dateRequested then
                    text ""

                  else
                    span [ class "activity-item__time" ]
                        [ text ("Requested " ++ d.dateRequested) ]
                ]
            ]
        ]


invoiceDossierRow : Invoice -> Html Msg
invoiceDossierRow inv =
    div
        [ class "activity-item"
        , Attr.attribute "role" "button"
        , Attr.attribute "tabindex" "0"
        , onClick (OpenedInvoiceDetail inv)
        ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ] [ text inv.invoiceNumber ]
                , milestoneBadge inv.paymentMilestone
                ]
            , div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ]
                    [ text ("Balance " ++ formatCurrencyWith inv.currency inv.balance) ]
                , if String.isEmpty inv.caseNumber then
                    text ""

                  else
                    span [ class "activity-item__time" ]
                        [ text inv.caseNumber ]
                ]
            ]
        ]


studentAlerts : Model -> List PriorityAlert
studentAlerts model =
    case model.studentDossier of
        Success dossier ->
            let
                corrections =
                    dossier.documents
                        |> List.filter (\d -> d.status == "Correction Required")
                        |> List.map documentCorrectionAlert

                expired =
                    dossier.documents
                        |> List.filter (\d -> d.status == "Expired")
                        |> List.map expiredDocumentAlert

                unpaid =
                    dossier.invoices
                        |> List.filter (\inv -> inv.balance > 0)
                        |> List.map outstandingInvoiceAlert
            in
            corrections ++ expired ++ unpaid

        _ ->
            []


studentActivity : Model -> List ActivityEntry
studentActivity model =
    case model.studentDossier of
        Success dossier ->
            List.map caseEntry dossier.cases
                ++ List.map documentEntry dossier.documents
                ++ List.map invoiceEntry dossier.invoices

        _ ->
            []


studentDetailView : Model -> Student -> Html Msg
studentDetailView model s =
    let
        display v =
            if String.isEmpty v then
                "—"

            else
                v

        schoolDisplay =
            display s.schoolName

        programDisplay =
            display s.program

        codeDisplay =
            display s.studentCode

        countryDisplay =
            display s.countryCode

        createdDisplay =
            display s.createdAt

        ownerDisplay =
            display s.createdBy

        dossierData =
            case model.studentDossier of
                Success dossier ->
                    { cases = dossier.cases
                    , documents = dossier.documents
                    , invoices = dossier.invoices
                    , status = "loaded"
                    }

                Loading ->
                    { cases = [], documents = [], invoices = [], status = "loading" }

                Failure _ ->
                    { cases = [], documents = [], invoices = [], status = "error" }

                NotAsked ->
                    { cases = [], documents = [], invoices = [], status = "notasked" }
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Students)
            ]
            [ iconBack
            , span [] [ text "Back to students" ]
            ]
        , a [ Attr.href "#record-tools?tab=workflows", Attr.target "_self", class "ecc-btn ecc-btn--ghost ecc-btn--inline" ] [ text "Application tools & create case" ]
        , header [ class "detail-hero" ]
            [ node "crm-record-photo"
                [ class "detail-hero__avatar"
                , Attr.attribute "entity" "students"
                , Attr.attribute "record-id" s.id
                , Attr.attribute "initials" (initials s.name)
                , Attr.attribute "editable" (if Maybe.withDefault False (Maybe.map (\u -> u.role /= "viewer") model.user) then "true" else "false")
                ]
                []
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text s.name ]
                    , studentStatusBadge "acceptance" s.acceptanceStatus
                    ]
                , p [ class "detail-hero__role" ]
                    [ text (schoolDisplay ++ " · " ++ programDisplay) ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditStudent s)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteStudent s)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , node "crm-student-journey" [ Attr.attribute "record-id" s.id ] []
        , div [ class "detail-stats" ]
            [ detailStat "Cases"
                (String.fromInt (List.length dossierData.cases))
                "Total"
            , detailStat "Documents"
                (String.fromInt (List.length dossierData.documents))
                "On file"
            , detailStat "Invoices"
                (String.fromInt (List.length dossierData.invoices))
                "Issued"
            , detailStat "Visa" s.visaStatus "Immigration"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "About"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconStudent "Student code" codeDisplay
                        , infoRow iconSchool "School" schoolDisplay
                        , infoRow iconUserTiny "Agent" (display s.agentName)
                        , infoRow iconTasks "Program" programDisplay
                        , infoRow iconPin "Country" countryDisplay
                        , infoRow iconCalendar "Created" createdDisplay
                        , infoRow iconContacts "Added by" ownerDisplay
                        ]
                    )
                , detailCard "Status"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconTasks "Acceptance" s.acceptanceStatus
                        , infoRow iconMail "Visa" s.visaStatus
                        , infoRow iconDeals "Invoice" s.invoiceStatus
                        ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty s.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text s.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Cases"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , onClick (OpenedAddCaseForStudent s)
                            ]
                            [ text "New case" ]
                        )
                    )
                    (case dossierData.status of
                        "loading" ->
                            div [ class "activity-loading" ] [ text "Loading cases…" ]

                        "error" ->
                            div [ class "activity-loading" ]
                                [ text "Could not load dossier." ]

                        _ ->
                            if List.isEmpty dossierData.cases then
                                detailEmpty
                                    iconTasks
                                    "No cases yet"
                                    "Open a case to start tracking submissions and visa progress."

                            else
                                div [ class "activity-list" ]
                                    (List.map caseDossierRow dossierData.cases)
                    )
                , detailCard "Documents"
                    Nothing
                    (if List.isEmpty dossierData.documents then
                        detailEmpty
                            iconContacts
                            "No documents yet"
                            "Documents attached to this student's cases will appear here."

                     else
                        div [ class "activity-list" ]
                            (List.map documentDossierRow dossierData.documents)
                    )
                , detailCard "Invoices"
                    Nothing
                    (if List.isEmpty dossierData.invoices then
                        detailEmpty
                            iconDeals
                            "No invoices yet"
                            "Invoices tied to this student's cases will appear here."

                     else
                        div [ class "activity-list" ]
                            (List.map invoiceDossierRow dossierData.invoices)
                    )
                ]
            ]
        , div [ class "bottom-row" ]
            [ alertsPanel (studentAlerts model)
            , activityPanel (studentActivity model)
            ]
        ]


studentFormFieldError : String -> StudentForm -> Maybe String
studentFormFieldError field sf =
    sf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


studentRichField : StudentForm -> String -> String -> String -> Html Msg
studentRichField sf fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "name" ->
                    sf.name

                "studentCode" ->
                    sf.studentCode

                "countryCode" ->
                    sf.countryCode

                "program" ->
                    sf.program

                _ ->
                    ""

        err =
            studentFormFieldError fieldId sf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("stf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedStudentFormField fieldId)
            , disabled sf.submitting
            ]
            []
         , label [ for ("stf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


studentSchoolSelect : Model -> StudentForm -> Html Msg
studentSchoolSelect model sf =
    let
        options =
            let
                loaded =
                    case model.schools of
                        Success data ->
                            data.items

                        _ ->
                            []

                current =
                    case model.viewingSchool of
                        Just school ->
                            if List.any (\item -> item.id == school.id) loaded then
                                []

                            else
                                [ school ]

                        Nothing ->
                            []
            in
            List.sortBy .name (loaded ++ current)

        err =
            studentFormFieldError "schoolId" sf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ select
            [ id "stf-schoolId"
            , onInput (UpdatedStudentFormField "schoolId")
            , disabled sf.submitting
            ]
            (option [ value "", Attr.selected (sf.schoolId == "") ]
                [ text "No school linked" ]
                :: List.map
                    (\s ->
                        option
                            [ value s.id, Attr.selected (sf.schoolId == s.id) ]
                            [ text s.name ]
                    )
                    options
            )
         , label [ for "stf-schoolId" ] [ text "School" ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


studentAgentSelect : Model -> StudentForm -> Html Msg
studentAgentSelect model sf =
    let
        options =
            let
                loaded =
                    case model.agents of
                        Success data ->
                            data.items

                        _ ->
                            []

                current =
                    case model.viewingAgent of
                        Just agent ->
                            if List.any (\item -> item.id == agent.id) loaded then
                                []

                            else
                                [ agent ]

                        Nothing ->
                            []
            in
            List.sortBy .name (loaded ++ current)

        err =
            studentFormFieldError "agentId" sf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ select
            [ id "stf-agentId"
            , onInput (UpdatedStudentFormField "agentId")
            , disabled sf.submitting
            ]
            (option [ value "", Attr.selected (sf.agentId == "") ]
                [ text "No agent linked" ]
                :: List.map
                    (\a ->
                        option
                            [ value a.id, Attr.selected (sf.agentId == a.id) ]
                            [ text a.name ]
                    )
                    options
            )
         , label [ for "stf-agentId" ] [ text "Agent" ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


studentStatusPills : String -> String -> List String -> (String -> Msg) -> Bool -> Html Msg
studentStatusPills label current options toMsg isDisabled =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text label ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    let
                        cls =
                            if current == s then
                                "stage-pill stage-pill--active"

                            else
                                "stage-pill"
                    in
                    button
                        [ type_ "button"
                        , class cls
                        , onClick (toMsg s)
                        , disabled isDisabled
                        ]
                        [ text s ]
                )
                options
            )
        ]


studentFormView : Model -> StudentForm -> Bool -> Html Msg
studentFormView model sf isEdit =
    let
        formError =
            studentFormFieldError "form" sf

        submitLabel =
            if sf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save student"
    in
    form [ onSubmit SubmittedStudentForm, Attr.novalidate True ]
        [ case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ studentRichField sf "name" "Student full name" "text"
            , studentRichField sf "studentCode" "Student ID code" "text"
            , studentRichField sf "countryCode" "Country code (e.g. PH)" "text"
            , studentSchoolSelect model sf
            , studentAgentSelect model sf
            , studentRichField sf "program" "Program enrolled" "text"
            ]
        , studentStatusPills "Acceptance status"
            sf.acceptanceStatus
            acceptanceStatuses
            (UpdatedStudentFormField "acceptanceStatus")
            sf.submitting
        , studentStatusPills "Visa status"
            sf.visaStatus
            visaStatuses
            (UpdatedStudentFormField "visaStatus")
            sf.submitting
        , studentStatusPills "Invoice status"
            sf.invoiceStatus
            invoiceStatuses
            (UpdatedStudentFormField "invoiceStatus")
            sf.submitting
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "stf-notes"
                , placeholder "Additional notes or remarks…"
                , value sf.notes
                , onInput (UpdatedStudentFormField "notes")
                , disabled sf.submitting
                , Attr.rows 4
                ]
                []
            ]
        , node "crm-form-quality"
            [ Attr.attribute "entity" "students"
            , Attr.attribute "name" sf.name
            , Attr.attribute "studentCode" sf.studentCode
            , Attr.attribute "schoolId" sf.schoolId
            , Attr.attribute "program" sf.program
            , Attr.attribute "record-id" (Maybe.withDefault "" model.editingStudentId)
            ]
            []
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseStudentForm
                , disabled sf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (sf.submitting || not sf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


studentFormModal : Model -> StudentForm -> Html Msg
studentFormModal model sf =
    let
        isEdit =
            model.editingStudentId /= Nothing

        titleText =
            if isEdit then
                "Edit student"

            else
                "Add student"
    in
    div [ class "modal-backdrop", onClick RequestedCloseStudentForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" titleText
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text titleText ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick RequestedCloseStudentForm
                    , Attr.attribute "aria-label" "Close"
                    ]
                    [ svgIcon
                        [ Attr.attribute "viewBox" "0 0 24 24"
                        , Attr.attribute "width" "18"
                        , Attr.attribute "height" "18"
                        , Attr.attribute "fill" "none"
                        , Attr.attribute "stroke" "currentColor"
                        , Attr.attribute "stroke-width" "2"
                        , Attr.attribute "stroke-linecap" "round"
                        , Attr.attribute "stroke-linejoin" "round"
                        ]
                        [ svgPath "M18 6 6 18"
                        , svgPath "M6 6l12 12"
                        ]
                    ]
                ]
            , if sf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes? They won't be saved." ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseStudentForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseStudentForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                studentFormView model sf isEdit
            ]
        ]


deleteStudentConfirmModal : Student -> Html Msg
deleteStudentConfirmModal student =
    div [ class "modal-backdrop", onClick CancelledDeleteStudent ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete student"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete student" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteStudent
                    , Attr.attribute "aria-label" "Close"
                    ]
                    [ svgIcon
                        [ Attr.attribute "viewBox" "0 0 24 24"
                        , Attr.attribute "width" "18"
                        , Attr.attribute "height" "18"
                        , Attr.attribute "fill" "none"
                        , Attr.attribute "stroke" "currentColor"
                        , Attr.attribute "stroke-width" "2"
                        , Attr.attribute "stroke-linecap" "round"
                        , Attr.attribute "stroke-linejoin" "round"
                        ]
                        [ svgPath "M18 6 6 18"
                        , svgPath "M6 6l12 12"
                        ]
                    ]
                ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text student.name ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteStudent
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteStudent
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
