package common

import (
	"github.com/pluralsh/deployment-operator/pkg/lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetLuaHealthConvert(obj *unstructured.Unstructured, luaScript string) (*HealthStatus, error) {
	out, err := lua.ExecuteLua(obj.Object, luaScript)
	if err != nil {
		return nil, err
	}
	healthStatus := &HealthStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(out, healthStatus); err != nil {
		return nil, err
	}
	if healthStatus.Status == "" && healthStatus.Message == "" {
		return nil, nil
	}
	return healthStatus, nil
}
