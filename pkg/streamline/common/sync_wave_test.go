package common

import (
	"testing"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetSyncWave(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		annotations map[string]string
		want        int
	}{
		{
			name:        "custom sync wave annotation with positive value",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "10"},
			want:        10,
		},
		{
			name:        "custom sync wave annotation with negative value",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "-5"},
			want:        -5,
		},
		{
			name:        "custom sync wave annotation with zero",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "0"},
			want:        0,
		},
		{
			name:        "invalid sync wave falls back to helm hook weight",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "invalid", HelmHookWeightAnnotation: "7"},
			want:        7,
		},
		{
			name:        "invalid sync wave and no helm hook falls back to default",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "not-a-number"},
			want:        2,
		},
		{
			name:        "helm hook weight annotation when sync wave not present",
			kind:        common.StatefulSetKind,
			annotations: map[string]string{HelmHookWeightAnnotation: "15"},
			want:        15,
		},
		{
			name:        "helm hook weight with negative value",
			kind:        common.ServiceKind,
			annotations: map[string]string{HelmHookWeightAnnotation: "-3"},
			want:        -3,
		},
		{
			name:        "invalid helm hook weight falls back to default",
			kind:        common.ServiceKind,
			annotations: map[string]string{HelmHookWeightAnnotation: "abc"},
			want:        3,
		},
		{
			name:        "sync wave takes precedence over helm hook weight",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "5", HelmHookWeightAnnotation: "10"},
			want:        5,
		},
		{
			name:        "no annotations - unknown kind uses default",
			kind:        "UnknownKind",
			annotations: nil,
			want:        SyncWaveDefault,
		},
		{
			name:        "empty annotations map - falls back to default",
			kind:        common.DeploymentKind,
			annotations: map[string]string{},
			want:        2,
		},
		{
			name:        "sync wave with large positive value",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "1000"},
			want:        1000,
		},
		{
			name:        "sync wave with large negative value",
			kind:        common.DeploymentKind,
			annotations: map[string]string{SyncWaveAnnotation: "-1000"},
			want:        -1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := unstructured.Unstructured{}
			u.SetKind(tt.kind)
			if tt.annotations != nil {
				u.SetAnnotations(tt.annotations)
			}

			got := GetSyncWave(u)
			if got != tt.want {
				t.Errorf("GetSyncWave() = %v, want %v", got, tt.want)
			}
		})
	}
}
