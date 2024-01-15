package lua

import (
	"errors"
	"fmt"
	"regexp"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/mitchellh/mapstructure"
	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"
)

func ExecuteLua(vals map[string]interface{}, tplate string) (map[string]interface{}, error) {
	output := map[string]interface{}{}
	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("Obj", luar.New(L, vals))

	for name, function := range GetFuncMap() {
		L.SetGlobal(name, luar.New(L, function))
	}
	for name, function := range sprig.GenericFuncMap() {
		L.SetGlobal(name, luar.New(L, function))
	}

	if err := L.DoString(tplate); err != nil {
		return nil, err
	}
	outTable, ok := L.GetGlobal("healthStatus").(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("the output variable is missing in the lua script")
	}
	if err := MapLua(outTable, &output); err != nil {
		return nil, err
	}

	return output, nil

}

func GetFuncMap() template.FuncMap {
	funcs := sprig.TxtFuncMap()
	funcs["isStatusConditionTrue"] = isStatusConditionTrue
	funcs["statusConditionExists"] = statusConditionExists
	return funcs
}

// Mapper maps a lua table to a Go struct pointer.
type Mapper struct {
}

// MapLua maps the lua table to the given struct pointer with default options.
func MapLua(tbl *lua.LTable, st interface{}) error {
	return NewMapper().Map(tbl, st)
}

// NewMapper returns a new mapper.
func NewMapper() *Mapper {

	return &Mapper{}
}

// Map maps the lua table to the given struct pointer.
func (mapper *Mapper) Map(tbl *lua.LTable, st interface{}) error {
	mp, ok := ToGoValue(tbl).(map[interface{}]interface{})
	if !ok {
		return errors.New("arguments #1 must be a table, but got an array")
	}
	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           st,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(mp)
}

// ToGoValue converts the given LValue to a Go object.
func ToGoValue(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LString:
		return trimQuotes(string(v))
	case lua.LNumber:
		return float64(v)
	case *lua.LTable:
		maxn := v.MaxN()
		if maxn == 0 { // table
			ret := make(map[interface{}]interface{})
			v.ForEach(func(key, value lua.LValue) {
				keystr := fmt.Sprint(ToGoValue(key))
				ret[keystr] = ToGoValue(value)
			})
			return ret
		} else { // array
			ret := make([]interface{}, 0, maxn)
			for i := 1; i <= maxn; i++ {
				ret = append(ret, ToGoValue(v.RawGetInt(i)))
			}
			return ret
		}
	default:
		return v
	}
}

func trimQuotes(s string) interface{} {
	return regexp.MustCompile(`^"(.*)"$`).ReplaceAllString(s, "$1")
}
