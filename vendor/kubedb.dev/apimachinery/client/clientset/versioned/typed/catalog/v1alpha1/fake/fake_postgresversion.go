/*
Copyright The KubeDB Authors.

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

	v1alpha1 "kubedb.dev/apimachinery/apis/catalog/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePostgresVersions implements PostgresVersionInterface
type FakePostgresVersions struct {
	Fake *FakeCatalogV1alpha1
}

var postgresversionsResource = schema.GroupVersionResource{Group: "catalog.kubedb.com", Version: "v1alpha1", Resource: "postgresversions"}

var postgresversionsKind = schema.GroupVersionKind{Group: "catalog.kubedb.com", Version: "v1alpha1", Kind: "PostgresVersion"}

// Get takes name of the postgresVersion, and returns the corresponding postgresVersion object, and an error if there is any.
func (c *FakePostgresVersions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PostgresVersion, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(postgresversionsResource, name), &v1alpha1.PostgresVersion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PostgresVersion), err
}

// List takes label and field selectors, and returns the list of PostgresVersions that match those selectors.
func (c *FakePostgresVersions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PostgresVersionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(postgresversionsResource, postgresversionsKind, opts), &v1alpha1.PostgresVersionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PostgresVersionList{ListMeta: obj.(*v1alpha1.PostgresVersionList).ListMeta}
	for _, item := range obj.(*v1alpha1.PostgresVersionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested postgresVersions.
func (c *FakePostgresVersions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(postgresversionsResource, opts))
}

// Create takes the representation of a postgresVersion and creates it.  Returns the server's representation of the postgresVersion, and an error, if there is any.
func (c *FakePostgresVersions) Create(ctx context.Context, postgresVersion *v1alpha1.PostgresVersion, opts v1.CreateOptions) (result *v1alpha1.PostgresVersion, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(postgresversionsResource, postgresVersion), &v1alpha1.PostgresVersion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PostgresVersion), err
}

// Update takes the representation of a postgresVersion and updates it. Returns the server's representation of the postgresVersion, and an error, if there is any.
func (c *FakePostgresVersions) Update(ctx context.Context, postgresVersion *v1alpha1.PostgresVersion, opts v1.UpdateOptions) (result *v1alpha1.PostgresVersion, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(postgresversionsResource, postgresVersion), &v1alpha1.PostgresVersion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PostgresVersion), err
}

// Delete takes name of the postgresVersion and deletes it. Returns an error if one occurs.
func (c *FakePostgresVersions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(postgresversionsResource, name), &v1alpha1.PostgresVersion{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePostgresVersions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(postgresversionsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.PostgresVersionList{})
	return err
}

// Patch applies the patch and returns the patched postgresVersion.
func (c *FakePostgresVersions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PostgresVersion, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(postgresversionsResource, name, pt, data, subresources...), &v1alpha1.PostgresVersion{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PostgresVersion), err
}
