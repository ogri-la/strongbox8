package core

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type test_item_a struct{ Name string }
type test_item_b struct{ Name string }

func noop_service_fn(_ *App, _ ServiceFnArgs) ServiceResult { return ServiceResult{} }

func test_service_groups() []ServiceGroup {
	ta := reflect.TypeFor[test_item_a]()
	tb := reflect.TypeFor[test_item_b]()
	return []ServiceGroup{{
		NS: MakeNS("test", "test", "service"),
		ServiceList: []Service{
			{ID: "one-a", Fn: noop_service_fn, Accepts: &Accepts{Types: []reflect.Type{ta}}},
			{ID: "many-a", Fn: noop_service_fn, Accepts: &Accepts{Types: []reflect.Type{ta}, Many: true}},
			{ID: "no-fn", Accepts: &Accepts{Types: []reflect.Type{ta}, Many: true}},
			{ID: "no-accepts", Fn: noop_service_fn},
			{ID: "many-b", Fn: noop_service_fn, Accepts: &Accepts{Types: []reflect.Type{tb}, Many: true},
				Applicable: func(_ *App, selected []Result) bool {
					return len(selected) > 0 && selected[0].Item.(test_item_b).Name == "enabled"
				}},
		},
	}}
}

func service_ids(service_list []Service) []string {
	ids := []string{}
	for _, s := range service_list {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestDeriveTypeMap(t *testing.T) {
	actual := DeriveTypeMap(test_service_groups())

	ta := reflect.TypeFor[test_item_a]()
	tb := reflect.TypeFor[test_item_b]()

	assert.Equal(t, []string{"one-a", "many-a"}, service_ids(actual[ta]))
	assert.Equal(t, []string{"many-a"}, service_ids(actual[reflect.SliceOf(ta)]))
	assert.Equal(t, []string{"many-b"}, service_ids(actual[tb]))
	assert.Equal(t, []string{"many-b"}, service_ids(actual[reflect.SliceOf(tb)]))
	assert.Len(t, actual, 4)
}

func TestSelectionGroups__single_vs_many(t *testing.T) {
	type_map := DeriveTypeMap(test_service_groups())

	given := []Result{MakeResult(NS{}, test_item_a{"x"}, "1")}
	actual := SelectionGroups(type_map, nil, given)
	assert.Len(t, actual, 1)
	assert.Equal(t, 2, len(actual[0].ServiceList))

	given = []Result{MakeResult(NS{}, test_item_a{"x"}, "1"), MakeResult(NS{}, test_item_a{"y"}, "2")}
	actual = SelectionGroups(type_map, nil, given)
	assert.Len(t, actual, 1)
	assert.Equal(t, "many-a", actual[0].ServiceList[0].Service.ID)
	assert.Len(t, actual[0].ServiceList, 1)
	assert.Len(t, actual[0].Items, 2)
}

func TestSelectionGroups__applicable(t *testing.T) {
	type_map := DeriveTypeMap(test_service_groups())

	given := []Result{MakeResult(NS{}, test_item_b{"disabled"}, "1")}
	actual := SelectionGroups(type_map, nil, given)
	assert.False(t, actual[0].ServiceList[0].Enabled)

	given = []Result{MakeResult(NS{}, test_item_b{"enabled"}, "1")}
	actual = SelectionGroups(type_map, nil, given)
	assert.True(t, actual[0].ServiceList[0].Enabled)
}

func TestSelectionGroups__mixed_types_ordered(t *testing.T) {
	type_map := DeriveTypeMap(test_service_groups())
	given := []Result{
		MakeResult(NS{}, test_item_b{"enabled"}, "1"),
		MakeResult(NS{}, test_item_a{"x"}, "2"),
	}
	actual := SelectionGroups(type_map, nil, given)
	assert.Len(t, actual, 2)
	assert.Equal(t, reflect.TypeFor[test_item_a](), actual[0].Type)
	assert.Equal(t, reflect.TypeFor[test_item_b](), actual[1].Type)
}

func TestServiceIsApplicable__no_fn(t *testing.T) {
	assert.False(t, Service{}.IsApplicable(nil, nil))
	assert.True(t, Service{Fn: noop_service_fn}.IsApplicable(nil, nil))
}

func TestServiceNeedsInput(t *testing.T) {
	selection_only := Service{Interface: ServiceInterface{ArgDefList: []ArgDef{{ID: "selected", FromSelection: true}}}}
	assert.False(t, selection_only.NeedsInput())

	more := Service{Interface: ServiceInterface{ArgDefList: []ArgDef{{ID: "selected", FromSelection: true}, {ID: "game-track"}}}}
	assert.True(t, more.NeedsInput())
}

func TestSelectionArgs(t *testing.T) {
	service := Service{Interface: ServiceInterface{ArgDefList: []ArgDef{{ID: "selected", FromSelection: true}, {ID: "other"}}}}

	given := []Result{MakeResult(NS{}, test_item_a{"x"}, "1")}
	actual := SelectionArgs(service, given)
	assert.Len(t, actual.ArgList, 1)
	assert.Equal(t, "1", actual.ArgList[0].Val.(*Result).ID)

	given = append(given, MakeResult(NS{}, test_item_a{"y"}, "2"))
	actual = SelectionArgs(service, given)
	assert.Len(t, actual.ArgList[0].Val.([]*Result), 2)
}

func TestFormSubmit__parses_and_keeps_selection(t *testing.T) {
	selected := &Result{ID: "1"}
	service := Service{Interface: ServiceInterface{ArgDefList: []ArgDef{
		{ID: "selected", FromSelection: true},
		{ID: "strict", Parser: ParseTruthyFalseyAsBool},
	}}}
	form := MakeForm(service)
	form.Update([]KeyVal{{Key: "selected", Val: selected}, {Key: "strict", Val: "false"}})

	actual, ferr := form.Submit()
	assert.Nil(t, ferr)
	assert.Equal(t, selected, actual.ArgList[0].Val)
	assert.Equal(t, false, actual.ArgList[1].Val)
}

func TestFormSubmit__panicking_parser_is_an_error(t *testing.T) {
	service := Service{Interface: ServiceInterface{ArgDefList: []ArgDef{
		{ID: "x", Parser: func(_ *App, _ string) (any, error) { panic("boom") }},
	}}}
	form := MakeForm(service)
	form.Update([]KeyVal{{Key: "x", Val: "1"}})
	_, ferr := form.Submit()
	assert.NotNil(t, ferr)
}

func TestArgChoiceResolve(t *testing.T) {
	choice := &ArgChoice{
		ChoiceList:  []any{"retail", "classic"},
		Exclusivity: ArgChoiceExclusive,
		LabelFn:     func(v any) string { return "L-" + v.(string) },
	}
	actual, err := choice.Resolve(nil, "L-classic")
	assert.NoError(t, err)
	assert.Equal(t, "classic", actual)

	_, err = choice.Resolve(nil, "L-mists")
	assert.Error(t, err)

	actual, err = choice.Resolve(nil, "retail") // a choice value itself
	assert.NoError(t, err)
	assert.Equal(t, "retail", actual)

	_, err = choice.Resolve(nil, []string{"L-retail"})
	assert.Error(t, err, "several labels for an exclusive choice")

	choice.Exclusivity = ArgChoiceNonExclusive
	actual, err = choice.Resolve(nil, []string{"L-retail", "L-classic"})
	assert.NoError(t, err)
	assert.Equal(t, []any{"retail", "classic"}, actual)
}
