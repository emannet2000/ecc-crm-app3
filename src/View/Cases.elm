module View.Cases exposing (caseDetailView, caseFormModal, casesView, deleteCaseConfirmModal, deleteDocumentConfirmModal, documentFormModal)

{-| Cases: list, detail (with documents panel), form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (checked, class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import View.RecordPicker as RecordPicker
import Types exposing (..)
import View.Dashboard exposing (ActivityEntry, PriorityAlert, activityPanel, alertsPanel, documentCorrectionAlert, documentEntry, expiredDocumentAlert, urgentCaseAlert)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials, onCheck, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconPin, iconTasks, iconTrash, iconUserTiny)
import View.Workflow exposing (workflowView)


priorityBadge : String -> Html Msg
priorityBadge p =
    let
        cls =
            case String.toLower p of
                "urgent" ->
                    "badge badge--lost"

                "high" ->
                    "badge badge--proposal"

                "medium" ->
                    "badge badge--info"

                _ ->
                    "badge badge--muted"
    in
    span [ class cls ] [ text p ]


stageBadge : String -> Html Msg
stageBadge s =
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


caseRow : Case -> Html Msg
caseRow c =
    tr [ class "contact-row", onClick (OpenedCaseDetail c) ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ div [ class "contact-avatar" ] [ text (initials c.studentName) ]
                , div [ class "contact-name-info" ]
                    [ span [ class "contact-name" ] [ text c.studentName ]
                    , span [ class "contact-email" ] [ text c.caseNumber ]
                    ]
                ]
            ]
        , td [] [ text c.serviceCategory ]
        , td [] [ text c.destinationCountry ]
        , td [] [ stageBadge c.currentStage ]
        , td [] [ priorityBadge c.priority ]
        , td [ class "contact-date" ]
            [ text
                (if String.isEmpty c.nextDeadline then
                    "—"

                 else
                    c.nextDeadline
                )
            ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ c.caseNumber)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditCase c, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ c.caseNumber)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteCase c, True ))
                ]
                [ iconTrash ]
            ]
        ]


casesSkeleton : Html Msg
casesSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search cases…", disabled True ] [] ]
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


casesView : Model -> Html Msg
casesView model =
    case model.cases of
        NotAsked ->
            casesSkeleton

        Loading ->
            casesSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load cases: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            let
                isQueryEmpty =
                    String.isEmpty (String.trim data.query)
            in
            div []
                [ div [ class "page-toolbar" ]
                    [ div [ class "page-toolbar__search" ]
                        [ input
                            [ type_ "search"
                            , Attr.attribute "aria-label" "Search cases"
                            , placeholder "Search cases…"
                            , value data.query
                            , onInput UpdatedCasesQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddCase
                        ]
                        [ text "Add case" ]
                    ]
                , div [ class "stage-pills", Attr.style "margin-bottom" "12px" ]
                    (button
                        [ type_ "button"
                        , class
                            (if data.stageFilter == "" then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedCasesStageFilter "")
                        ]
                        [ text "All" ]
                        :: List.map
                            (\s ->
                                button
                                    [ type_ "button"
                                    , class
                                        (if data.stageFilter == s then
                                            "stage-pill stage-pill--active"

                                         else
                                            "stage-pill"
                                        )
                                    , onClick (UpdatedCasesStageFilter s)
                                    ]
                                    [ text s ]
                            )
                            caseStages
                    )
                , if List.isEmpty data.items then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty && String.isEmpty data.stageFilter then
                                    "No cases yet"

                                 else
                                    "No cases match"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty && String.isEmpty data.stageFilter then
                                    "Add your first case to track submissions and visa progress."

                                 else
                                    "Try a different search or clear the stage filter."
                                )
                            ]
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Student" ]
                                    , th [] [ text "Service" ]
                                    , th [] [ text "Destination" ]
                                    , th [] [ text "Stage" ]
                                    , th [] [ text "Priority" ]
                                    , th [] [ text "Next deadline" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map caseRow data.items)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit CasesPageChanged
                ]


