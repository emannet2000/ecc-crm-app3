module View.Leads exposing (deleteLeadConfirmModal, leadDetailView, leadFormModal, leadsView)

{-| Leads: list, detail, form.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Svg
import Types exposing (..)
import View.Helpers exposing (detailCard, detailStat, infoRow, initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconContacts, iconDeals, iconEdit, iconMail, iconPhone, iconPin, iconTasks, iconTrash, iconUserTiny)


leadSourceBadge : String -> Html Msg
leadSourceBadge source =
    let
        cls =
            case String.toLower source of
                "facebook" ->
                    "badge badge--info"

                "instagram" ->
                    "badge badge--proposal"

                "whatsapp" ->
                    "badge badge--success"

                "website" ->
                    "badge badge--qualified"

                "referral" ->
                    "badge badge--customer"

                _ ->
                    "badge badge--muted"
    in
    span [ class cls ] [ text source ]


leadStatusBadge : String -> Html Msg
leadStatusBadge status =
    let
        cls =
            case String.toLower status of
                "new" ->
                    "badge badge--info"

                "contacted" ->
                    "badge badge--qualified"

                "qualified" ->
                    "badge badge--proposal"

                "consultation booked" ->
                    "badge badge--negotiation"

                "proposal sent" ->
                    "badge badge--negotiation"

                "converted" ->
                    "badge badge--success"

                "closed/lost" ->
                    "badge badge--lost"

                _ ->
                    "badge"
    in
    span [ class cls ] [ text status ]


leadRow : Lead -> Html Msg
leadRow l =
    tr [ class "contact-row", onClick (OpenedLeadDetail l) ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ div [ class "contact-avatar" ] [ text (initials l.name) ]
                , div [ class "contact-name-info" ]
                    [ span [ class "contact-name" ] [ text l.name ]
                    , span [ class "contact-email" ]
                        [ text
                            (if String.isEmpty l.email then
                                l.phone

                             else
                                l.email
                            )
                        ]
                    ]
                ]
            ]
        , td [] [ text l.interestedCountry ]
        , td [] [ leadSourceBadge l.source ]
        , td [] [ leadStatusBadge l.status ]
        , td [ class "contact-date" ]
            [ text
                (if String.isEmpty l.followUpDate then
                    "—"

                 else
                    l.followUpDate
                )
            ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Attr.attribute "aria-label" ("Edit " ++ l.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditLead l, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Attr.attribute "aria-label" ("Delete " ++ l.name)
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteLead l, True ))
                ]
                [ iconTrash ]
            ]
        ]


leadsSkeleton : Html Msg
leadsSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search leads…", disabled True ] [] ]
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


leadsView : Model -> Html Msg
leadsView model =
    case model.leads of
        NotAsked ->
            leadsSkeleton

        Loading ->
            leadsSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load leads: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

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
                            , Attr.attribute "aria-label" "Search leads"
                            , placeholder "Search leads…"
                            , value data.query
                            , onInput UpdatedLeadsQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddLead
                        ]
                        [ text "Add lead" ]
                    ]
                , if List.isEmpty filtered then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No leads yet"

                                 else
                                    "No leads match your search"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text
                                (if isQueryEmpty then
                                    "Add your first lead to start tracking inquiries."

                                 else
                                    "Try a different search term."
                                )
                            ]
                        , if isQueryEmpty then
                            div [ class "empty-state__action" ]
                                [ button
                                    [ class "ecc-btn ecc-btn--inline"
                                    , type_ "button"
                                    , onClick OpenedAddLead
                                    ]
                                    [ text "Add your first lead" ]
                                ]

                          else
                            text ""
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Lead" ]
                                    , th [] [ text "Destination" ]
                                    , th [] [ text "Source" ]
                                    , th [] [ text "Status" ]
                                    , th [] [ text "Follow-up" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map leadRow filtered)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit LeadsPageChanged
                ]


leadDetailView : Model -> Lead -> Html Msg
leadDetailView _ l =
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
            , onClick (NavigatedTo Leads)
            ]
            [ iconBack
            , span [] [ text "Back to leads" ]
            ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar" ] [ text (initials l.name) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text l.name ]
                    , leadStatusBadge l.status
                    ]
                , p [ class "detail-hero__role" ]
                    [ text
                        (if String.isEmpty l.interestedService then
                            display l.interestedCountry

                         else
                            l.interestedService
                        )
                    ]
                , div [ class "detail-hero__contact" ]
                    [ if String.isEmpty l.email then
                        text ""

                      else
                        a
                            [ class "detail-hero__chip"
                            , Attr.href ("mailto:" ++ l.email)
                            ]
                            [ iconMail
                            , span [] [ text l.email ]
                            ]
                    , if String.isEmpty l.phone then
                        text ""

                      else
                        a
                            [ class "detail-hero__chip"
                            , Attr.href ("tel:" ++ l.phone)
                            ]
                            [ iconPhone
                            , span [] [ text l.phone ]
                            ]
                    ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditLead l)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteLead l)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Source" l.source "Channel"
            , detailStat "Destination" (display l.interestedCountry) "Interested"
            , detailStat "Follow-up" (display l.followUpDate) "Next touch"
            , detailStat "Assigned to" (display l.assignedTo) "Consultant"
            ]
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "About"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconUserTiny "Lead number" (display l.leadNumber)
                        , infoRow iconMail "Email" (display l.email)
                        , infoRow iconPhone "Phone" (display l.phone)
                        , infoRow iconPin "Nationality" (display l.nationality)
                        , infoRow iconPin "Currently in" (display l.currentCountry)
                        , infoRow iconPin "Interested in" (display l.interestedCountry)
                        , infoRow iconDeals "Service" (display l.interestedService)
                        , infoRow iconCalendar "Follow-up" (display l.followUpDate)
                        ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty l.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text l.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Conversion"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconTasks "Status" l.status
                        , infoRow iconContacts "Assigned to" (display l.assignedTo)
                        , infoRow iconCalendar "Created" (display l.createdAt)
                        ]
                    )
                ]
            ]
        ]


