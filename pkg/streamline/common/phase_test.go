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
		isUpgrade   bool
		want        bool
	}{
		{
			name:        "no annotations - defaults to sync phase",
			annotations: nil,
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "no annotations - not in pre-sync phase",
			annotations: nil,
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "no annotations - not in post-sync phase",
			annotations: nil,
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "has pre-sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync"},
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "has pre-sync annotation - not in sync phase",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync"},
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "has sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "sync"},
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "has post-sync annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "post-sync"},
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "has sync-fail annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "sync-fail"},
			phase:       SyncPhaseSyncFail,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "has skip annotation",
			annotations: map[string]string{SyncPhaseAnnotation: "skip"},
			phase:       SyncPhaseSkip,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "multiple phases with spaces",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync, sync, post-sync"},
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "multiple phases without spaces",
			annotations: map[string]string{SyncPhaseAnnotation: "pre-sync,sync,post-sync"},
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "helm pre-install hook during install",
			annotations: map[string]string{HelmHookAnnotation: "pre-install"},
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "helm pre-install hook during upgrade - should not match",
			annotations: map[string]string{HelmHookAnnotation: "pre-install"},
			phase:       SyncPhasePreSync,
			isUpgrade:   true,
			want:        false,
		},
		{
			name:        "helm post-install hook during install",
			annotations: map[string]string{HelmHookAnnotation: "post-install"},
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "helm post-install hook during upgrade - should not match",
			annotations: map[string]string{HelmHookAnnotation: "post-install"},
			phase:       SyncPhasePostSync,
			isUpgrade:   true,
			want:        false,
		},
		{
			name:        "helm pre-upgrade hook during upgrade",
			annotations: map[string]string{HelmHookAnnotation: "pre-upgrade"},
			phase:       SyncPhasePreSync,
			isUpgrade:   true,
			want:        true,
		},
		{
			name:        "helm pre-upgrade hook during install - should not match",
			annotations: map[string]string{HelmHookAnnotation: "pre-upgrade"},
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "helm post-upgrade hook during upgrade",
			annotations: map[string]string{HelmHookAnnotation: "post-upgrade"},
			phase:       SyncPhasePostSync,
			isUpgrade:   true,
			want:        true,
		},
		{
			name:        "helm post-upgrade hook during install - should not match",
			annotations: map[string]string{HelmHookAnnotation: "post-upgrade"},
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "helm multiple hooks with spaces",
			annotations: map[string]string{HelmHookAnnotation: "pre-install, pre-upgrade"},
			phase:       SyncPhasePreSync,
			isUpgrade:   true,
			want:        true,
		},
		{
			name:        "helm multiple hooks without spaces",
			annotations: map[string]string{HelmHookAnnotation: "post-install,post-upgrade"},
			phase:       SyncPhasePostSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "helm annotation with no recognized hooks - defaults to sync", // TODO FIX
			annotations: map[string]string{HelmHookAnnotation: "invalid-hook"},
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name:        "empty annotations map - defaults to sync phase",
			annotations: map[string]string{},
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        true,
		},
		{
			name:        "empty annotations map - not in pre-sync phase",
			annotations: map[string]string{},
			phase:       SyncPhasePreSync,
			isUpgrade:   false,
			want:        false,
		},
		{
			name: "sync-phase annotation takes precedence over helm annotation",
			annotations: map[string]string{
				SyncPhaseAnnotation: "sync",
				HelmHookAnnotation:  "pre-install",
			},
			phase:     SyncPhaseSync,
			isUpgrade: false,
			want:      true,
		},
		{
			name: "sync-phase annotation takes precedence - helm hook should not match",
			annotations: map[string]string{
				SyncPhaseAnnotation: "sync",
				HelmHookAnnotation:  "pre-install",
			},
			phase:     SyncPhasePreSync,
			isUpgrade: false,
			want:      false,
		},
		{
			name: "helm pre-install hook during install - should not match sync phase",
			annotations: map[string]string{
				HelmHookAnnotation: "pre-install",
			},
			phase:     SyncPhaseSync,
			isUpgrade: false,
			want:      false,
		},
		{
			name:        "no helm hook during install defaults to sync", // TODO FIX
			annotations: map[string]string{HelmHookAnnotation: ""},
			phase:       SyncPhaseSync,
			isUpgrade:   false,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := unstructured.Unstructured{}
			if tt.annotations != nil {
				u.SetAnnotations(tt.annotations)
			}

			got := HasPhase(u, tt.phase, tt.isUpgrade)
			if got != tt.want {
				t.Errorf("HasPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}