documentItem : Document -> Html Msg
documentItem d =
    div [ class "activity-item" ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__meta" ]
                [ docStatusBadge d.status
                , if String.isEmpty d.dateRequested then
                    text ""

                  else
                    span [ class "activity-item__time" ]
                        [ text ("Requested " ++ d.dateRequested) ]
                , if String.isEmpty d.verificationDate then
                    text ""

                  else
                    span [ class "activity-item__by" ]
                        [ text ("· Verified " ++ d.verificationDate) ]
                ]
            , div [ class "activity-item__title-row" ]
                [ a [ Attr.href ("#record-tools?tab=documents&document=" ++ d.id), Attr.target "_self" ] [ text "Upload / versions" ]
                , h4 [ class "activity-item__title" ] [ text d.docName ]
                , div [ class "row-action-group" ]
                    [ button
                        [ class "row-action"
                        , type_ "button"
                        , Attr.title "Edit"
                        , onClick (OpenedEditDocument d)
                        ]
                        [ iconEdit ]
                    , button
                        [ class "row-action row-action--danger"
                        , type_ "button"
                        , Attr.title "Delete"
                        , onClick (RequestedDeleteDocument d)
                        ]
                        [ iconTrash ]
                    ]
                ]
            , div [ class "stage-pills", Attr.style "margin-top" "8px" ]
                (List.map
                    (\s ->
                        button
                            [ type_ "button"
                            , class
                                (if d.status == s then
                                    "stage-pill stage-pill--active"

                                 else
                                    "stage-pill"
                                )
                            , onClick (UpdatedDocumentStatus d s)
                            ]
                            [ text s ]
                    )
                    documentStatuses
                )
            , if String.isEmpty d.notes then
                text ""

              else
                p [ class "activity-item__text" ] [ text d.notes ]
            ]
        ]