leadFormFieldError : String -> LeadForm -> Maybe String
leadFormFieldError field lf =
    lf.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


leadRichField : LeadForm -> String -> String -> String -> Html Msg
leadRichField lf fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "leadNumber" ->
                    lf.leadNumber

                "name" ->
                    lf.name

                "email" ->
                    lf.email

                "phone" ->
                    lf.phone

                "nationality" ->
                    lf.nationality

                "currentCountry" ->
                    lf.currentCountry

                "interestedCountry" ->
                    lf.interestedCountry

                "interestedService" ->
                    lf.interestedService

                "assignedTo" ->
                    lf.assignedTo

                "followUpDate" ->
                    lf.followUpDate

                _ ->
                    ""

        err =
            leadFormFieldError fieldId lf

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("lf-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedLeadFormField fieldId)
            , disabled lf.submitting
            ]
            []
         , label [ for ("lf-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


leadStatusPills : String -> String -> List String -> (String -> Msg) -> Bool -> Html Msg
leadStatusPills label current options toMsg isDisabled =
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


leadFormView : Model -> LeadForm -> Bool -> Html Msg
leadFormView model lf isEdit =
    let
        formError =
            leadFormFieldError "form" lf

        submitLabel =
            if lf.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save lead"
    in
    form [ onSubmit SubmittedLeadForm, Attr.novalidate True ]
        [ case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ leadRichField lf "name" "Lead full name" "text"
            , leadRichField lf "leadNumber" "Lead number" "text"
            , leadRichField lf "email" "Email" "email"
            , leadRichField lf "phone" "Phone" "tel"
            , leadRichField lf "nationality" "Nationality (e.g. PH)" "text"
            , leadRichField lf "currentCountry" "Currently in (e.g. PH)" "text"
            , leadRichField lf "interestedCountry" "Interested country (e.g. CA)" "text"
            , leadRichField lf "interestedService" "Interested service" "text"
            , leadRichField lf "assignedTo" "Assigned consultant" "text"
            , leadRichField lf "followUpDate" "Follow-up date" "date"
            ]
        , leadStatusPills "Source"
            lf.source
            leadSources
            (UpdatedLeadFormField "source")
            lf.submitting
        , leadStatusPills "Status"
            lf.status
            leadStatuses
            (UpdatedLeadFormField "status")
            lf.submitting
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "lf-notes"
                , placeholder "Notes and communication history…"
                , value lf.notes
                , onInput (UpdatedLeadFormField "notes")
                , disabled lf.submitting
                , Attr.rows 4
                ]
                []
            ]
        , node "crm-form-quality"
            [ Attr.attribute "entity" "leads"
            , Attr.attribute "name" lf.name
            , Attr.attribute "email" lf.email
            , Attr.attribute "phone" lf.phone
            , Attr.attribute "followUpDate" lf.followUpDate
            , Attr.attribute "status" lf.status
            , Attr.attribute "record-id" (Maybe.withDefault "" model.editingLeadId)
            ]
            []
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseLeadForm
                , disabled lf.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (lf.submitting || not lf.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


leadFormModal : Model -> LeadForm -> Html Msg
leadFormModal model lf =
    let
        isEdit =
            model.editingLeadId /= Nothing

        titleText =
            if isEdit then
                "Edit lead"

            else
                "Add lead"
    in
    div [ class "modal-backdrop", onClick RequestedCloseLeadForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" titleText
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text titleText ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick RequestedCloseLeadForm
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
            , if lf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes? They won't be saved." ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseLeadForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseLeadForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                leadFormView model lf isEdit
            ]
        ]


deleteLeadConfirmModal : Lead -> Html Msg
deleteLeadConfirmModal lead =
    div [ class "modal-backdrop", onClick CancelledDeleteLead ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete lead"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete lead" ]
                , button
                    [ class "modal__close"
                    , type_ "button"
                    , onClick CancelledDeleteLead
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
                , strong [] [ text lead.name ]
                , text "? This cannot be undone."
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteLead
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteLead
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]
