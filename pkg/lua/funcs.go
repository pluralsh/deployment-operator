package lua

import (
	"fmt"

	"github.com/pluralsh/polly/luautils"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Status struct {
	Conditions []metav1.Condition
}

func statusConditionExists(s map[string]interface{}, condition string) bool {
	sts := Status{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(s, &sts); err != nil {
		return false
	}

	return meta.FindStatusCondition(sts.Conditions, condition) != nil
}

func isStatusConditionTrue(s map[string]interface{}, condition string) bool {
	sts := Status{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(s, &sts); err != nil {
		return false
	}

	if meta.FindStatusCondition(sts.Conditions, condition) != nil {
		if meta.IsStatusConditionTrue(sts.Conditions, condition) {
			return true
		}
	}

	return false
}

func luaStatusConditionExists(L *lua.LState) int {
	tbl := L.CheckTable(1)
	cond := L.CheckString(2)

	// Convert table to Go value
	obj := luautils.ToGoValue(tbl)

	// Sanitize to ensure map[string]interface{}
	m, ok := obj.(map[string]interface{})
	if !ok {
		// Wrap slice or convert map[interface{}]interface{} recursively
		m = sanitizeValue(obj).(map[string]interface{})
	}

	L.Push(lua.LBool(statusConditionExists(m, cond)))
	return 1
}

func luaIsStatusConditionTrue(L *lua.LState) int {
	tbl := L.CheckTable(1)
	cond := L.CheckString(2)

	obj := luautils.ToGoValue(tbl)

	m, ok := obj.(map[string]interface{})
	if !ok {
		m = sanitizeValue(obj).(map[string]interface{})
	}

	L.Push(lua.LBool(isStatusConditionTrue(m, cond)))
	return 1
}

func sanitizeValue(val interface{}) interface{} {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for key, value := range v {
			strKey := fmt.Sprintf("%v", key) // Convert key to string
			m[strKey] = sanitizeValue(value)
		}
		return m
	case []interface{}:
		for i := range v {
			v[i] = sanitizeValue(v[i])
		}
		return v
	default:
		return v
	}
}
