module View.Home exposing (homeView)

{-| Dashboard home page: 5-stat row, deal pipeline
donut, win-rate gauge, pipeline-value bar chart, contribution
heatmap, priority alerts, and board activity.
-}

import Dict
import Html exposing (..)
import Html.Attributes as Attr exposing (class)
import Types exposing (..)
import View.Charts exposing (BarDatum, barChart, gauge)
import View.Dashboard exposing (ActivityEntry, PriorityAlert, activityPanel, alertsPanel, caseEntry, dealEntry, leadEntry, outstandingInvoiceAlert, overdueTaskAlert, pendingAgentAlert, pendingSchoolAlert, studentEntry, urgentCaseAlert)
import View.Donut exposing (donutWithLegend)
import View.Format exposing (dateToDays)
import View.Heatmap exposing (activityHeatmap)


statCard : String -> String -> String -> Html Msg
statCard label valueText hint =
    div [ class "stat-card" ]
        [ span [ class "stat-card__label" ] [ text label ]
        , span [ class "stat-card__value" ] [ text valueText ]
        , span [ class "stat-card__hint" ] [ text hint ]
        ]


isPast : String -> String -> Bool
isPast today dateStr =
    case ( dateToDays today, dateToDays dateStr ) of
        ( Just t, Just d ) ->
            d < t

        _ ->
            False



-- ─── Derived deal numbers ────────────────────────────────────


stageCount : Model -> String -> Int
stageCount model stage =
    case model.reports of
        Success report ->
            Dict.get stage report.stageCounts |> Maybe.withDefault 0

        _ ->
            0


stageValue : Model -> String -> Float
stageValue model stage =
    case model.reports of
        Success report ->
            Dict.get stage report.stageValues |> Maybe.withDefault 0

        _ ->
            0


wonCount : Model -> Int
wonCount model =
    stageCount model "Won"


lostCount : Model -> Int
lostCount model =
    stageCount model "Lost"


conversionPct : Model -> Float
conversionPct model =
    let
        won =
            toFloat (wonCount model)

        lost =
            toFloat (lostCount model)

        closed =
            won + lost
    in
    if closed <= 0 then
        0

    else
        won / closed



-- ─── Chart data ──────────────────────────────────────────────


stageColors : List ( String, String )
stageColors =
    [ ( "Lead", "#7eb8d4" )
    , ( "Qualified", "#b8922e" )
    , ( "Proposal", "#e2b94d" )
    , ( "Negotiation", "#f0cf74" )
    , ( "Won", "#6bcb8a" )
    , ( "Lost", "#e07070" )
    ]


donutSlices : Model -> List View.Donut.Slice
donutSlices model =
    List.map
        (\( stage, color ) ->
            { label = stage
            , value = toFloat (stageCount model stage)
            , color = color
            }
        )
        stageColors


barData : Model -> List BarDatum
barData model =
    List.map
        (\( stage, color ) ->
            { label = stage
            , value = stageValue model stage
            , color = color
            }
        )
        stageColors



-- ─── Heatmap source data ─────────────────────────────────────


allCreatedAt : Model -> List ( String, Int )
allCreatedAt model =
    case model.reports of
        Success report ->
            Dict.toList report.createdCounts

        _ ->
            []


-- ─── Priority alerts ──────────────────────────────────────────


priorityAlerts : Model -> List PriorityAlert
priorityAlerts model =
    let
        fromCases =
            case model.cases of
                Success d ->
                    d.items
                        |> List.filter (\c -> c.priority == "Urgent")
                        |> List.map urgentCaseAlert

                _ ->
                    []

        fromTasks =
            case model.tasks of
                Success d ->
                    d.items
                        |> List.filter (\t -> t.status /= "done" && isPast model.today t.dueDate)
                        |> List.map overdueTaskAlert

                _ ->
                    []

        fromInvoices =
            case model.invoices of
                Success d ->
                    d.items
                        |> List.filter (\inv -> inv.balance > 0)
                        |> List.map outstandingInvoiceAlert

                _ ->
                    []

        fromSchools =
            case model.schools of
                Success d ->
                    d.items
                        |> List.filter (\s -> s.contractStatus == "Pending")
                        |> List.map pendingSchoolAlert

                _ ->
                    []

        fromAgents =
            case model.agents of
                Success d ->
                    d.items
                        |> List.filter (\a -> a.contractStatus == "Not Signed")
                        |> List.map pendingAgentAlert

                _ ->
                    []
    in
    fromCases ++ fromTasks ++ fromInvoices ++ fromSchools ++ fromAgents



