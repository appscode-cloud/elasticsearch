package elastic_stack

import (
	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	"kubedb.dev/elasticsearch/pkg/distribution"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Elasticsearch struct {
	kClient       kubernetes.Interface
	extClient     cs.Interface
	elasticsearch *api.Elasticsearch
	esVersion     *catalog.ElasticsearchVersion
}

var _ distribution.Elasticsearch = &Elasticsearch{}

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

func (es *Elasticsearch) GetInitContainers() ([]corev1.Container, error) {

	return nil, nil
}

func (es *Elasticsearch) GetContainers() ([]corev1.Container, error) {

	return nil, nil
}
