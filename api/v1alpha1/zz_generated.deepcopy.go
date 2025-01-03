//go:build !ignore_autogenerated

/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/pluralsh/console/go/client"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSProviderCredentials) DeepCopyInto(out *AWSProviderCredentials) {
	*out = *in
	if in.AccessKeyID != nil {
		in, out := &in.AccessKeyID, &out.AccessKeyID
		*out = new(string)
		**out = **in
	}
	if in.SecretAccessKeyRef != nil {
		in, out := &in.SecretAccessKeyRef, &out.SecretAccessKeyRef
		*out = new(corev1.SecretReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSProviderCredentials.
func (in *AWSProviderCredentials) DeepCopy() *AWSProviderCredentials {
	if in == nil {
		return nil
	}
	out := new(AWSProviderCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AgentHelmConfiguration) DeepCopyInto(out *AgentHelmConfiguration) {
	*out = *in
	in.HelmConfiguration.DeepCopyInto(&out.HelmConfiguration)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AgentHelmConfiguration.
func (in *AgentHelmConfiguration) DeepCopy() *AgentHelmConfiguration {
	if in == nil {
		return nil
	}
	out := new(AgentHelmConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Binding) DeepCopyInto(out *Binding) {
	*out = *in
	if in.ID != nil {
		in, out := &in.ID, &out.ID
		*out = new(string)
		**out = **in
	}
	if in.UserID != nil {
		in, out := &in.UserID, &out.UserID
		*out = new(string)
		**out = **in
	}
	if in.UserEmail != nil {
		in, out := &in.UserEmail, &out.UserEmail
		*out = new(string)
		**out = **in
	}
	if in.GroupID != nil {
		in, out := &in.GroupID, &out.GroupID
		*out = new(string)
		**out = **in
	}
	if in.GroupName != nil {
		in, out := &in.GroupName, &out.GroupName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Binding.
func (in *Binding) DeepCopy() *Binding {
	if in == nil {
		return nil
	}
	out := new(Binding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Bindings) DeepCopyInto(out *Bindings) {
	*out = *in
	if in.Read != nil {
		in, out := &in.Read, &out.Read
		*out = make([]Binding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Write != nil {
		in, out := &in.Write, &out.Write
		*out = make([]Binding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Bindings.
func (in *Bindings) DeepCopy() *Bindings {
	if in == nil {
		return nil
	}
	out := new(Bindings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	if in.Handle != nil {
		in, out := &in.Handle, &out.Handle
		*out = new(string)
		**out = **in
	}
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.Bindings != nil {
		in, out := &in.Bindings, &out.Bindings
		*out = new(Bindings)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomHealth) DeepCopyInto(out *CustomHealth) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomHealth.
func (in *CustomHealth) DeepCopy() *CustomHealth {
	if in == nil {
		return nil
	}
	out := new(CustomHealth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CustomHealth) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomHealthList) DeepCopyInto(out *CustomHealthList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CustomHealth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomHealthList.
func (in *CustomHealthList) DeepCopy() *CustomHealthList {
	if in == nil {
		return nil
	}
	out := new(CustomHealthList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CustomHealthList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomHealthSpec) DeepCopyInto(out *CustomHealthSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomHealthSpec.
func (in *CustomHealthSpec) DeepCopy() *CustomHealthSpec {
	if in == nil {
		return nil
	}
	out := new(CustomHealthSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomHealthStatus) DeepCopyInto(out *CustomHealthStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomHealthStatus.
func (in *CustomHealthStatus) DeepCopy() *CustomHealthStatus {
	if in == nil {
		return nil
	}
	out := new(CustomHealthStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GateSpec) DeepCopyInto(out *GateSpec) {
	*out = *in
	if in.JobSpec != nil {
		in, out := &in.JobSpec, &out.JobSpec
		*out = new(batchv1.JobSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GateSpec.
func (in *GateSpec) DeepCopy() *GateSpec {
	if in == nil {
		return nil
	}
	out := new(GateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmConfiguration) DeepCopyInto(out *HelmConfiguration) {
	*out = *in
	if in.ChartName != nil {
		in, out := &in.ChartName, &out.ChartName
		*out = new(string)
		**out = **in
	}
	if in.RepoUrl != nil {
		in, out := &in.RepoUrl, &out.RepoUrl
		*out = new(string)
		**out = **in
	}
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.ValuesSecretRef != nil {
		in, out := &in.ValuesSecretRef, &out.ValuesSecretRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.ValuesConfigMapRef != nil {
		in, out := &in.ValuesConfigMapRef, &out.ValuesConfigMapRef
		*out = new(corev1.ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmConfiguration.
func (in *HelmConfiguration) DeepCopy() *HelmConfiguration {
	if in == nil {
		return nil
	}
	out := new(HelmConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmSpec) DeepCopyInto(out *HelmSpec) {
	*out = *in
	if in.Agent != nil {
		in, out := &in.Agent, &out.Agent
		*out = new(AgentHelmConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.VCluster != nil {
		in, out := &in.VCluster, &out.VCluster
		*out = new(VClusterHelmConfiguration)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmSpec.
func (in *HelmSpec) DeepCopy() *HelmSpec {
	if in == nil {
		return nil
	}
	out := new(HelmSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressReplica) DeepCopyInto(out *IngressReplica) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressReplica.
func (in *IngressReplica) DeepCopy() *IngressReplica {
	if in == nil {
		return nil
	}
	out := new(IngressReplica)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IngressReplica) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressReplicaList) DeepCopyInto(out *IngressReplicaList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IngressReplica, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressReplicaList.
func (in *IngressReplicaList) DeepCopy() *IngressReplicaList {
	if in == nil {
		return nil
	}
	out := new(IngressReplicaList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IngressReplicaList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressReplicaSpec) DeepCopyInto(out *IngressReplicaSpec) {
	*out = *in
	out.IngressRef = in.IngressRef
	if in.IngressClassName != nil {
		in, out := &in.IngressClassName, &out.IngressClassName
		*out = new(string)
		**out = **in
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = make([]networkingv1.IngressTLS, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.HostMappings != nil {
		in, out := &in.HostMappings, &out.HostMappings
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressReplicaSpec.
func (in *IngressReplicaSpec) DeepCopy() *IngressReplicaSpec {
	if in == nil {
		return nil
	}
	out := new(IngressReplicaSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubecostExtractor) DeepCopyInto(out *KubecostExtractor) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubecostExtractor.
func (in *KubecostExtractor) DeepCopy() *KubecostExtractor {
	if in == nil {
		return nil
	}
	out := new(KubecostExtractor)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KubecostExtractor) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubecostExtractorList) DeepCopyInto(out *KubecostExtractorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KubecostExtractor, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubecostExtractorList.
func (in *KubecostExtractorList) DeepCopy() *KubecostExtractorList {
	if in == nil {
		return nil
	}
	out := new(KubecostExtractorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KubecostExtractorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubecostExtractorSpec) DeepCopyInto(out *KubecostExtractorSpec) {
	*out = *in
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(string)
		**out = **in
	}
	out.KubecostServiceRef = in.KubecostServiceRef
	if in.KubecostPort != nil {
		in, out := &in.KubecostPort, &out.KubecostPort
		*out = new(int32)
		**out = **in
	}
	if in.RecommendationsSettings != nil {
		in, out := &in.RecommendationsSettings, &out.RecommendationsSettings
		*out = new(RecommendationsSettings)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubecostExtractorSpec.
func (in *KubecostExtractorSpec) DeepCopy() *KubecostExtractorSpec {
	if in == nil {
		return nil
	}
	out := new(KubecostExtractorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricsAggregate) DeepCopyInto(out *MetricsAggregate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricsAggregate.
func (in *MetricsAggregate) DeepCopy() *MetricsAggregate {
	if in == nil {
		return nil
	}
	out := new(MetricsAggregate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MetricsAggregate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricsAggregateList) DeepCopyInto(out *MetricsAggregateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MetricsAggregate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricsAggregateList.
func (in *MetricsAggregateList) DeepCopy() *MetricsAggregateList {
	if in == nil {
		return nil
	}
	out := new(MetricsAggregateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MetricsAggregateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricsAggregateStatus) DeepCopyInto(out *MetricsAggregateStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricsAggregateStatus.
func (in *MetricsAggregateStatus) DeepCopy() *MetricsAggregateStatus {
	if in == nil {
		return nil
	}
	out := new(MetricsAggregateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineGate) DeepCopyInto(out *PipelineGate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineGate.
func (in *PipelineGate) DeepCopy() *PipelineGate {
	if in == nil {
		return nil
	}
	out := new(PipelineGate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineGate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineGateList) DeepCopyInto(out *PipelineGateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineGate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineGateList.
func (in *PipelineGateList) DeepCopy() *PipelineGateList {
	if in == nil {
		return nil
	}
	out := new(PipelineGateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineGateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineGateSpec) DeepCopyInto(out *PipelineGateSpec) {
	*out = *in
	if in.GateSpec != nil {
		in, out := &in.GateSpec, &out.GateSpec
		*out = new(GateSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineGateSpec.
func (in *PipelineGateSpec) DeepCopy() *PipelineGateSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineGateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineGateStatus) DeepCopyInto(out *PipelineGateStatus) {
	*out = *in
	if in.State != nil {
		in, out := &in.State, &out.State
		*out = new(GateState)
		**out = **in
	}
	if in.JobRef != nil {
		in, out := &in.JobRef, &out.JobRef
		*out = new(client.NamespacedName)
		**out = **in
	}
	if in.SHA != nil {
		in, out := &in.SHA, &out.SHA
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineGateStatus.
func (in *PipelineGateStatus) DeepCopy() *PipelineGateStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineGateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProviderCredentials) DeepCopyInto(out *ProviderCredentials) {
	*out = *in
	if in.AWS != nil {
		in, out := &in.AWS, &out.AWS
		*out = new(AWSProviderCredentials)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProviderCredentials.
func (in *ProviderCredentials) DeepCopy() *ProviderCredentials {
	if in == nil {
		return nil
	}
	out := new(ProviderCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RecommendationsSettings) DeepCopyInto(out *RecommendationsSettings) {
	*out = *in
	if in.ExcludeNamespaces != nil {
		in, out := &in.ExcludeNamespaces, &out.ExcludeNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RequireAnnotations != nil {
		in, out := &in.RequireAnnotations, &out.RequireAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RecommendationsSettings.
func (in *RecommendationsSettings) DeepCopy() *RecommendationsSettings {
	if in == nil {
		return nil
	}
	out := new(RecommendationsSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Status) DeepCopyInto(out *Status) {
	*out = *in
	if in.ID != nil {
		in, out := &in.ID, &out.ID
		*out = new(string)
		**out = **in
	}
	if in.SHA != nil {
		in, out := &in.SHA, &out.SHA
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Status.
func (in *Status) DeepCopy() *Status {
	if in == nil {
		return nil
	}
	out := new(Status)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeInsights) DeepCopyInto(out *UpgradeInsights) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeInsights.
func (in *UpgradeInsights) DeepCopy() *UpgradeInsights {
	if in == nil {
		return nil
	}
	out := new(UpgradeInsights)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpgradeInsights) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeInsightsList) DeepCopyInto(out *UpgradeInsightsList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]UpgradeInsights, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeInsightsList.
func (in *UpgradeInsightsList) DeepCopy() *UpgradeInsightsList {
	if in == nil {
		return nil
	}
	out := new(UpgradeInsightsList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpgradeInsightsList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeInsightsSpec) DeepCopyInto(out *UpgradeInsightsSpec) {
	*out = *in
	if in.Distro != nil {
		in, out := &in.Distro, &out.Distro
		*out = new(client.ClusterDistro)
		**out = **in
	}
	if in.ClusterName != nil {
		in, out := &in.ClusterName, &out.ClusterName
		*out = new(string)
		**out = **in
	}
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(string)
		**out = **in
	}
	if in.Credentials != nil {
		in, out := &in.Credentials, &out.Credentials
		*out = new(ProviderCredentials)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeInsightsSpec.
func (in *UpgradeInsightsSpec) DeepCopy() *UpgradeInsightsSpec {
	if in == nil {
		return nil
	}
	out := new(UpgradeInsightsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VClusterHelmConfiguration) DeepCopyInto(out *VClusterHelmConfiguration) {
	*out = *in
	in.HelmConfiguration.DeepCopyInto(&out.HelmConfiguration)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VClusterHelmConfiguration.
func (in *VClusterHelmConfiguration) DeepCopy() *VClusterHelmConfiguration {
	if in == nil {
		return nil
	}
	out := new(VClusterHelmConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualCluster) DeepCopyInto(out *VirtualCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualCluster.
func (in *VirtualCluster) DeepCopy() *VirtualCluster {
	if in == nil {
		return nil
	}
	out := new(VirtualCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VirtualCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualClusterList) DeepCopyInto(out *VirtualClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VirtualCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualClusterList.
func (in *VirtualClusterList) DeepCopy() *VirtualClusterList {
	if in == nil {
		return nil
	}
	out := new(VirtualClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VirtualClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualClusterSpec) DeepCopyInto(out *VirtualClusterSpec) {
	*out = *in
	out.KubeconfigRef = in.KubeconfigRef
	in.CredentialsRef.DeepCopyInto(&out.CredentialsRef)
	if in.Cluster != nil {
		in, out := &in.Cluster, &out.Cluster
		*out = new(ClusterSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.External != nil {
		in, out := &in.External, &out.External
		*out = new(bool)
		**out = **in
	}
	if in.Helm != nil {
		in, out := &in.Helm, &out.Helm
		*out = new(HelmSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualClusterSpec.
func (in *VirtualClusterSpec) DeepCopy() *VirtualClusterSpec {
	if in == nil {
		return nil
	}
	out := new(VirtualClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualClusterStatus) DeepCopyInto(out *VirtualClusterStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualClusterStatus.
func (in *VirtualClusterStatus) DeepCopy() *VirtualClusterStatus {
	if in == nil {
		return nil
	}
	out := new(VirtualClusterStatus)
	in.DeepCopyInto(out)
	return out
}
