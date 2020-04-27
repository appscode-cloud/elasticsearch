package distribution

import (
	"fmt"

	kutil "kmodules.xyz/client-go"

	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	"kubedb.dev/elasticsearch/pkg/distribution/elastic_stack"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Elasticsearch interface {
	GetElasticsearch() *api.Elasticsearch
	EnsureCertSecret() error
	EnsureDatabaseSecret() error
	EnsureDefaultConfig() error
	EnsureMasterNodes() (kutil.VerbType, error)
	EnsureClientNodes() (kutil.VerbType, error)
	EnsureDataNodes() (kutil.VerbType, error)
	EnsureCombinedNode() (kutil.VerbType, error)
}

func GetElasticsearch(kc kubernetes.Interface, extClient cs.Interface, es *api.Elasticsearch) (Elasticsearch, error) {
	if kc == nil {
		return nil, errors.New("Kubernetes client is empty")
	}
	if extClient == nil {
		return nil, errors.New("KubeDB client is empty")
	}
	if es == nil {
		return nil, errors.New("Elasticsearch object is empty")
	}

	v := es.Spec.Version
	esVersion, err := extClient.CatalogV1alpha1().ElasticsearchVersions().Get(v, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get elasticsearchVersion: %s", v))
	}

	if esVersion.Spec.AuthPlugin == catalog.ElasticsearchAuthPluginXpack {
		return elastic_stack.New(kc, extClient, es, esVersion), nil
	} else {
		return nil, errors.New("Unknown elasticsearch auth plugin")
	}
}
