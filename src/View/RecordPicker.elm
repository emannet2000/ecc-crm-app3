module View.RecordPicker exposing (view)

import Html exposing (Html, node)
import Html.Attributes as Attr
import Html.Events exposing (onInput)
import Types exposing (Msg)


view : String -> String -> String -> String -> Bool -> (String -> Msg) -> Html Msg
view fieldId entity selected label submitting update =
    node "crm-record-picker"
        [ Attr.id fieldId
        , Attr.attribute "entity" entity
        , Attr.attribute "value" selected
        , Attr.attribute "label" label
        , Attr.disabled submitting
        , onInput update
        ]
        []
