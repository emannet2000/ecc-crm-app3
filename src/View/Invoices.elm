module View.Invoices exposing (deleteInvoiceConfirmModal, deletePaymentConfirmModal, invoiceDetailView, invoiceFormModal, invoicesView, paymentFormModal, refundConfirmModal)

{-| Invoices: list, detail (with payments panel), form + payment + refund modals.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import View.RecordPicker as RecordPicker
import Types exposing (..)
import View.Format exposing (formatCurrency, formatCurrencyWith)
import View.Helpers exposing (detailCard, detailEmpty, detailStat, infoRow, initials, paginationBar, svgIcon, svgPath)
import View.Icons exposing (iconBack, iconCalendar, iconDeals, iconEdit, iconMail, iconTasks, iconTrash, iconUserTiny)


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


invoiceRow : Invoice -> Html Msg
invoiceRow inv =
    tr [ class "contact-row", onClick (OpenedInvoiceDetail inv) ]
        [ td []
            [ div [ class "contact-name-cell" ]
                [ div [ class "contact-avatar" ] [ text (initials inv.clientName) ]
                , div [ class "contact-name-info" ]
                    [ span [ class "contact-name" ] [ text inv.invoiceNumber ]
                    , span [ class "contact-email" ] [ text inv.clientName ]
                    ]
                ]
            ]
        , td [] [ text inv.caseNumber ]
        , td [] [ text (formatCurrencyWith inv.currency inv.balance) ]
        , td [] [ milestoneBadge inv.paymentMilestone ]
        , td [ class "contact-actions-cell" ]
            [ button
                [ class "row-action"
                , type_ "button"
                , Attr.title "Edit"
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( OpenedEditInvoice inv, True ))
                ]
                [ iconEdit ]
            , button
                [ class "row-action row-action--danger"
                , type_ "button"
                , Attr.title "Delete"
                , Html.Events.stopPropagationOn "click"
                    (D.succeed ( RequestedDeleteInvoice inv, True ))
                ]
                [ iconTrash ]
            ]
        ]


invoicesSkeleton : Html Msg
invoicesSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search invoices…", disabled True ] [] ]
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


invoicesView : Model -> Html Msg
invoicesView model =
    case model.invoices of
        NotAsked ->
            invoicesSkeleton

        Loading ->
            invoicesSkeleton

        Failure msg ->
            div [ class "content__empty-block" ]
                [ text ("Could not load invoices: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

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
                            , Attr.attribute "aria-label" "Search invoices"
                            , placeholder "Search invoices…"
                            , value data.query
                            , onInput UpdatedInvoicesQuery
                            ]
                            []
                        ]
                    , button
                        [ class "ecc-btn ecc-btn--inline"
                        , type_ "button"
                        , onClick OpenedAddInvoice
                        ]
                        [ text "Add invoice" ]
                    ]
                , if List.isEmpty data.items then
                    div [ class "empty-state" ]
                        [ h3 [ class "empty-state__title" ]
                            [ text
                                (if isQueryEmpty then
                                    "No invoices yet"

                                 else
                                    "No invoices match your search"
                                )
                            ]
                        , p [ class "empty-state__desc" ]
                            [ text "Create an invoice to track fees and payments." ]
                        ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Invoice" ]
                                    , th [] [ text "Case" ]
                                    , th [] [ text "Balance" ]
                                    , th [] [ text "Milestone" ]
                                    , th [ class "th-actions" ] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map invoiceRow data.items)
                            ]
                        ]
                , paginationBar data.total data.offset data.limit InvoicesPageChanged
                ]


paymentItem : Payment -> Html Msg
paymentItem pmt =
    div [ class "activity-item" ]
        [ div [ class "activity-item__marker" ] []
        , div [ class "activity-item__body" ]
            [ div [ class "activity-item__meta" ]
                [ span [ class "activity-item__kind" ] [ text (formatCurrencyWith pmt.currency pmt.amount) ]
                , if String.isEmpty pmt.paidOn then
                    text ""

                  else
                    span [ class "activity-item__time" ] [ text pmt.paidOn ]
                , span [ class "activity-item__kind" ] [ text (if pmt.status == "pending_approval" then "Awaiting approval" else if pmt.status == "void" then "Rejected / void" else if not (String.isEmpty pmt.reviewedBy) then "Approved" else String.replace "_" " " pmt.status) ]
                , if String.isEmpty pmt.method then
                    text ""

                  else
                    span [ class "activity-item__by" ] [ text ("· " ++ pmt.method) ]
                ]
            , div [ class "activity-item__title-row" ]
                [ h4 [ class "activity-item__title" ]
                    [ text
                        (if String.isEmpty pmt.reference then
                            "Payment"

                         else
                            "Ref: " ++ pmt.reference
                        )
                    ]
                , if pmt.status == "pending_approval" || pmt.status == "void" || not (String.isEmpty pmt.reviewedBy) then
                    text ""

                  else
                    button
                    [ class "row-action row-action--danger row-action--tiny"
                    , type_ "button"
                    , Attr.title "Delete payment"
                    , onClick (RequestedDeletePayment pmt)
                    ]
                    [ iconTrash ]
                ]
            , if String.isEmpty pmt.notes then
                text ""

              else
                p [ class "activity-item__text" ] [ text pmt.notes ]
            ]
        ]


