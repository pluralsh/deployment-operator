package applier

import (
	"slices"

	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Phase struct {
	name  smcommon.SyncPhase
	waves []Wave
}

func (p *Phase) Name() smcommon.SyncPhase {
	return p.name
}

func (p *Phase) Waves() []Wave {
	return p.waves
}

func (p *Phase) AddWave(wave Wave) {
	p.waves = append(p.waves, wave)
}

func NewPhase(name smcommon.SyncPhase, resources []unstructured.Unstructured) Phase {
	wavesMap := make(map[int]Wave)
	for _, resource := range resources {
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
		name: name,
		waves: algorithms.Map(waves, func(e lo.Entry[int, Wave]) Wave {
			return e.Value
		}),
	}
}
