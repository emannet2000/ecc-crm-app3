module View.Layout exposing (mobileNavigation, pageDescription, pageTitle, sidebar, topbar)

{-| App chrome: sidebar, topbar, page title.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, type_)
import Html.Events exposing (onClick)
import Json.Encode
import Svg
import Types exposing (..)
import View.Icons exposing (iconAgent, iconContacts, iconDeals, iconHome, iconLead, iconMoon, iconReports, iconSchool, iconSettings, iconSignOut, iconStudent, iconSun, iconTasks)
import View.Search exposing (searchView)


navItem : Route -> Route -> String -> Html Msg -> Html Msg
navItem current target label icon =
    let
        cls =
            if current == target then
                "nav-item nav-item--active"

            else
                "nav-item"
    in
    button
        [ class cls
        , type_ "button"
        , Attr.attribute "aria-current"
            (if current == target then
                "page"

             else
                "false"
            )
        , Attr.attribute "data-nav-label" label
        , onClick (NavigatedTo target)
        ]
        [ span [ class "nav-item__icon" ] [ icon ]
        , span [ class "nav-item__label" ] [ text label ]
        ]


hamburgerIcon : Html Msg
hamburgerIcon =
    Svg.node "svg"
        [ Attr.attribute "viewBox" "0 0 24 24"
        , Attr.attribute "width" "20"
        , Attr.attribute "height" "20"
        , Attr.attribute "fill" "none"
        , Attr.attribute "stroke" "currentColor"
        , Attr.attribute "stroke-width" "2"
        , Attr.attribute "stroke-linecap" "round"
        , Attr.attribute "stroke-linejoin" "round"
        ]
        [ Svg.node "path" [ Attr.attribute "d" "M3 12h18" ] []
        , Svg.node "path" [ Attr.attribute "d" "M3 6h18" ] []
        , Svg.node "path" [ Attr.attribute "d" "M3 18h18" ] []
        ]


isLoading : RemoteData a -> Bool
isLoading rd =
    case rd of
        Loading ->
            True

        _ ->
            False


anyLoading : Model -> Bool
anyLoading model =
    isLoading model.contacts
        || isLoading model.deals
        || isLoading model.activities
        || isLoading model.tasks
        || isLoading model.schools
        || isLoading model.students
        || isLoading model.agents
        || isLoading model.leads
        || isLoading model.cases
        || isLoading model.invoices
        || isLoading model.partners
        || isLoading model.studentDossier
        || isLoading model.caseDocuments
        || isLoading model.invoicePayments



