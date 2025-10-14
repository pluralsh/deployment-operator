package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHasPhase(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		phase       SyncPhase
		want        bool
	}{
		{
			name:        "no annotations - defaults to sync phase",
			annotations: nil,
			phase:       SyncPhaseSync,
			want:        true,
		},
		{
			name:        "no annotations - not in pre-sync phase",
			annotations: nil,
			phase:       SyncPhasePreSync,
			want:        false,
		},
		{
			name:        "no annotations - not in post-sync phase",
			annotations: nil,
			phase:       SyncPhasePostSync,
			want:        false,
		},
		{
			name:        "has pre-sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync"},
			phase:       SyncPhasePreSync,
			want:        true,
		},
		{
			name:        "has pre-sync annotation - not in sync phase",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync"},
			phase:       SyncPhaseSync,
			want:        false,
		},
		{
			name:        "has sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "sync"},
			phase:       SyncPhaseSync,
			want:        true,
		},
		{
			name:        "has post-sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "post-sync"},
			phase:       SyncPhasePostSync,
			want:        true,
		},
		{
			name:        "has sync-fail annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "sync-fail"},
			phase:       SyncPhaseSyncFail,
			want:        true,
		},
		{
			name:        "has skip annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "skip"},
			phase:       SyncPhaseSkip,
			want:        true,
		},
		{
			name:        "multiple phases with spaces",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync, sync, post-sync"},
			phase:       SyncPhasePreSync,
			want:        true,
		},
		{
			name:        "multiple phases without spaces",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync,sync,post-sync"},
			phase:       SyncPhasePostSync,
			want:        true,
		},
		{
			name:        "helm post-install hook",
			annotations: map[string]string{HelmHookAnnotation: "post-install"},
			phase:       SyncPhasePostSync,
			want:        true,
		},
		{
			name:        "helm post-upgrade hook",
			annotations: map[string]string{HelmHookAnnotation: "post-upgrade"},
			phase:       SyncPhasePostSync,
			want:        true,
		},
		{
			name:        "helm multiple hooks with spaces",
			annotations: map[string]string{HelmHookAnnotation: "pre-install, pre-upgrade"},
			phase:       SyncPhasePreSync,

			want: true,
		},
		{
			name:        "helm multiple hooks without spaces",
			annotations: map[string]string{HelmHookAnnotation: "post-install,post-upgrade"},
			phase:       SyncPhasePostSync,

			want: true,
		},
		{
			name:        "empty annotations map - defaults to sync phase",
			annotations: map[string]string{},
			phase:       SyncPhaseSync,
			want:        true,
		},
		{
			name:        "empty annotations map - not in pre-sync phase",
			annotations: map[string]string{},
			phase:       SyncPhasePreSync,
			want:        false,
		},
		{
			name: "sync-phase annotation takes precedence over helm annotation",
			annotations: map[string]string{
				SyncPhaseAnnotation: "sync",
				HelmHookAnnotation:  "pre-install",
			},
			phase: SyncPhaseSync,
			want:  true,
		},
		{
			name: "sync-phase annotation takes precedence - helm hook should not match",
			annotations: map[string]string{
				SyncPhaseAnnotation: "sync",
				HelmHookAnnotation:  "pre-install",
			},
			phase: SyncPhasePreSync,
			want:  false,
		},
		{
			name: "helm pre-install hook - should not match sync phase",
			annotations: map[string]string{
				HelmHookAnnotation: "pre-install",
			},
			phase: SyncPhaseSync,
			want:  false,
		},
		{
			name:        "invalid helm hook is not recognized",
			annotations: map[string]string{HelmHookAnnotation: "invalid-hook"},
			phase:       SyncPhaseSync,
			want:        false,
		},
		{
			name:        "no helm hook is not recognized",
			annotations: map[string]string{HelmHookAnnotation: ""},
			phase:       SyncPhaseSync,
			want:        false,
		},
		{
			name:        "invalid sync phase is not recognized",
			annotations: map[string]string{SyncPhaseAnnotation: "invalid-phase"},
			phase:       SyncPhaseSync,
			want:        false,
		},
		{
			name:        "no sync phase is not recognized",
			annotations: map[string]string{SyncPhaseAnnotation: ""},
			phase:       SyncPhaseSync,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := unstructured.Unstructured{}
			if tt.annotations != nil {
				u.SetAnnotations(tt.annotations)
			}

			got := HasPhase(u, tt.phase)
			if got != tt.want {
				t.Errorf("HasPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}