paymentsPanel : Model -> Html Msg
paymentsPanel model =
    case model.invoicePayments of
        NotAsked ->
            div [ class "activity-loading" ] [ text "Loading payments…" ]

        Loading ->
            div [ class "activity-loading" ] [ text "Loading payments…" ]

        Failure msg ->
            div [ class "activity-loading" ]
                [ text ("Could not load payments: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success [] ->
            detailEmpty
                iconDeals
                "No payments yet"
                "Record the first payment against this invoice."

        Success ps ->
            div [ class "activity-list" ] (List.map paymentItem ps)


invoiceDetailView : Model -> Invoice -> Html Msg
invoiceDetailView model inv =
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
            , onClick (NavigatedTo Invoices)
            ]
            [ iconBack
            , span [] [ text "Back to invoices" ]
            ]
        , a [ Attr.href "#record-tools?tab=workflows", Attr.target "_self", class "ecc-btn ecc-btn--ghost ecc-btn--inline" ] [ text "Tax, refunds & reminders" ]
        , a [ Attr.href ("/api/invoices/" ++ inv.id ++ "/pdf"), Attr.attribute "download" "invoice.pdf", class "ecc-btn ecc-btn--ghost ecc-btn--inline" ] [ text "Download PDF" ]
        , header [ class "detail-hero" ]
            [ div [ class "detail-hero__avatar detail-hero__avatar--deal" ]
                [ text (formatCurrencyWith inv.currency inv.balance) ]
            , div [ class "detail-hero__body" ]
                [ div [ class "detail-hero__title-row" ]
                    [ h1 [ class "detail-hero__name" ] [ text inv.invoiceNumber ]
                    , milestoneBadge inv.paymentMilestone
                    ]
                , p [ class "detail-hero__role" ]
                    [ text (inv.clientName ++ " · " ++ display inv.caseNumber) ]
                ]
            , div [ class "detail-hero__actions" ]
                [ button
                    [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , type_ "button"
                    , onClick (OpenedEditInvoice inv)
                    ]
                    [ iconEdit
                    , span [] [ text "Edit" ]
                    ]
                , button
                    [ class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , type_ "button"
                    , onClick (RequestedDeleteInvoice inv)
                    ]
                    [ iconTrash
                    , span [] [ text "Delete" ]
                    ]
                ]
            ]
        , div [ class "detail-stats" ]
            [ detailStat "Total fee" (formatCurrencyWith inv.currency inv.totalFee) "Base"
            , detailStat "Received" (formatCurrencyWith inv.currency inv.amountReceived) "Collected"
            , detailStat "Balance" (formatCurrencyWith inv.currency inv.balance) "Outstanding"
            , detailStat "Gov fee" (formatCurrencyWith inv.currency inv.governmentFee) "Pass-through"
            ]
        , node "crm-invoice-status" [ Attr.attribute "record-id" inv.id, id "receipt-approvals" ] []
        , div [ class "detail__grid" ]
            [ aside [ class "detail__sidebar" ]
                [ detailCard "Invoice info"
                    Nothing
                    (div [ class "info-list" ]
                        [ infoRow iconUserTiny "Client" inv.clientName
                        , infoRow iconTasks "Case" (display inv.caseNumber)
                        , infoRow iconDeals "School partner fee" (formatCurrencyWith inv.currency inv.schoolPartnerFee)
                        , infoRow iconDeals "Referral commission" (formatCurrencyWith inv.currency inv.referralCommission)
                        , infoRow iconDeals "Partner payable" (formatCurrencyWith inv.currency inv.partnerPayable)
                        , infoRow iconMail "Method" (display inv.paymentMethod)
                        , infoRow iconCalendar "Receipt #" (display inv.officialReceiptNumber)
                        , infoRow iconUserTiny "Approval note" (display inv.paymentApproval)
                        , infoRow iconCalendar "Created" (display inv.createdAt)
                        ]
                    )
                , detailCard "Refund"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , onClick (RequestedRefund inv)
                            , disabled (inv.refundStatus == "Requested")
                            ]
                            [ text
                                (if inv.refundStatus == "Requested" then
                                    "Refund requested"

                                 else
                                    "Request refund"
                                )
                            ]
                        )
                    )
                    (if String.isEmpty inv.refundStatus then
                        p [ class "detail-muted" ] [ text "No refund on file." ]

                     else
                        p [ class "detail-notes" ] [ text ("Status: " ++ inv.refundStatus) ]
                    )
                , detailCard "Notes"
                    Nothing
                    (if String.isEmpty inv.notes then
                        p [ class "detail-muted" ] [ text "No notes yet." ]

                     else
                        p [ class "detail-notes" ] [ text inv.notes ]
                    )
                ]
            , div [ class "detail__main" ]
                [ detailCard "Payments"
                    (Just
                        (button
                            [ class "detail-card__action"
                            , type_ "button"
                            , onClick OpenedPaymentForm
                            ]
                            [ text "Record payment" ]
                        )
                    )
                    (paymentsPanel model)
                ]
            ]
        ]