documentsPanel : Model -> Case -> Html Msg
documentsPanel model _ =
    case model.caseDocuments of
        NotAsked ->
            div [ class "activity-loading" ] [ text "Loading documents…" ]

        Loading ->
            div [ class "activity-loading" ] [ text "Loading documents…" ]

        Failure msg ->
            div [ class "activity-loading" ]
                [ text ("Could not load documents: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success [] ->
            detailEmpty
                iconTasks
                "No documents yet"
                "Add the first document for this case to start tracking requirements."

        Success docs ->
            div [ class "activity-list" ]
                (List.map documentItem docs)


caseAlerts : Model -> Case -> List PriorityAlert
caseAlerts model c =
    let
        fromDocs =
            case model.caseDocuments of
                Success docs ->
                    let
                        corrections =
                            docs
                                |> List.filter (\d -> d.status == "Correction Required")
                                |> List.map documentCorrectionAlert

                        expired =
                            docs
                                |> List.filter (\d -> d.status == "Expired")
                                |> List.map expiredDocumentAlert
                    in
                    corrections ++ expired

                _ ->
                    []

        selfAlert =
            if c.priority == "Urgent" then
                [ urgentCaseAlert c ]

            else
                []
    in
    fromDocs ++ selfAlert


caseActivity : Model -> Case -> List ActivityEntry
caseActivity model _ =
    case model.caseDocuments of
        Success docs ->
            List.map documentEntry docs

        _ ->
            []


caseDetailView : Model -> Case -> Html Msg
caseDetailView model c =
    let
        display v =
            if String.isEmpty v then
                "—"

            else
                v
    in
    div [ class "detail" ]
        [ button
            [ class "detail__back"
            , type_ "button"
            , onClick (NavigatedTo Cases)
            ]
            [ iconBack
            , span [] [ text "Back to cases" ]
            ]
        , a [ Attr.href "#record-tools?tab=workflows", Attr.target "_self", class "ecc-btn ecc-btn--ghost ecc-btn--inline" ] [ text "Checklist, SLA & linked cases" ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar" ] [ text (initials c.studentName) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text c.caseNumber ]
                    , stageBadge c.currentStage
                    , priorityBadge c.priority
                    ]
                , p [ class "detail-hero__role" ]
                    [ text (display c.studentName ++ " · " ++ c.serviceCategory) ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditCase c)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteCase c)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , node "crm-application-blockers" [ Attr.attribute "record-id" c.id ] []
        , node "crm-case-handoff" [ Attr.attribute "record-id" c.id, id "handoff" ] []
        , div [ class "detail-stats" ]
            [ detailStat "Destination" (display c.destinationCountry) "Country"
            , detailStat "Visa type" (display c.visaType) "Category"
            , detailStat "Next deadline" (display c.nextDeadline) "Target"
            , detailStat "Assigned" (display c.assignedOfficer) "Officer"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "Case info"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconUserTiny "Student" (display c.studentName)
                        , infoRow iconUserTiny "Contact" (display c.clientName)
                        , infoRow iconTasks "Service" (display c.serviceCategory)
                        , infoRow iconPin "Destination" (display c.destinationCountry)
                        , infoRow iconDeals "Visa type" (display c.visaType)
                        , infoRow iconContacts "School/Employer" (display c.schoolOrEmployer)
                        , infoRow iconUserTiny "Adviser" (display c.externalAdviser)
                        , infoRow iconCalendar "Opened" (display c.dateOpened)
                        , infoRow iconCalendar "Target submission" (display c.targetSubmission)
                        , infoRow iconCalendar "Actual submission" (display c.actualSubmission)
                        , infoRow iconTasks "Gov reference" (display c.governmentRef)
                        ]
                    )
                , detailCard "Workflow Progress"
                    Nothing
                    (workflowView caseStages c.currentStage)
                , detailCard "Change stage"
                    Nothing
                    (div [ class "stage-pills stage-pills--detail" ]
                        (List.map
                            (\s ->
                                button
                                    [ type_ "button"
                                    , class
                                        (if c.currentStage == s then
                                            "stage-pill stage-pill--active"

                                         else
                                            "stage-pill"
                                        )
                                    , onClick (OpenedEditCase { c | currentStage = s })
                                    ]
                                    [ text s ]
                            )
                            caseStages
                        )
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty c.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text c.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Documents"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , onClick OpenedAddDocument
                            ]
                            [ text "Add document" ]
                        )
                    )
                    (documentsPanel model c)
                , detailCard "Next action"
                    Nothing
                    (if String.isEmpty c.nextAction then
                        p [ class "detail-muted" ] [ text "No next action set." ]

                     else
                        div [ class "info-list" ]
                            [ infoRow iconTasks "Action" c.nextAction
                            , infoRow iconCalendar "Deadline" (display c.nextDeadline)
                            ]
                    )
                ]
            ]
        , div [ class "bottom-row" ]
            [ alertsPanel (caseAlerts model c)
            , activityPanel (caseActivity model c)
            ]
        ]


caseFormFieldError : String -> CaseForm -> Maybe String
caseFormFieldError field cf =
    cf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


caseRichField : CaseForm -> String -> String -> String -> Html Msg
caseRichField cf fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "caseNumber" ->
                    cf.caseNumber

                "serviceCategory" ->
                    cf.serviceCategory

                "destinationCountry" ->
                    cf.destinationCountry

                "visaType" ->
                    cf.visaType

                "schoolOrEmployer" ->
                    cf.schoolOrEmployer

                "assignedOfficer" ->
                    cf.assignedOfficer

                "externalAdviser" ->
                    cf.externalAdviser

                "dateOpened" ->
                    cf.dateOpened

                "targetSubmission" ->
                    cf.targetSubmission

                "actualSubmission" ->
                    cf.actualSubmission

                "governmentRef" ->
                    cf.governmentRef

                "nextAction" ->
                    cf.nextAction

                "nextDeadline" ->
                    cf.nextDeadline

                "result" ->
                    cf.result

                "closureDate" ->
                    cf.closureDate

                _ ->
                    ""

        err =
            caseFormFieldError fieldId cf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("cf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedCaseFormField fieldId)
            , disabled cf.submitting
            ]
            []
         , label [ for ("cf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


caseStudentSelect : Model -> CaseForm -> Html Msg
caseStudentSelect _ cf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Student" ]
        , RecordPicker.view "cf-studentId" "students" cf.studentId "Find student" cf.submitting (UpdatedCaseFormField "studentId")
        ]


caseClientSelect : Model -> CaseForm -> Html Msg
caseClientSelect _ cf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Contact" ]
        , RecordPicker.view "cf-clientId" "contacts" cf.clientId "Find contact" cf.submitting (UpdatedCaseFormField "clientId")
        , case caseFormFieldError "clientId" cf of
            Just message ->
                p [ class "ecc-field__message" ] [ text message ]
            Nothing ->
                text ""
        ]