sidebar : Model -> User -> Html Msg
sidebar model user =
    let
        effectiveRoute =
            case model.route of
                ContactDetail _ ->
                    Contacts

                DealDetail _ ->
                    Deals

                SchoolDetail _ ->
                    Schools

                StudentDetail _ ->
                    Students

                AgentDetail _ ->
                    Agents

                LeadDetail _ ->
                    Leads

                CaseDetail _ ->
                    Cases

                InvoiceDetail _ ->
                    Invoices

                PartnerDetail _ ->
                    Partners

                other ->
                    other

        cls =
            if model.sidebarOpen then
                "sidebar is-open"

            else
                "sidebar"
    in
    aside [ class cls, Attr.id "workspace-navigation", Attr.attribute "aria-label" "Workspace navigation" ]
        [ div [ class "sidebar__brand" ]
            [ img
                [ class "sidebar-brand-logo"
                , Attr.src "/public/icons/ecc-192.png"
                , Attr.alt "ECC Consulting Global emblem"
                ]
                []
            , div [ class "sidebar__wordmark" ]
                [ strong [ class "sidebar__name" ] [ text "ECC" ]
                , span [ class "sidebar__product" ] [ text "Client workspace" ]
                ]
            , button
                [ class "sidebar__close", type_ "button", onClick ToggledSideBar, Attr.attribute "aria-label" "Close navigation" ]
                [ text "×" ]
            ]
        , Html.node "crm-nav-focus"
            [ Attr.attribute "user-id" user.id
            , Attr.attribute "default-focus" (if user.role == "admin" || user.role == "manager" then "management" else "adviser")
            ] []
        , nav [ class "sidebar__nav", Attr.attribute "aria-label" "Daily work" ]
            [ navItem effectiveRoute Home "My work today" iconHome
            , div [ class "nav-preset nav-preset--adviser" ]
                [ navItem effectiveRoute Students "Students" iconStudent
                , navItem effectiveRoute Cases "Applications" iconTasks
                , navItem effectiveRoute Leads "Leads" iconLead
                , navItem effectiveRoute Tasks "Tasks" iconTasks
                ]
            , div [ class "nav-preset nav-preset--finance" ]
                [ navItem effectiveRoute Invoices "Invoices & payments" iconDeals
                , navItem effectiveRoute Reports "Reports" iconReports
                , navItem effectiveRoute Tasks "Tasks" iconTasks
                ]
            , div [ class "nav-preset nav-preset--management" ]
                [ if user.role == "admin" then navItem effectiveRoute Administration "System administration" iconSettings else text ""
                , navItem effectiveRoute Workspace "Workspace tools" iconSettings
                , navItem effectiveRoute Reports "Reports" iconReports
                , navItem effectiveRoute Cases "Applications" iconTasks
                ]
            , details [ class "navigation-more", Attr.property "open" (Json.Encode.bool (List.member effectiveRoute [ Contacts, Schools, Agents, Partners, Deals ])) ]
                [ summary [] [ text "All modules" ]
                , navItem effectiveRoute Contacts "Contacts" iconContacts
                , navItem effectiveRoute Leads "Leads" iconLead
                , navItem effectiveRoute Students "Students" iconStudent
                , navItem effectiveRoute Cases "Cases" iconTasks
                , navItem effectiveRoute Tasks "Tasks" iconTasks
                , navItem effectiveRoute Schools "Schools" iconSchool
                , navItem effectiveRoute Agents "Agents" iconAgent
                , navItem effectiveRoute Partners "Partners" iconSchool
                , navItem effectiveRoute Deals "Deals" iconDeals
                , navItem effectiveRoute Invoices "Invoices" iconDeals
                , navItem effectiveRoute Reports "Reports" iconReports
                ]
            ]
        , nav [ class "sidebar__nav sidebar__nav--bottom" ]
            [ navItem effectiveRoute Workspace "Workspace tools" iconSettings
            , if user.role == "admin" then
                navItem effectiveRoute Administration "System administration" iconSettings

              else
                text ""
            , navItem effectiveRoute Settings "Settings" iconSettings
            ]
        , div [ class "sidebar__user" ]
            [ div [ class "sidebar__avatar" ]
                [ text (String.left 1 user.name) ]
            , div [ class "sidebar__user-info" ]
                [ span [ class "sidebar__user-name" ] [ text user.name ]
                , span [ class "sidebar__user-email" ] [ text user.email ]
                ]
            ]
        , button
            [ class "sidebar__signout-btn"
            , type_ "button"
            , onClick LoggedOut
            ]
            [ iconSignOut
            , span [] [ text "Sign out" ]
            ]
        ]


mobileNavigation : Model -> Html Msg
mobileNavigation model =
    let
        selected =
            case model.route of
                StudentDetail _ ->
                    Students

                ContactDetail _ ->
                    Contacts

                CaseDetail _ ->
                    Cases

                route ->
                    route

        moreSelected =
            model.sidebarOpen || not (List.member selected [ Home, Students, Cases, Tasks, Invoices, Reports ])
    in
    nav [ class "mobile-nav", Attr.attribute "aria-label" "Mobile navigation" ]
        [ navItem selected Home "Home" iconHome
        , navItem selected Students "Students" iconStudent
        , navItem selected Cases "Applications" iconTasks
        , navItem selected Invoices "Invoices" iconDeals
        , navItem selected Reports "Reports" iconReports
        , navItem selected Tasks "Tasks" iconTasks
        , button
            [ class ("nav-item" ++ (if moreSelected then " nav-item--active" else ""))
            , type_ "button"
            , onClick ToggledSideBar
            , Attr.attribute "aria-controls" "workspace-navigation"
            , Attr.attribute "aria-expanded" (if model.sidebarOpen then "true" else "false")
            , Attr.attribute "aria-label" "More navigation options"
            ]
            [ span [ class "nav-item__icon" ] [ hamburgerIcon ]
            , span [ class "nav-item__label" ] [ text "More" ]
            ]
        ]