invoiceFormFieldError : String -> InvoiceForm -> Maybe String
invoiceFormFieldError field inv =
    inv.errors
        |> List.filter (\( f, _ ) -> f == field)
        |> List.head
        |> Maybe.map Tuple.second


invoiceRichField : InvoiceForm -> String -> String -> String -> Html Msg
invoiceRichField inv fieldId labelText inputType =
    let
        currentValue =
            case fieldId of
                "invoiceNumber" ->
                    inv.invoiceNumber

                "totalFee" ->
                    inv.totalFee

                "governmentFee" ->
                    inv.governmentFee

                "schoolPartnerFee" ->
                    inv.schoolPartnerFee

                "amountReceived" ->
                    inv.amountReceived

                "paymentMethod" ->
                    inv.paymentMethod

                "officialReceiptNumber" ->
                    inv.officialReceiptNumber

                "refundStatus" ->
                    inv.refundStatus

                "referralCommission" ->
                    inv.referralCommission

                "partnerPayable" ->
                    inv.partnerPayable

                "paymentApproval" ->
                    inv.paymentApproval

                _ ->
                    ""

        err =
            invoiceFormFieldError fieldId inv

        cls =
            case err of
                Just _ ->
                    "ecc-field ecc-field--error"

                Nothing ->
                    "ecc-field"
    in
    div [ class cls ]
        ([ input
            [ id ("inv-" ++ fieldId)
            , type_ inputType
            , placeholder " "
            , value currentValue
            , onInput (UpdatedInvoiceFormField fieldId)
            , disabled inv.submitting
            ]
            []
         , label [ for ("inv-" ++ fieldId) ] [ text labelText ]
         ]
            ++ (case err of
                    Just msg ->
                        [ p [ class "ecc-field__message" ] [ text msg ] ]

                    Nothing ->
                        []
               )
        )


invoiceClientSelect : Model -> InvoiceForm -> Html Msg
invoiceClientSelect _ inv =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Client" ]
        , RecordPicker.view "inv-clientId" "contacts" inv.clientId "Find client" inv.submitting (UpdatedInvoiceFormField "clientId")
        , case invoiceFormFieldError "clientId" inv of
            Just message ->
                p [ class "ecc-field__message" ] [ text message ]
            Nothing ->
                text ""
        ]


invoiceCaseSelect : Model -> InvoiceForm -> Html Msg
invoiceCaseSelect _ inv =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Application" ]
        , RecordPicker.view "inv-caseId" "cases" inv.caseId "Find application" inv.submitting (UpdatedInvoiceFormField "caseId")
        ]


milestonePills : InvoiceForm -> Html Msg
milestonePills inv =
    div [ class "ecc-field" ]
        [ span [ class "ecc-field__label" ] [ text "Payment milestone" ]
        , div [ class "stage-pills" ]
            (List.map
                (\m ->
                    button
                        [ type_ "button"
                        , class
                            (if inv.paymentMilestone == m then
                                "stage-pill stage-pill--active"

                             else
                                "stage-pill"
                            )
                        , onClick (UpdatedInvoiceFormField "paymentMilestone" m)
                        , disabled inv.submitting
                        ]
                        [ text m ]
                )
                paymentMilestones
            )
        ]