-- ─── Board activity ───────────────────────────────────────────


recentEntries : Model -> List ActivityEntry
recentEntries model =
    let
        fromLeads =
            case model.leads of
                Success d ->
                    List.map leadEntry d.items

                _ ->
                    []

        fromCases =
            case model.cases of
                Success d ->
                    List.map caseEntry d.items

                _ ->
                    []

        fromStudents =
            case model.students of
                Success d ->
                    List.map studentEntry d.items

                _ ->
                    []

        fromDeals =
            case model.deals of
                Success d ->
                    List.map dealEntry d.items

                _ ->
                    []
    in
    fromLeads ++ fromCases ++ fromStudents ++ fromDeals



-- ─── Home view ────────────────────────────────────────────────


summaryText : Model -> (ReportSummary -> String) -> String
summaryText model format =
    case model.reports of
        Success report ->
            format report

        Failure _ ->
            "Unavailable"

        _ ->
            "Loading…"


homeView : Model -> User -> Html Msg
homeView model user =
    div []
        [ p [ class "content__lede content__lede--hero" ]
            [ text ("Welcome back, " ++ user.name) ]
        , p [ class "content__lede" ]
            [ text
                (if String.isEmpty model.today then
                    "Here's what's happening across your workspace today."

                 else
                    model.today ++ " · Here's what's happening across your workspace today."
                )
            ]
        , Html.node "crm-my-work" [] []
        , Html.node "crm-ai-productivity" [] []
        , div [ class "stats" ]
            [ statCard "Deals"
                (summaryText model (\report -> String.fromInt (List.sum (Dict.values report.stageCounts))))
                (summaryText model (\report -> String.fromInt (List.sum (Dict.values report.stageCounts) - (Dict.get "Won" report.stageCounts |> Maybe.withDefault 0) - (Dict.get "Lost" report.stageCounts |> Maybe.withDefault 0)) ++ " active"))
            , statCard "Open pipeline"
                (summaryText model .pipelineLabel)
                "All accessible opportunities"
            , statCard "Leads"
                (summaryText model (\report -> String.fromInt report.leads))
                (summaryText model (\report -> String.fromInt report.newLeads ++ " new"))
            , statCard "Students"
                (summaryText model (\report -> String.fromInt report.students))
                (summaryText model (\report -> String.fromInt report.approvedStudents ++ " approved"))
            , statCard "Cases"
                (summaryText model (\report -> String.fromInt report.totalCases))
                (summaryText model (\report -> String.fromInt report.activeCases ++ " open"))
            ]
        , div [ class "chart-row" ]
            [ div [ class "chart-card" ]
                [ h3 [ class "chart-card__title" ] [ text "Pipeline distribution" ]
                , p [ class "chart-card__subtitle" ]
                    [ text "Deals by stage · current snapshot" ]
                , donutWithLegend
                    (donutSlices model)
                    (String.fromInt (List.sum (List.map (\( stage, _ ) -> stageCount model stage) stageColors)))
                    "Deals"
                ]
            , div [ class "chart-card" ]
                [ h3 [ class "chart-card__title" ] [ text "Win rate" ]
                , p [ class "chart-card__subtitle" ]
                    [ text "Won ÷ (Won + Lost) · all time" ]
                , gauge
                    "Win rate"
                    (conversionPct model * 100)
                    100
                , div [ class "chart-card__subtitle" ]
                    [ text
                        (String.fromInt (wonCount model)
                            ++ " won · "
                            ++ String.fromInt (lostCount model)
                            ++ " lost"
                        )
                    ]
                ]
            ]
        , div [ class "chart-card" ]
            [ h3 [ class "chart-card__title" ] [ text "Pipeline value by stage" ]
            , p [ class "chart-card__subtitle" ]
                [ text "USD deals only · Other currencies are included in Reports" ]
            , barChart (barData model)
            ]
        , activityHeatmap model.today (allCreatedAt model)
        , Html.node "crm-recent-changes" [] []
        ]
