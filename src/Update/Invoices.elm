module Update.Invoices exposing (update)

import Api
import Types exposing (..)
import Update.Loaders exposing (loadInvoices, pageSize)
import Update.Navigation


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotInvoices result ->
            case result of
                Ok ( items, total ) ->
                    let
                        prev =
                            case model.invoices of
                                Success d ->
                                    d

                                _ ->
                                    { items = [], query = "", total = 0, offset = 0, limit = pageSize }
                    in
                    ( { model
                        | invoices =
                            Success
                                { items = items
                                , query = prev.query
                                , total = total
                                , offset = prev.offset
                                , limit = pageSize
                                }
                      }
                    , Cmd.none
                    )

                Err message ->
                    ( { model | invoices = Failure message }, Cmd.none )

        UpdatedInvoicesQuery q ->
            let
                next =
                    case model.invoices of
                        Success data ->
                            Success { data | query = q, offset = 0 }

                        _ ->
                            Success { items = [], query = q, total = 0, offset = 0, limit = pageSize }
            in
            ( { model | invoices = next, pendingInvoicesQuery = Just q }, Cmd.none )

        FlushInvoicesSearch ->
            case ( model.token, model.pendingInvoicesQuery ) of
                ( Just t, Just q ) ->
                    ( { model | pendingInvoicesQuery = Nothing }
                    , Api.fetchInvoices t q pageSize 0 GotInvoices
                    )

                _ ->
                    ( { model | pendingInvoicesQuery = Nothing }, Cmd.none )

        InvoicesPageChanged newOffset ->
            case ( model.token, model.invoices ) of
                ( Just t, Success data ) ->
                    ( { model | invoices = Success { data | offset = newOffset } }
                    , Api.fetchInvoices t data.query pageSize newOffset GotInvoices
                    )

                _ ->
                    ( model, Cmd.none )

        OpenedAddInvoice ->
            ( { model | invoiceForm = Just emptyInvoiceForm, editingInvoiceId = Nothing, toast = Nothing }, Cmd.none )

        OpenedEditInvoice inv ->
            ( { model | invoiceForm = Just (invoiceToForm inv), editingInvoiceId = Just inv.id, toast = Nothing }, Cmd.none )

        OpenedInvoiceDetail inv ->
            Update.Navigation.update (NavigatedTo (InvoiceDetail inv.id))
                { model | viewingInvoice = Just inv, toast = Nothing }

        RequestedCloseInvoiceForm ->
            case ( model.invoiceForm, model.deletingInvoice ) of
                ( _, Just _ ) ->
                    ( { model | deletingInvoice = Nothing }, Cmd.none )

                ( Just inv, _ ) ->
                    if inv.dirty && not inv.submitting then
                        ( { model | invoiceForm = Just { inv | confirmDiscard = True } }, Cmd.none )

                    else
                        ( { model | invoiceForm = Nothing, editingInvoiceId = Nothing }, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        ConfirmedCloseInvoiceForm ->
            ( { model | invoiceForm = Nothing, editingInvoiceId = Nothing }, Cmd.none )

        CancelledCloseInvoiceForm ->
            case model.invoiceForm of
                Just inv ->
                    ( { model | invoiceForm = Just { inv | confirmDiscard = False } }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        UpdatedInvoiceFormField field value ->
            case model.invoiceForm of
                Just inv ->
                    let
                        updated =
                            case field of
                                "invoiceNumber" ->
                                    { inv | invoiceNumber = value }

                                "clientId" ->
                                    { inv | clientId = value }

                                "caseId" ->
                                    { inv | caseId = value }

                                "totalFee" ->
                                    { inv | totalFee = value }

                                "governmentFee" ->
                                    { inv | governmentFee = value }

                                "schoolPartnerFee" ->
                                    { inv | schoolPartnerFee = value }

                                "amountReceived" ->
                                    { inv | amountReceived = value }

                                "paymentMilestone" ->
                                    { inv | paymentMilestone = value }

                                "paymentMethod" ->
                                    { inv | paymentMethod = value }

                                "officialReceiptNumber" ->
                                    { inv | officialReceiptNumber = value }

                                "refundStatus" ->
                                    { inv | refundStatus = value }

                                "referralCommission" ->
                                    { inv | referralCommission = value }

                                "partnerPayable" ->
                                    { inv | partnerPayable = value }

                                "paymentApproval" ->
                                    { inv | paymentApproval = value }

                                "notes" ->
                                    { inv | notes = value }

                                _ ->
                                    inv
                    in
                    ( { model
                        | invoiceForm =
                            Just
                                { updated
                                    | dirty = True
                                    , errors = List.filter (\( f, _ ) -> f /= field) updated.errors
                                }
                      }
                    , Cmd.none
                    )

                Nothing ->
                    ( model, Cmd.none )

        SubmittedInvoiceForm ->
            case ( model.invoiceForm, model.token ) of
                ( Just inv, Just token ) ->
                    let
                        errs =
                            if String.isEmpty (String.trim inv.clientId) then
                                [ ( "clientId", "Client is required." ) ]

                            else
                                []
                    in
                    if not (List.isEmpty errs) then
                        ( { model | invoiceForm = Just { inv | errors = errs } }, Cmd.none )

                    else
                        let
                            cmd =
                                case model.editingInvoiceId of
                                    Just id ->
                                        Api.updateInvoice token id inv GotSavedInvoice

                                    Nothing ->
                                        Api.createInvoice token inv GotSavedInvoice
                        in
                        ( { model | invoiceForm = Just { inv | submitting = True, errors = [] } }, cmd )

                _ ->
                    ( model, Cmd.none )

        GotSavedInvoice result ->
            case result of
                Ok inv ->
                    let
                        verb =
                            if model.editingInvoiceId /= Nothing then
                                "Updated "

                            else
                                "Added "

                        fresh =
                            { model
                                | invoiceForm = Nothing
                                , editingInvoiceId = Nothing
                                , viewingInvoice =
                                    case model.route of
                                        InvoiceDetail _ ->
                                            Just inv

                                        _ ->
                                            model.viewingInvoice
                                , toast = Just (verb ++ inv.invoiceNumber)
                            }
                    in
                    ( fresh, Tuple.second (loadInvoices fresh) )

                Err (FieldErrors fields) ->
                    case model.invoiceForm of
                        Just inv ->
                            ( { model | invoiceForm = Just { inv | submitting = False, errors = fields } }, Cmd.none )

                        Nothing ->
                            ( model, Cmd.none )

                Err (GenericError message) ->
                    case model.invoiceForm of
                        Just inv ->
                            ( { model
                                | invoiceForm =
                                    Just { inv | submitting = False, errors = [ ( "form", message ) ] }
                              }
                            , Cmd.none
                            )

                        Nothing ->
                            ( model, Cmd.none )

        RequestedDeleteInvoice inv ->
            ( { model | deletingInvoice = Just inv }, Cmd.none )

        CancelledDeleteInvoice ->
            ( { model | deletingInvoice = Nothing }, Cmd.none )

        ConfirmedDeleteInvoice ->
            case ( model.deletingInvoice, model.token ) of
                ( Just inv, Just token ) ->
                    ( model, Api.deleteInvoice token inv.id GotDeletedInvoice )

                _ ->
                    ( model, Cmd.none )

        GotDeletedInvoice result ->
            case result of
                Ok _ ->
                    let
                        name =
                            case model.deletingInvoice of
                                Just inv ->
                                    inv.invoiceNumber

                                Nothing ->
                                    "invoice"

                        fresh =
                            { model
                                | deletingInvoice = Nothing
                                , viewingInvoice =
                                    case model.route of
                                        InvoiceDetail _ ->
                                            Nothing

                                        _ ->
                                            model.viewingInvoice
                                , invoicePayments = NotAsked
                                , route =
                                    case model.route of
                                        InvoiceDetail _ ->
                                            Invoices

                                        _ ->
                                            model.route
                                , toast = Just ("Deleted " ++ name)
                            }
                    in
                    ( fresh, Tuple.second (loadInvoices fresh) )

                Err message ->
                    ( { model
                        | deletingInvoice = Nothing
                        , toast = Just ("Delete failed: " ++ message)
                      }
                    , Cmd.none
                    )

        GotInvoicePayments result ->
            case result of
                Ok ps ->
                    ( { model | invoicePayments = Success ps }, Cmd.none )

                Err message ->
                    ( { model | invoicePayments = Failure message }, Cmd.none )

        OpenedPaymentForm ->
            ( { model | paymentForm = Just emptyPaymentForm, toast = Nothing }, Cmd.none )

        ClosedPaymentForm ->
            ( { model | paymentForm = Nothing }, Cmd.none )

        UpdatedPaymentFormField field value ->
            case model.paymentForm of
                Just pf ->
                    let
                        updated =
                            case field of
                                "requireApproval" ->
                                    { pf | requireApproval = value == "true" }

                                "confirmDuplicate" ->
                                    { pf | confirmDuplicate = value == "true" }

                                "amount" ->
                                    { pf | amount = value, confirmDuplicate = False }

                                "paidOn" ->
                                    { pf | paidOn = value, confirmDuplicate = False }

                                "method" ->
                                    { pf | method = value, confirmDuplicate = False }

                                "reference" ->
                                    { pf | reference = value, confirmDuplicate = False }

                                "notes" ->
                                    { pf | notes = value }

                                _ ->
                                    pf
                    in
                    ( { model
                        | paymentForm =
                            Just
                                { updated
                                    | errors = List.filter (\( f, _ ) -> f /= field) updated.errors
                                }
                      }
                    , Cmd.none
                    )

                Nothing ->
                    ( model, Cmd.none )

        SubmittedPaymentForm ->
            case ( model.paymentForm, model.token, model.viewingInvoice ) of
                ( Just pf, Just token, Just inv ) ->
                    case String.toFloat pf.amount of
                        Just amt ->
                            if amt <= 0 then
                                ( { model
                                    | paymentForm =
                                        Just { pf | errors = [ ( "amount", "Amount must be positive." ) ] }
                                  }
                                , Cmd.none
                                )

                            else
                                ( { model | paymentForm = Just { pf | submitting = True, errors = [] } }
                                , Api.createPayment token inv.id pf GotSavedPayment
                                )

                        Nothing ->
                            ( { model
                                | paymentForm =
                                    Just { pf | errors = [ ( "amount", "Enter a valid number." ) ] }
                              }
                            , Cmd.none
                            )

                _ ->
                    ( model, Cmd.none )

        GotSavedPayment result ->
            case result of
                Ok p ->
                    let
                        fresh =
                            { model
                                | paymentForm = Nothing
                                , toast = Just ((if p.status == "pending_approval" then "Receipt awaiting manager approval: " else "Recorded payment of ") ++ String.fromFloat p.amount)
                            }
                    in
                    ( fresh, refreshPayments fresh )

                Err (FieldErrors fields) ->
                    case model.paymentForm of
                        Just pf ->
                            ( { model | paymentForm = Just { pf | submitting = False, errors = fields } }, Cmd.none )

                        Nothing ->
                            ( model, Cmd.none )

                Err (GenericError message) ->
                    case model.paymentForm of
                        Just pf ->
                            ( { model
                                | paymentForm =
                                    Just { pf | submitting = False, errors = [ ( "form", message ) ] }
                              }
                            , Cmd.none
                            )

                        Nothing ->
                            ( model, Cmd.none )

        RequestedDeletePayment p ->
            ( { model | deletingPayment = Just p }, Cmd.none )

        CancelledDeletePayment ->
            ( { model | deletingPayment = Nothing }, Cmd.none )

        ConfirmedDeletePayment ->
            case ( model.deletingPayment, model.token ) of
                ( Just p, Just token ) ->
                    ( model, Api.deletePayment token p.id GotDeletedPayment )

                _ ->
                    ( model, Cmd.none )

        GotDeletedPayment result ->
            case result of
                Ok _ ->
                    let
                        fresh =
                            { model | deletingPayment = Nothing, toast = Just "Payment deleted" }
                    in
                    ( fresh, refreshPayments fresh )

                Err message ->
                    ( { model
                        | deletingPayment = Nothing
                        , toast = Just ("Delete failed: " ++ message)
                      }
                    , Cmd.none
                    )

        RequestedRefund inv ->
            ( { model | refundConfirmInvoice = Just inv }, Cmd.none )

        CancelledRefund ->
            ( { model | refundConfirmInvoice = Nothing }, Cmd.none )

        ConfirmedRefund ->
            case ( model.refundConfirmInvoice, model.token ) of
                ( Just inv, Just token ) ->
                    ( { model | refundConfirmInvoice = Nothing }
                    , Api.refundInvoice token inv.id "Refund requested via UI" GotRefunded
                    )

                _ ->
                    ( { model | refundConfirmInvoice = Nothing }, Cmd.none )

        GotRefunded result ->
            case result of
                Ok inv ->
                    let
                        updatedViewingInvoice =
                            case model.route of
                                InvoiceDetail _ ->
                                    Just inv

                                _ ->
                                    model.viewingInvoice
                    in
                    ( { model
                        | viewingInvoice = updatedViewingInvoice
                        , toast = Just "Refund requested"
                      }
                    , Cmd.none
                    )

                Err err ->
                    let
                        message =
                            case err of
                                FieldErrors _ ->
                                    "Could not request refund."

                                GenericError m ->
                                    m
                    in
                    ( { model | toast = Just message }, Cmd.none )

        _ ->
            ( model, Cmd.none )


refreshPayments : Model -> Cmd Msg
refreshPayments model =
    case ( model.token, model.viewingInvoice ) of
        ( Just t, Just inv ) ->
            Api.fetchInvoicePayments t inv.id GotInvoicePayments

        _ ->
            Cmd.none