invoiceFormView : Model -> InvoiceForm -> Bool -> Html Msg
invoiceFormView model inv isEdit =
    let
        formError =
            invoiceFormFieldError "form" inv

        submitLabel =
            if inv.submitting then
                "Saving…"

            else if isEdit then
                "Save changes"

            else
                "Save invoice"
    in
    form [ onSubmit SubmittedInvoiceForm, Attr.novalidate True ]
        [ case formError of
            Just msg ->
                div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

            Nothing ->
                text ""
        , div [ class "form-grid" ]
            [ invoiceRichField inv "invoiceNumber" "Invoice number (auto if empty)" "text"
            , invoiceClientSelect model inv
            , invoiceCaseSelect model inv
            , invoiceRichField inv "totalFee" "Total fee" "number"
            , invoiceRichField inv "governmentFee" "Government fee" "number"
            , invoiceRichField inv "schoolPartnerFee" "School partner fee" "number"
            , invoiceRichField inv "amountReceived" "Amount received" "number"
            , invoiceRichField inv "paymentMethod" "Payment method" "text"
            , invoiceRichField inv "officialReceiptNumber" "Official receipt #" "text"
            , invoiceRichField inv "refundStatus" "Refund status" "text"
            , invoiceRichField inv "referralCommission" "Referral commission" "number"
            , invoiceRichField inv "partnerPayable" "Partner payable" "number"
            , invoiceRichField inv "paymentApproval" "Approval note (legacy)" "text"
            ]
        , milestonePills inv
        , div [ class "ecc-field ecc-field--notes" ]
            [ span [ class "ecc-field__label" ] [ text "Notes" ]
            , textarea
                [ id "inv-notes"
                , placeholder "Invoice notes…"
                , value inv.notes
                , onInput (UpdatedInvoiceFormField "notes")
                , disabled inv.submitting
                , Attr.rows 3
                ]
                []
            ]
        , div [ class "modal__actions" ]
            [ button
                [ type_ "button"
                , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , onClick RequestedCloseInvoiceForm
                , disabled inv.submitting
                ]
                [ text "Cancel" ]
            , button
                [ type_ "submit"
                , class "ecc-btn ecc-btn--inline"
                , disabled (inv.submitting || not inv.dirty)
                ]
                [ text submitLabel ]
            ]
        ]


invoiceFormModal : Model -> InvoiceForm -> Html Msg
invoiceFormModal model inv =
    let
        isEdit =
            model.editingInvoiceId /= Nothing

        titleText =
            if isEdit then
                "Edit invoice"

            else
                "Add invoice"
    in
    div [ class "modal-backdrop", onClick RequestedCloseInvoiceForm ]
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
                    , onClick RequestedCloseInvoiceForm
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
            , if inv.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ]
                        [ text "Discard your changes?" ]
                    , div [ class "modal__actions" ]
                        [ button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                            , onClick CancelledCloseInvoiceForm
                            ]
                            [ text "Keep editing" ]
                        , button
                            [ type_ "button"
                            , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                            , onClick ConfirmedCloseInvoiceForm
                            ]
                            [ text "Discard" ]
                        ]
                    ]

              else
                invoiceFormView model inv isEdit
            ]
        ]


deleteInvoiceConfirmModal : Invoice -> Html Msg
deleteInvoiceConfirmModal inv =
    div [ class "modal-backdrop", onClick CancelledDeleteInvoice ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete invoice"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete invoice" ] ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text inv.invoiceNumber ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeleteInvoice
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeleteInvoice
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]


