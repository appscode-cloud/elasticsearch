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
package api

import (
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	kutil "kmodules.xyz/client-go"
)

type ElasticsearchInterface interface {
	UpdatedElasticsearch() *api.Elasticsearch
	EnsureCertSecret() error
	EnsureDatabaseSecret() error
	EnsureDefaultConfig() error
	EnsureMasterNodes() (kutil.VerbType, error)
	EnsureClientNodes() (kutil.VerbType, error)
	EnsureDataNodes() (kutil.VerbType, error)
	EnsureCombinedNode() (kutil.VerbType, error)
}
