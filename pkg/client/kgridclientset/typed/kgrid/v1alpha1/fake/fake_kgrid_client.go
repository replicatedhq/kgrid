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
	v1alpha1 "github.com/replicatedhq/kgrid/pkg/client/kgridclientset/typed/kgrid/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeKgridV1alpha1 struct {
	*testing.Fake
}

func (c *FakeKgridV1alpha1) Applications(namespace string) v1alpha1.ApplicationInterface {
	return &FakeApplications{c, namespace}
}

func (c *FakeKgridV1alpha1) Grids(namespace string) v1alpha1.GridInterface {
	return &FakeGrids{c, namespace}
}

func (c *FakeKgridV1alpha1) Results(namespace string) v1alpha1.ResultInterface {
	return &FakeResults{c, namespace}
}

func (c *FakeKgridV1alpha1) Versions(namespace string) v1alpha1.VersionInterface {
	return &FakeVersions{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeKgridV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
