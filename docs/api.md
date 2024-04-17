# API Reference

## Packages
- [deployments.plural.sh/v1alpha1](#deploymentspluralshv1alpha1)


## deployments.plural.sh/v1alpha1

Package v1alpha1 contains API Schema definitions for the deployments v1alpha1 API group

### Resource Types
- [CustomHealth](#customhealth)
- [PipelineGate](#pipelinegate)









#### CustomHealth



CustomHealth is the Schema for the HealthConverts API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployments.plural.sh/v1alpha1` | | |
| `kind` _string_ | `CustomHealth` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CustomHealthSpec](#customhealthspec)_ |  |  |  |


#### CustomHealthSpec



CustomHealthSpec defines the desired state of CustomHealth



_Appears in:_
- [CustomHealth](#customhealth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `script` _string_ |  |  |  |




#### GateSpec



GateSpec defines the detailed gate specifications



_Appears in:_
- [PipelineGateSpec](#pipelinegatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `job` _[JobSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#jobspec-v1-batch)_ | resuse JobSpec type from the kubernetes api |  |  |


#### GateState

_Underlying type:_ _GateState_

GateState represents the state of a gate, reused from console client

_Validation:_
- Enum: [PENDING OPEN CLOSED RUNNING]

_Appears in:_
- [PipelineGateStatus](#pipelinegatestatus)



#### GateType

_Underlying type:_ _GateType_

GateType represents the type of a gate, reused from console client

_Validation:_
- Enum: [APPROVAL WINDOW JOB]

_Appears in:_
- [PipelineGateSpec](#pipelinegatespec)



#### PipelineGate



PipelineGate represents a gate blocking promotion along a release pipeline





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployments.plural.sh/v1alpha1` | | |
| `kind` _string_ | `PipelineGate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PipelineGateSpec](#pipelinegatespec)_ |  |  |  |


#### PipelineGateSpec



PipelineGateSpec defines the detailed gate specifications



_Appears in:_
- [PipelineGate](#pipelinegate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ |  |  |  |
| `name` _string_ |  |  |  |
| `type` _[GateType](#gatetype)_ |  |  | Enum: [APPROVAL WINDOW JOB] <br /> |
| `gateSpec` _[GateSpec](#gatespec)_ |  |  |  |




