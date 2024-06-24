package service

import (
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (s *ServiceReconciler) getLuaHealthConvert(obj *unstructured.Unstructured) (*common.HealthStatus, error) {
	out, err := lua.ExecuteLua(obj.Object, s.LuaScript)
	if err != nil {
		return nil, err
	}
	healthStatus := &common.HealthStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(out, healthStatus); err != nil {
		return nil, err
	}
	if healthStatus.Status == "" && healthStatus.Message == "" {
		return nil, nil
	}
	return healthStatus, nil
}
