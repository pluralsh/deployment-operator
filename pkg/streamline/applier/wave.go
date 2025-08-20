package applier

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/pkg/common"
)

type Wave unstructured.UnstructuredList

func (in Wave) Add(resource unstructured.Unstructured) {
	in.Items = append(in.Items, resource)
}

type Waves []Wave

func NewWaves(resources unstructured.UnstructuredList) Waves {
	waves := make([]Wave, 5)

	kindToWave := map[string]int{
		// Wave 0 - core non-namespaced resources
		common.NamespaceKind:                0,
		common.CustomResourceDefinitionKind: 0,
		common.PersistentVolumeKind:         0,
		common.ClusterRoleKind:              0,
		common.ClusterRoleListKind:          0,
		common.ClusterRoleBindingKind:       0,
		common.ClusterRoleBindingListKind:   0,
		common.StorageClassKind:             0,

		// Wave 1 - core namespaced configuration resources
		common.ConfigMapKind:           1,
		common.SecretKind:              1,
		common.SecretListKind:          1,
		common.ServiceAccountKind:      1,
		common.RoleKind:                1,
		common.RoleListKind:            1,
		common.RoleBindingKind:         1,
		common.RoleBindingListKind:     1,
		common.PodDisruptionBudgetKind: 1,
		common.ResourceQuotaKind:       1,
		common.NetworkPolicyKind:       1,
		common.LimitRangeKind:          1,
		common.PodSecurityPolicyKind:   1,
		common.IngressClassKind:        1,

		// Wave 2 - core namespaced workload resources
		common.DeploymentKind:            2,
		common.DaemonSetKind:             2,
		common.StatefulSetKind:           2,
		common.ReplicaSetKind:            2,
		common.JobKind:                   2,
		common.CronJobKind:               2,
		common.PodKind:                   2,
		common.ReplicationControllerKind: 2,

		// Wave 3 - core namespaced networking resources
		common.EndpointsKind:  3,
		common.ServiceKind:    3,
		common.IngressKind:    3,
		common.APIServiceKind: 3,
	}

	for _, resource := range resources.Items {
		if waveIdx, exists := kindToWave[resource.GetKind()]; exists {
			waves[waveIdx].Add(resource)
		} else {
			// Unknown resource kind, put it in the last wave (4)
			waves[4].Add(resource)
		}
	}

	return waves
}
