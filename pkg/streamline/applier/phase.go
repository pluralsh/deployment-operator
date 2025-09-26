package applier

import (
	"slices"

	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Phase struct {
	name    smcommon.SyncPhase
	skipped []unstructured.Unstructured
	waves   []Wave
}

func (p *Phase) Name() smcommon.SyncPhase {
	return p.name
}

func (p *Phase) Skipped() []unstructured.Unstructured {
	return p.skipped
}

func (p *Phase) Waves() []Wave {
	return p.waves
}

func (p *Phase) AddWave(wave Wave) {
	p.waves = append(p.waves, wave)
}

func NewPhase(name smcommon.SyncPhase, resources []unstructured.Unstructured, skipFilter, deleteFilter FilterFunc) Phase {
	skipped := make([]unstructured.Unstructured, 0)
	deleteWave := NewWave([]unstructured.Unstructured{}, DeleteWave)

	wavesMap := make(map[int]Wave)
	for _, resource := range resources {
		if deleteFilter(resource) {
			deleteWave.Add(resource)
			continue
		}

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

	phase := Phase{
		name:    name,
		skipped: skipped,
		waves:   algorithms.Map(waves, func(e lo.Entry[int, Wave]) Wave { return e.Value }),
	}

	phase.AddWave(deleteWave)

	return phase
}

func NewPhases(resources []unstructured.Unstructured, skipFilter, deleteFilter FilterFunc) map[smcommon.SyncPhase]Phase {
	phases := make(map[smcommon.SyncPhase][]unstructured.Unstructured)
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