paymentFormModal : PaymentForm -> Html Msg
paymentFormModal pf =
    let
        fieldErr field =
            pf.errors
                |> List.filter (\( f, _ ) -> f == field)
                |> List.head
                |> Maybe.map Tuple.second
    in
    div [ class "modal-backdrop", onClick ClosedPaymentForm ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Record payment"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Record payment" ] ]
            , form [ onSubmit SubmittedPaymentForm, Attr.novalidate True ]
                [ case fieldErr "form" of
                    Just msg ->
                        div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

                    Nothing ->
                        text ""
                , div [ class "form-grid" ]
                    [ div
                        [ class
                            (case fieldErr "amount" of
                                Just _ ->
                                    "ecc-field ecc-field--error"

                                Nothing ->
                                    "ecc-field"
                            )
                        ]
                        [ input
                            [ id "pf-amount"
                            , type_ "number"
                            , placeholder " "
                            , value pf.amount
                            , onInput (UpdatedPaymentFormField "amount")
                            , disabled pf.submitting
                            , Attr.autofocus True
                            ]
                            []
                        , label [ for "pf-amount" ] [ text "Amount" ]
                        , case fieldErr "amount" of
                            Just msg ->
                                p [ class "ecc-field__message" ] [ text msg ]

                            Nothing ->
                                text ""
                        ]
                    , div [ class "ecc-field" ]
                        [ input
                            [ id "pf-paidOn"
                            , type_ "date"
                            , placeholder " "
                            , value pf.paidOn
                            , onInput (UpdatedPaymentFormField "paidOn")
                            , disabled pf.submitting
                            ]
                            []
                        , label [ for "pf-paidOn" ] [ text "Paid on" ]
                        ]
                    , div [ class "ecc-field" ]
                        [ input
                            [ id "pf-method"
                            , type_ "text"
                            , placeholder " "
                            , value pf.method
                            , onInput (UpdatedPaymentFormField "method")
                            , disabled pf.submitting
                            ]
                            []
                        , label [ for "pf-method" ] [ text "Method" ]
                        ]
                    , div [ class "ecc-field" ]
                        [ input
                            [ id "pf-reference"
                            , type_ "text"
                            , placeholder " "
                            , value pf.reference
                            , onInput (UpdatedPaymentFormField "reference")
                            , disabled pf.submitting
                            ]
                            []
                        , label [ for "pf-reference" ] [ text "Reference" ]
                        ]
                    ]
                , div [ class "ecc-field ecc-field--notes" ]
                    [ span [ class "ecc-field__label" ] [ text "Notes" ]
                    , textarea
                        [ value pf.notes
                        , onInput (UpdatedPaymentFormField "notes")
                        , disabled pf.submitting
                        , Attr.rows 2
                        ]
                        []
                    ]
                , label [ class "receipt-choice" ]
                    [ input [ id "pf-requireApproval", type_ "checkbox", Attr.checked pf.requireApproval, disabled pf.submitting, Html.Events.onCheck (\checked -> UpdatedPaymentFormField "requireApproval" (if checked then "true" else "false")), Html.Events.stopPropagationOn "click" (D.map (\checked -> ( UpdatedPaymentFormField "requireApproval" (if checked then "true" else "false"), True )) (D.at [ "target", "checked" ] D.bool)) ] []
                    , text "Request manager approval; keep this receipt out of the posted balance until approved."
                    ]
                , case fieldErr "duplicate" of
                    Just _ ->
                        label [ class "receipt-choice" ]
                            [ input [ id "pf-confirmDuplicate", type_ "checkbox", Attr.checked pf.confirmDuplicate, disabled pf.submitting, Html.Events.onCheck (\checked -> UpdatedPaymentFormField "confirmDuplicate" (if checked then "true" else "false")), Html.Events.stopPropagationOn "click" (D.map (\checked -> ( UpdatedPaymentFormField "confirmDuplicate" (if checked then "true" else "false"), True )) (D.at [ "target", "checked" ] D.bool)) ] []
                            , text "I checked payment history and confirm this is a separate receipt."
                            ]

                    Nothing ->
                        text ""
                , div [ class "modal__actions" ]
                    [ button
                        [ type_ "button"
                        , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                        , onClick ClosedPaymentForm
                        , disabled pf.submitting
                        ]
                        [ text "Cancel" ]
                    , button
                        [ type_ "submit"
                        , class "ecc-btn ecc-btn--inline"
                        , disabled pf.submitting
                        ]
                        [ text "Save payment" ]
                    ]
                ]
            ]
        ]


deletePaymentConfirmModal : Payment -> Html Msg
deletePaymentConfirmModal pmt =
    div [ class "modal-backdrop", onClick CancelledDeletePayment ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Delete payment"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete payment" ] ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete payment of "
                , strong [] [ text (formatCurrencyWith pmt.currency pmt.amount) ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledDeletePayment
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedDeletePayment
                    ]
                    [ text "Delete" ]
                ]
            ]
        ]


refundConfirmModal : Invoice -> Html Msg
refundConfirmModal inv =
    div [ class "modal-backdrop", onClick CancelledRefund ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Attr.attribute "aria-modal" "true"
            , Attr.attribute "aria-label" "Request refund"
            , Html.Events.stopPropagationOn "click"
                (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Request refund" ] ]
            , p [ class "modal__confirm-text" ]
                [ text "Request a refund for "
                , strong [] [ text inv.invoiceNumber ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                    , onClick CancelledRefund
                    ]
                    [ text "Cancel" ]
                , button
                    [ type_ "button"
                    , class "ecc-btn ecc-btn--danger ecc-btn--inline"
                    , onClick ConfirmedRefund
                    ]
                    [ text "Request refund" ]
                ]
            ]
        ]
