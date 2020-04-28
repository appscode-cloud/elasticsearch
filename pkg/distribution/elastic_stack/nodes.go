package elastic_stack

import (
	"fmt"

	"github.com/appscode/go/types"
	corev1 "k8s.io/api/core/v1"
	kutil "kmodules.xyz/client-go"
	core_util "kmodules.xyz/client-go/core/v1"
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

	// Environment variable list for main container.
	// These are node specific, i.e. changes depending on node type.
	// Following are for Client node:
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
	// Upsert common environment variables.
	// These are same for all type of node.
	envList = es.upsertContainerEnv(envList)

	// add/overwrite user provided env; these are provided via crd spec
	envList = core_util.UpsertEnvVars(envList, es.elasticsearch.Spec.PodTemplate.Spec.Env...)

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
