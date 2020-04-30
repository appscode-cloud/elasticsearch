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
