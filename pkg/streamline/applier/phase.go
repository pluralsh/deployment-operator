package applier

import (
	"slices"

	"github.com/pluralsh/deployment-operator/pkg/streamline"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Phase struct {
	name       smcommon.SyncPhase
	skipped    []unstructured.Unstructured
	waves      []Wave
	deleteWave Wave
}

func (p *Phase) Name() smcommon.SyncPhase {
	return p.name
}

func (p *Phase) Skipped() []unstructured.Unstructured {
	return p.skipped
}

func (p *Phase) Waves() []Wave {
	if p.deleteWave.Len() != 0 {
		return append(p.waves, p.deleteWave)
	}

	return p.waves
}

func (p *Phase) HasWaves() bool {
	return len(p.Waves()) > 0
}

func (p *Phase) AddWave(wave Wave) {
	p.waves = append(p.waves, wave)
}

func (p *Phase) DeletedCount() int {
	return p.deleteWave.Len()
}

func (p *Phase) Successful() bool {
	resources := p.skipped
	for _, wave := range p.waves {
		resources = append(resources, wave.items...)
	}

	return streamline.GetGlobalStore().AreResourcesHealthy(resources)
}

func NewPhase(name smcommon.SyncPhase, resources []unstructured.Unstructured, skipFilter FilterFunc, deleteFilter func(resources []unstructured.Unstructured) (toApply, toDelete []unstructured.Unstructured)) Phase {
	skipped := make([]unstructured.Unstructured, 0)
	toDelete, toApply := deleteFilter(resources)
	deleteWave := NewWave(toDelete, DeleteWave)

	wavesMap := make(map[int]Wave)
	for _, resource := range toApply {
		if skipFilter(resource) {
			skipped = append(skipped, resource)
			continue
		}

		i := smcommon.GetSyncWave(resource)
		if wave, ok := wavesMap[i]; !ok {
			wavesMap[i] = NewWave([]unstructured.Unstructured{resource}, ApplyWave)
		} else {
			wave.Add(resource)
			wavesMap[i] = wave
		}
	}

	waves := lo.Entries(wavesMap)
	slices.SortFunc(waves, func(a, b lo.Entry[int, Wave]) int {
		return a.Key - b.Key
	})

	return Phase{
		name:       name,
		skipped:    skipped,
		deleteWave: deleteWave,
		waves:      algorithms.Map(waves, func(e lo.Entry[int, Wave]) Wave { return e.Value }),
	}
}

type Phases map[smcommon.SyncPhase]Phase

func (in Phases) Next(phase *smcommon.SyncPhase, failed bool) *Phase {
	if phase == nil {
		return in.get(smcommon.SyncPhasePreSync)
	}

	if failed && *phase != smcommon.SyncPhaseSync {
		return nil
	}

	if failed {
		return in.get(smcommon.SyncPhaseSyncFail)
	}

	switch *phase {
	case smcommon.SyncPhasePreSync:
		return in.get(smcommon.SyncPhaseSync)
	case smcommon.SyncPhaseSync:
		return in.get(smcommon.SyncPhasePostSync)
	case smcommon.SyncPhasePostSync:
		return nil
	}

	return nil
}

func (in Phases) get(phase smcommon.SyncPhase) *Phase {
	p, ok := in[phase]
	if !ok {
		return nil
	}

	return &p
}

func NewPhases(resources []unstructured.Unstructured, skipFilter FilterFunc, deleteFilter func(resources []unstructured.Unstructured) (toApply, toDelete []unstructured.Unstructured)) Phases {
	phases := map[smcommon.SyncPhase][]unstructured.Unstructured{}

	// Ensure all phases are initialized for the `Next` function to work properly.
	for _, p := range smcommon.SyncPhases {
		phases[p] = make([]unstructured.Unstructured, 0)
	}

	for _, resource := range resources {
		p := smcommon.GetSyncPhase(resource)
		if phase, ok := phases[p]; !ok {
			phases[p] = []unstructured.Unstructured{resource}
		} else {
			phases[p] = append(phase, resource)
		}
	}

	return lo.MapValues(phases, func(u []unstructured.Unstructured, p smcommon.SyncPhase) Phase {
		return NewPhase(p, u, skipFilter, deleteFilter)
	})
}