caseStagePills : CaseForm -> Html Msg
caseStagePills cf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Stage" ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    button
                        [ type_ "button"
                        , class
                            (if cf.currentStage == s then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedCaseFormField "currentStage" s)
                        , disabled cf.submitting
                        ]
                        [ text s ]
                )
                caseStages
            )
        ]


casePriorityPills : CaseForm -> Html Msg
casePriorityPills cf =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Priority" ]
        , div [ class "stage-pills" ]
            (List.map
                (\p ->
                    button
                        [ type_ "button"
                        , class
                            (if cf.priority == p then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedCaseFormField "priority" p)
                        , disabled cf.submitting
                        ]
                        [ text p ]
                )
                casePriorities
            )
        ]


caseFormView : Model -> CaseForm -> Bool -> Html Msg
caseFormView model cf isEdit =
    let
        formError =
            caseFormFieldError "form" cf

        submitLabel =
            if cf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save case"
    in
    form [ onSubmit SubmittedCaseForm, Attr.novalidate True ]
        [ case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ caseRichField cf "caseNumber" "Case number (auto if empty)" "text"
            , caseStudentSelect model cf
            , caseClientSelect model cf
            , caseRichField cf "serviceCategory" "Service category" "text"
            , caseRichField cf "destinationCountry" "Destination (e.g. CA)" "text"
            , caseRichField cf "visaType" "Visa type" "text"
            , caseRichField cf "schoolOrEmployer" "School or employer" "text"
            , caseRichField cf "assignedOfficer" "Assigned officer" "text"
            , caseRichField cf "externalAdviser" "External adviser" "text"
            , caseRichField cf "dateOpened" "Date opened" "date"
            , caseRichField cf "targetSubmission" "Target submission" "date"
            , caseRichField cf "actualSubmission" "Actual submission" "date"
            , caseRichField cf "governmentRef" "Government reference" "text"
            , caseRichField cf "nextAction" "Next action" "text"
            , caseRichField cf "nextDeadline" "Next deadline" "date"
            , caseRichField cf "result" "Result" "text"
            , caseRichField cf "closureDate" "Closure date" "date"
            ]
        , caseStagePills cf
        , casePriorityPills cf
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "cf-notes"
                , placeholder "Case notes…"
                , value cf.notes
                , onInput (UpdatedCaseFormField "notes")
                , disabled cf.submitting
                , Attr.rows 4
                ]
                []
            ]
        , node "crm-form-quality"
            [ Attr.attribute "entity" "cases"
            , Attr.attribute "studentId" cf.studentId
            , Attr.attribute "clientId" cf.clientId
            , Attr.attribute "nextAction" cf.nextAction
            , Attr.attribute "nextDeadline" cf.nextDeadline
            , Attr.attribute "currentStage" cf.currentStage
            , Attr.attribute "record-id" (Maybe.withDefault "" model.editingCaseId)
            ]
            []
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseCaseForm
                , disabled cf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (cf.submitting || not cf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


caseFormModal : Model -> CaseForm -> Html Msg
caseFormModal model cf =
    let
        isEdit =
            model.editingCaseId /= Nothing

        titleText =
            if isEdit then
                "Edit case"

            else
                "Add case"
    in
    div [ class "modal-backdrop", onClick RequestedCloseCaseForm ]
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
                    , onClick RequestedCloseCaseForm
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
            , if cf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes? They won't be saved." ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseCaseForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseCaseForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                caseFormView model cf isEdit
            ]
        ]


