package elastic_stack

import (
	"fmt"

	"github.com/appscode/go/types"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	kutil "kmodules.xyz/client-go"
)

const (
	DefaultClientNodePrefix = "client"
	DefaultDataNodePrefix   = "data"
	DefaultMasterNodePrefix = "master"

	NodeRoleMaster = "node.role.master"
	NodeRoleClient = "node.role.client"
	NodeRoleData   = "node.role.data"
	NodeRoleSet    = "set"
)

var (
	defaultClientPort = corev1.ServicePort{
		Name:       api.ElasticsearchRestPortName,
		Port:       api.ElasticsearchRestPort,
		TargetPort: intstr.FromString(api.ElasticsearchRestPortName),
	}
	defaultPeerPort = corev1.ServicePort{
		Name:       api.ElasticsearchNodePortName,
		Port:       api.ElasticsearchNodePort,
		TargetPort: intstr.FromString(api.ElasticsearchNodePortName),
	}
)

func (es *Elasticsearch) EnsureMasterNodes() (kutil.VerbType, error) {

	return kutil.VerbUnchanged, nil
}

func (es *Elasticsearch) EnsureDataNodes() (kutil.VerbType, error) {

	return kutil.VerbUnchanged, nil
}

func (es *Elasticsearch) EnsureClientNodes() (kutil.VerbType, error) {
	statefulSetName := es.elasticsearch.OffshootName()
	clientNode := es.elasticsearch.Spec.Topology.Client

	if clientNode.Prefix != "" {
		statefulSetName = fmt.Sprintf("%v-%v", clientNode.Prefix, statefulSetName)
	} else {
		statefulSetName = fmt.Sprintf("%v-%v", DefaultClientNodePrefix, statefulSetName)
	}

	labels := map[string]string{
		NodeRoleClient: NodeRoleSet,
	}

	heapSize := int64(134217728) // 128mb
	if request, found := clientNode.Resources.Requests[corev1.ResourceMemory]; found && request.Value() > 0 {
		heapSize = getHeapSizeForNode(request.Value())
	}

	// Environment variable list for main container
	envList := []corev1.EnvVar{
		{
			Name:  "ES_JAVA_OPTS",
			Value: fmt.Sprintf("-Xms%v -Xmx%v", heapSize, heapSize),
		},
		{
			Name:  "node.ingest",
			Value: "true",
		},
		{
			Name:  "node.master",
			Value: "false",
		},
		{
			Name:  "node.data",
			Value: "false",
		},
	}

	// Environment variables for init container (i.e. config-merger)
	initEnvList := []corev1.EnvVar{
		{
			Name:  "NODE_MASTER",
			Value: "false",
		},
		{
			Name:  "NODE_DATA",
			Value: "false",
		},
		{
			Name:  "NODE_INGEST",
			Value: "true",
		},
	}

	replicas := types.Int32P(1)
	if clientNode.Replicas != nil {
		replicas = clientNode.Replicas
	}

	return es.ensureStatefulSet(&clientNode, statefulSetName, labels, replicas, envList, initEnvList)
}

func (es *Elasticsearch) EnsureCombinedNode() (kutil.VerbType, error) {

	return kutil.VerbUnchanged, nil
}

func getHeapSizeForNode(val int64) int64 {
	ret := val / 100
	return ret * 80
}