topbar : Model -> User -> Html Msg
topbar model user =
    let
        alerts =
            model.unreadNotifications
    in
    header [ class "topbar" ]
        [ div [ class "header-left" ]
            [ button
                [ class "topbar__menu-btn"
                , type_ "button"
                , onClick ToggledSideBar
                , Attr.attribute "aria-controls" "workspace-navigation"
                , Attr.attribute "aria-label" "Toggle navigation"
                , Attr.attribute "aria-expanded"
                    (if model.sidebarOpen then
                        "true"

                     else
                        "false"
                    )
                ]
                [ hamburgerIcon ]
            , div [ class "workspace-breadcrumb" ]
                [ span [] [ text "Workspace" ]
                , span [ class "workspace-breadcrumb__divider", Attr.attribute "aria-hidden" "true" ] [ text "/" ]
                , strong [] [ text (pageTitle model) ]
                ]
            ]
        , searchView model
        , div [ class "header-right" ]
            [ if anyLoading model then
                div
                    [ class "topbar__loading"
                    , Attr.attribute "role" "status"
                    , Attr.attribute "aria-label" "Loading data"
                    ]
                    []

              else
                text ""
            , div [ class "live-pill" ]
                [ span [ class "dot" ] []
                , text "System live"
                ]
            , button
                [ class "ecc-btn ecc-btn--ghost ecc-btn--inline"
                , type_ "button"
                , onClick (NavigatedTo Workspace)
                ]
                [ text "Notifications"
                , if alerts > 0 then
                    span [ class "badge-count" ]
                        [ text (String.fromInt alerts) ]

                  else
                    text ""
                ]
            , button
                [ class "icon-button theme-toggle"
                , type_ "button"
                , onClick ToggledTheme
                , Attr.title
                    (if model.theme == LightTheme then
                        "Switch to dark theme"

                     else
                        "Switch to light theme"
                    )
                , Attr.attribute "aria-label"
                    (if model.theme == LightTheme then
                        "Switch to dark theme"

                     else
                        "Switch to light theme"
                    )
                ]
                [ if model.theme == LightTheme then
                    iconMoon

                  else
                    iconSun
                ]
            , button [ class "user-chip", type_ "button", onClick (NavigatedTo Settings), Attr.attribute "aria-label" "Open profile settings" ]
                [ div [ class "avatar" ]
                    [ text (String.left 1 user.name) ]
                , span [ class "user-chip__name" ] [ text user.name ]
                ]
            ]
        ]


pageTitle : Model -> String
pageTitle model =
    case model.route of
        Home ->
            "My work today"

        Contacts ->
            "Contacts"

        ContactDetail _ ->
            "Contact"

        Deals ->
            "Deals"

        DealDetail _ ->
            "Deal"

        Tasks ->
            "Tasks"

        Workspace ->
            "Workspace tools"

        Administration ->
            "System administration"

        Reports ->
            "Reports"

        Settings ->
            "Settings"

        Schools ->
            "Schools"

        SchoolDetail _ ->
            "School"

        Students ->
            "Students"

        StudentDetail _ ->
            "Student"

        Agents ->
            "Agents"

        AgentDetail _ ->
            "Agent"

        Leads ->
            "Leads"

        LeadDetail _ ->
            "Lead"

        Cases ->
            "Cases"

        CaseDetail _ ->
            "Case"

        Invoices ->
            "Invoices"

        InvoiceDetail _ ->
            "Invoice"

        Partners ->
            "Partners"

        PartnerDetail _ ->
            "Partner"


pageDescription : Model -> String
pageDescription model =
    case model.route of
        Home ->
            "Your pipeline, priorities, and recent activity in one place."

        Contacts ->
            "Manage client relationships and keep every conversation connected."

        Leads ->
            "Track new enquiries from first contact to conversion."

        Students ->
            "Manage student profiles, applications, and progress."

        Schools ->
            "Keep school partnerships and enrolment details up to date."

        Agents ->
            "Manage your agent network and referral relationships."

        Partners ->
            "Organise partner agreements, contacts, and compliance."

        Deals ->
            "Move opportunities forward and track your sales pipeline."

        Tasks ->
            "Stay on top of follow-ups, deadlines, and next steps."

        Cases ->
            "Track applications, documents, and decisions."

        Invoices ->
            "Manage fees, payments, and outstanding balances."

        Workspace ->
            "Manage your team, security, communication, and daily workflows."

        Administration ->
            "Manage user accounts, access levels, teams, and workspace security."

        Reports ->
            "Understand performance, export records, and review changes."

        Settings ->
            "Manage your profile, password, and account sessions."

        _ ->
            "Review this record and manage its related activity."
