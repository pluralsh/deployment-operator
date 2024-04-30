package template

import (
	"reflect"
	"strings"

	console "github.com/pluralsh/console-client-go"
)

func configMap(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}

func contexts(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) map[string]map[string]interface{} {
	res := map[string]map[string]interface{}{}
	for _, context := range svc.Contexts {
		res[context.Name] = context.Configuration
	}
	return res
}

func indent(v string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.ReplaceAll(v, "\n", "\n"+pad)
}

func nindent(v string, spaces int) string {
	return "\n" + indent(v, spaces)
}

func coalesce(v ...interface{}) interface{} {
	for _, val := range v {
		if !empty(val) {
			return val
		}
	}
	return nil
}

func ternary(v bool, vt interface{}, vf interface{}) interface{} {
	if v {
		return vt
	}

	return vf
}

func dfault(v1, v2 interface{}) interface{} {
	if empty(v1) {
		return v2
	}

	return v1
}

func empty(given interface{}) bool {
	g := reflect.ValueOf(given)
	if !g.IsValid() {
		return true
	}

	// Basically adapted from text/template.isTrue
	switch g.Kind() {
	default:
		return g.IsNil()
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return g.Len() == 0
	case reflect.Bool:
		return !g.Bool()
	case reflect.Complex64, reflect.Complex128:
		return g.Complex() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return g.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return g.Float() == 0
	case reflect.Struct:
		return false
	}
}
