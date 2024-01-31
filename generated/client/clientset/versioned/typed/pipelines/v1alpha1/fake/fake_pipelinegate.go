/*
Copyright 2021.

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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/pluralsh/deployment-operator/api/pipelines/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePipelineGates implements PipelineGateInterface
type FakePipelineGates struct {
	Fake *FakePipelinesV1alpha1
	ns   string
}

var pipelinegatesResource = schema.GroupVersionResource{Group: "pipelines.plural.sh", Version: "v1alpha1", Resource: "pipelinegates"}

var pipelinegatesKind = schema.GroupVersionKind{Group: "pipelines.plural.sh", Version: "v1alpha1", Kind: "PipelineGate"}

// Get takes name of the pipelineGate, and returns the corresponding pipelineGate object, and an error if there is any.
func (c *FakePipelineGates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PipelineGate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(pipelinegatesResource, c.ns, name), &v1alpha1.PipelineGate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineGate), err
}

// List takes label and field selectors, and returns the list of PipelineGates that match those selectors.
func (c *FakePipelineGates) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PipelineGateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(pipelinegatesResource, pipelinegatesKind, c.ns, opts), &v1alpha1.PipelineGateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PipelineGateList{ListMeta: obj.(*v1alpha1.PipelineGateList).ListMeta}
	for _, item := range obj.(*v1alpha1.PipelineGateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested pipelineGates.
func (c *FakePipelineGates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(pipelinegatesResource, c.ns, opts))

}

// Create takes the representation of a pipelineGate and creates it.  Returns the server's representation of the pipelineGate, and an error, if there is any.
func (c *FakePipelineGates) Create(ctx context.Context, pipelineGate *v1alpha1.PipelineGate, opts v1.CreateOptions) (result *v1alpha1.PipelineGate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(pipelinegatesResource, c.ns, pipelineGate), &v1alpha1.PipelineGate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineGate), err
}

// Update takes the representation of a pipelineGate and updates it. Returns the server's representation of the pipelineGate, and an error, if there is any.
func (c *FakePipelineGates) Update(ctx context.Context, pipelineGate *v1alpha1.PipelineGate, opts v1.UpdateOptions) (result *v1alpha1.PipelineGate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(pipelinegatesResource, c.ns, pipelineGate), &v1alpha1.PipelineGate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineGate), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePipelineGates) UpdateStatus(ctx context.Context, pipelineGate *v1alpha1.PipelineGate, opts v1.UpdateOptions) (*v1alpha1.PipelineGate, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(pipelinegatesResource, "status", c.ns, pipelineGate), &v1alpha1.PipelineGate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineGate), err
}

// Delete takes name of the pipelineGate and deletes it. Returns an error if one occurs.
func (c *FakePipelineGates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(pipelinegatesResource, c.ns, name, opts), &v1alpha1.PipelineGate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePipelineGates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(pipelinegatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.PipelineGateList{})
	return err
}

// Patch applies the patch and returns the patched pipelineGate.
func (c *FakePipelineGates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PipelineGate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(pipelinegatesResource, c.ns, name, pt, data, subresources...), &v1alpha1.PipelineGate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PipelineGate), err
}