deleteCaseConfirmModal : Case -> Html Msg
deleteCaseConfirmModal c =
    div [ class "modal-backdrop", onClick CancelledDeleteCase ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete case"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete case" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteCase
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
                , strong [] [ text c.caseNumber ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteCase
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteCase
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]


documentFormFieldError : String -> DocumentForm -> Maybe String
documentFormFieldError field df =
    df.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


documentRichField : DocumentForm -> String -> String -> String -> Html Msg
documentRichField df fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "docName" ->
                    df.docName

                "dateRequested" ->
                    df.dateRequested

                "dateReceived" ->
                    df.dateReceived

                "expiryDate" ->
                    df.expiryDate

                "verifiedBy" ->
                    df.verifiedBy

                "verificationDate" ->
                    df.verificationDate

                "rejectionReason" ->
                    df.rejectionReason

                "filePath" ->
                    df.filePath

                _ ->
                    ""

        err =
            documentFormFieldError fieldId df

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("df-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedDocumentFormField fieldId)
            , disabled df.submitting
            ]
            []
         , label [ for ("df-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


documentStatusPills : DocumentForm -> Html Msg
documentStatusPills df =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Status" ]
        , div [ class "stage-pills" ]
            (List.map
                (\s ->
                    button
                        [ type_ "button"
                        , class
                            (if df.status == s then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedDocumentFormField "status" s)
                        , disabled df.submitting
                        ]
                        [ text s ]
                )
                documentStatuses
            )
        ]


documentFormView : DocumentForm -> Bool -> Html Msg
documentFormView df isEdit =
    let
        formError =
            documentFormFieldError "form" df

        submitLabel =
            if df.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save document"
    in
    form [ onSubmit SubmittedDocumentForm, Attr.novalidate True ]
        [ case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ documentRichField df "docName" "Document name" "text"
            , documentRichField df "dateRequested" "Date requested" "date"
            , documentRichField df "dateReceived" "Date received" "date"
            , documentRichField df "expiryDate" "Expiry date" "date"
            , documentRichField df "verifiedBy" "Verified by" "text"
            , documentRichField df "verificationDate" "Verification date" "date"
            , documentRichField df "filePath" "File path / URL" "text"
            ]
        , documentStatusPills df
        , div [ class "ecc-field" ]
            [ label [ class "ecc-checkbox" ]
                [ input
                    [ type_ "checkbox"
                    , checked df.required
                    , onCheck (ToggledDocumentBool "required")
                    , disabled df.submitting
                    ]
                    []
                , span [] [ text "Required" ]
                ]
            ]
        , div [ class "ecc-field" ]
            [ label [ class "ecc-checkbox" ]
                [ input
                    [ type_ "checkbox"
                    , checked df.translationRequired
                    , onCheck (ToggledDocumentBool "translationRequired")
                    , disabled df.submitting
                    ]
                    []
                , span [] [ text "Translation required" ]
                ]
            ]
        , div [ class "ecc-field" ]
            [ label [ class "ecc-checkbox" ]
                [ input
                    [ type_ "checkbox"
                    , checked df.legalizationRequired
                    , onCheck (ToggledDocumentBool "legalizationRequired")
                    , disabled df.submitting
                    ]
                    []
                , span [] [ text "Legalization required" ]
                ]
            ]
        , if df.status == "Correction Required" then
            documentRichField df "rejectionReason" "Correction reason" "text"

          else
            text ""
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "df-notes"
                , placeholder "Document notes…"
                , value df.notes
                , onInput (UpdatedDocumentFormField "notes")
                , disabled df.submitting
                , Attr.rows 3
                ]
                []
            ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseDocumentForm
                , disabled df.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (df.submitting || not df.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


documentFormModal : Model -> DocumentForm -> Html Msg
documentFormModal model df =
    let
        isEdit =
            model.editingDocumentId /= Nothing

        titleText =
            if isEdit then
                "Edit document"

            else
                "Add document"
    in
    div [ class "modal-backdrop", onClick RequestedCloseDocumentForm ]
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
                    , onClick RequestedCloseDocumentForm
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
            , if df.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes?" ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseDocumentForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseDocumentForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                documentFormView df isEdit
            ]
        ]


deleteDocumentConfirmModal : Document -> Html Msg
deleteDocumentConfirmModal d =
    div [ class "modal-backdrop", onClick CancelledDeleteDocument ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete document"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( NoOp, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete document" ] ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text d.docName ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteDocument
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteDocument
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
