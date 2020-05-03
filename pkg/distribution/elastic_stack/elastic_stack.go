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
package elastic_stack

import (
	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
)

type Elasticsearch struct {
	kClient       kubernetes.Interface
	extClient     cs.Interface
	elasticsearch *api.Elasticsearch
	esVersion     *catalog.ElasticsearchVersion
}

func New(kc kubernetes.Interface, extClient cs.Interface, es *api.Elasticsearch, esVersion *catalog.ElasticsearchVersion) *Elasticsearch {
	return &Elasticsearch{
		kClient:       kc,
		extClient:     extClient,
		elasticsearch: es,
		esVersion:     esVersion,
	}
}

func (es *Elasticsearch) GetElasticsearch() *api.Elasticsearch {
	return es.elasticsearch
}
