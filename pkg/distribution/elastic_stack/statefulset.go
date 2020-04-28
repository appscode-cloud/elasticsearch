package elastic_stack

import (
	"fmt"
	"strings"

	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/pkg/eventer"

	appsv1 "k8s.io/api/apps/v1"
	"github.com/appscode/go/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/core"
	kutil "kmodules.xyz/client-go"
	app_util "kmodules.xyz/client-go/apps/v1"
	core_util "kmodules.xyz/client-go/core/v1"
)

const (
	CustomConfigMountPath         = "/elasticsearch/custom-config"
	ExporterCertDir               = "/usr/config/certs"
	DataDir                       = "/usr/share/elasticsearch/data"
	ConfigMergerInitContainerName = "config-merger"
)

var (
	defaultClientPort = corev1.ContainerPort{
		Name:          api.ElasticsearchRestPortName,
		ContainerPort: api.ElasticsearchRestPort,
		Protocol:      corev1.ProtocolTCP,
	}
	defaultPeerPort = corev1.ContainerPort{
		Name:          api.ElasticsearchNodePortName,
		ContainerPort: api.ElasticsearchNodePort,
		Protocol:      corev1.ProtocolTCP,
	}
)

func (es *Elasticsearch) ensureStatefulSet(
	esNode *api.ElasticsearchNode,
	stsName string,
	labels map[string]string,
	replicas *int32,
	envList []corev1.EnvVar,
	initEnvList []corev1.EnvVar,
) (kutil.VerbType, error) {

	if esNode == nil {
		return kutil.VerbUnchanged, errors.New("ElasticsearchNode is empty")
	}

	if err := es.checkStatefulSet(stsName); err != nil {
		return kutil.VerbUnchanged, err
	}

	statefulSetMeta := metav1.ObjectMeta{
		Name:      stsName,
		Namespace: es.elasticsearch.Namespace,
	}

	owner := metav1.NewControllerRef(es.elasticsearch, api.SchemeGroupVersion.WithKind(api.ResourceKindElasticsearch))

	// Make a new map "labelSelector", so that it remains
	// unchanged even if the "labels" changes.
	// It contains:
	//	-	kubedb.com/kind: ResourceKindElasticsearch
	//	-	kubedb.com/name: elasticsearch.Name
	//	-	node.role.<master/data/client>: set
	labelSelector := es.elasticsearch.OffshootSelectors()
	labelSelector = core_util.UpsertMap(labelSelector, labels)

	initContainers, err := es.getInitContainers(esNode, initEnvList)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to get initContainers")
	}
	initContainers = core_util.UpsertContainers(initContainers, es.elasticsearch.Spec.PodTemplate.Spec.InitContainers)

	containers, err := es.getContainers(esNode, envList)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to get containers")
	}

	statefulSet, vt, err := app_util.CreateOrPatchStatefulSet(es.kClient, statefulSetMeta, func(in *appsv1.StatefulSet) *appsv1.StatefulSet {
		in.Labels = core_util.UpsertMap(labels, es.elasticsearch.OffshootLabels())
		in.Annotations = es.elasticsearch.Spec.PodTemplate.Controller.Annotations
		core_util.EnsureOwnerReference(&in.ObjectMeta, owner)

		in.Spec.Replicas = replicas
		in.Spec.ServiceName = es.elasticsearch.GvrSvcName()

		in.Spec.Selector = &metav1.LabelSelector{MatchLabels: labelSelector}
		in.Spec.Template.Labels = labelSelector

		in.Spec.Template.Annotations = es.elasticsearch.Spec.PodTemplate.Annotations

		in.Spec.Template.Spec.InitContainers = core_util.UpsertContainers(in.Spec.Template.Spec.InitContainers, initContainers)
		in.Spec.Template.Spec.Containers = core_util.UpsertContainers(in.Spec.Template.Spec.Containers, containers)

		in = upsertCustomConfig(in, elasticsearch, esVersion)

		in.Spec.Template.Spec.NodeSelector = elasticsearch.Spec.PodTemplate.Spec.NodeSelector
		in.Spec.Template.Spec.Affinity = elasticsearch.Spec.PodTemplate.Spec.Affinity
		if elasticsearch.Spec.PodTemplate.Spec.SchedulerName != "" {
			in.Spec.Template.Spec.SchedulerName = elasticsearch.Spec.PodTemplate.Spec.SchedulerName
		}
		in.Spec.Template.Spec.Tolerations = elasticsearch.Spec.PodTemplate.Spec.Tolerations
		in.Spec.Template.Spec.ImagePullSecrets = elasticsearch.Spec.PodTemplate.Spec.ImagePullSecrets
		in.Spec.Template.Spec.PriorityClassName = elasticsearch.Spec.PodTemplate.Spec.PriorityClassName
		in.Spec.Template.Spec.Priority = elasticsearch.Spec.PodTemplate.Spec.Priority
		in.Spec.Template.Spec.SecurityContext = elasticsearch.Spec.PodTemplate.Spec.SecurityContext

		if isClient {
			in = c.upsertMonitoringContainer(in, elasticsearch, esVersion)
			in = upsertDatabaseSecretForSG(in, esVersion, elasticsearch.Spec.DatabaseSecret.SecretName)
		}
		if !elasticsearch.Spec.DisableSecurity {
			in = upsertCertificate(in, elasticsearch.Spec.CertificateSecret.SecretName, isClient, esVersion)
		}

		if esVersion.Spec.AuthPlugin == catalog.ElasticsearchAuthPluginXpack &&
			in.Spec.Template.Spec.SecurityContext == nil {
			in.Spec.Template.Spec.SecurityContext = &core.PodSecurityContext{
				FSGroup: types.Int64P(1000),
			}
		}

		in = upsertDatabaseConfigforXPack(in, elasticsearch, esVersion)

		in = upsertDataVolume(in, elasticsearch.Spec.StorageType, pvcSpec, esVersion)
		in = upsertTemporaryVolume(in)

		in.Spec.Template.Spec.ServiceAccountName = elasticsearch.Spec.PodTemplate.Spec.ServiceAccountName
		in.Spec.UpdateStrategy = elasticsearch.Spec.UpdateStrategy

		return in
	})

	if err != nil {
		return kutil.VerbUnchanged, err
	}

	if vt == kutil.VerbCreated || vt == kutil.VerbPatched {
		// Check StatefulSet Pod status
		if err := c.CheckStatefulSetPodStatus(statefulSet); err != nil {
			return kutil.VerbUnchanged, err
		}
		c.recorder.Eventf(
			elasticsearch,
			core.EventTypeNormal,
			eventer.EventReasonSuccessful,
			"Successfully %v StatefulSet",
			vt,
		)
	}

	// ensure pdb
	if maxUnavailable != nil {
		if err := c.createPodDisruptionBudget(statefulSet, maxUnavailable); err != nil {
			return vt, err
		}
	}

	return vt, nil
}

func (es *Elasticsearch) getContainers(esNode *api.ElasticsearchNode, envList []corev1.EnvVar) ([]corev1.Container, error) {
	if esNode == nil {
		return nil, errors.New("ElasticsearchNode is empty")
	}

	containers := []corev1.Container{
		{
			Name:            api.ResourceSingularElasticsearch,
			Image:           es.esVersion.Spec.DB.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env:             envList,

			// The clientPort is only necessary for Client nodes.
			// But it is set for all type of nodes, so that our controller can
			// communicate with each nodes specifically.
			// The DBA controller uses the clientPort to check health of a node.
			Ports: []corev1.ContainerPort{defaultClientPort, defaultPeerPort},
			SecurityContext: &corev1.SecurityContext{
				Privileged: types.BoolP(false),
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"IPC_LOCK", "SYS_RESOURCE"},
				},
			},
			Resources:      esNode.Resources,
			LivenessProbe:  es.elasticsearch.Spec.PodTemplate.Spec.LivenessProbe,
			ReadinessProbe: es.elasticsearch.Spec.PodTemplate.Spec.ReadinessProbe,
			Lifecycle:      es.elasticsearch.Spec.PodTemplate.Spec.Lifecycle,
		},
	}

	return containers, nil
}

func (es *Elasticsearch) getInitContainers(esNode *api.ElasticsearchNode, envList []corev1.EnvVar) ([]corev1.Container, error) {
	if esNode == nil {
		return nil, errors.New("ElasticsearchNode is empty")
	}

	initContainers := []corev1.Container{
		{
			Name:            "init-sysctl",
			Image:           es.esVersion.Spec.InitContainer.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sysctl", "-w", "vm.max_map_count=262144"},
			SecurityContext: &corev1.SecurityContext{
				Privileged: types.BoolP(true),
			},
			Resources: esNode.Resources,
		},
	}

	initContainers = es.upsertConfigMergerInitContainer(initContainers, envList)
	return initContainers, nil
}

func (es *Elasticsearch) upsertConfigMergerInitContainer(initCon []corev1.Container, envList []corev1.EnvVar) []corev1.Container {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "esconfig",
			MountPath: ConfigFileMountPath,
		},
		{
			Name:      "data",
			MountPath: DataDir,
		},
	}

	if !es.elasticsearch.Spec.DisableSecurity {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "temp-esconfig",
			MountPath: TempConfigFileMountPath,
		})
	}

	configMerger := corev1.Container{
		Name:            ConfigMergerInitContainerName,
		Image:           es.esVersion.Spec.InitContainer.YQImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh"},
		Env:             envList,
		Args: []string{
			"-c",
			`set -x
echo "changing ownership of data folder: /usr/share/elasticsearch/data"
chown -R 1000:1000 /usr/share/elasticsearch/data

TEMP_CONFIG_FILE=/elasticsearch/temp-config/elasticsearch.yml
CUSTOM_CONFIG_DIR="/elasticsearch/custom-config"
CONFIG_FILE=/usr/share/elasticsearch/config/elasticsearch.yml

if [ -f $TEMP_CONFIG_FILE ]; then
  cp $TEMP_CONFIG_FILE $CONFIG_FILE
else
  touch $CONFIG_FILE
fi

# yq changes the file permissions after merging custom configuration.
# we need to restore the original permissions after merging done.
ORIGINAL_PERMISSION=$(stat -c '%a' $CONFIG_FILE)

# if common-config file exist then apply it
if [ -f $CUSTOM_CONFIG_DIR/common-config.yml ]; then
  yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/common-config.yml
elif [ -f $CUSTOM_CONFIG_DIR/common-config.yaml ]; then
  yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/common-config.yaml
fi

# if it is data node and data-config file exist then apply it
if [[ "$NODE_DATA" == true ]]; then
  if [ -f $CUSTOM_CONFIG_DIR/data-config.yml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/data-config.yml
  elif [ -f $CUSTOM_CONFIG_DIR/data-config.yaml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/data-config.yaml
  fi
fi

# if it is client node and client-config file exist then apply it
if [[ "$NODE_INGEST" == true ]]; then
  if [ -f $CUSTOM_CONFIG_DIR/client-config.yml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/client-config.yml
  elif [ -f $CUSTOM_CONFIG_DIR/client-config.yaml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/client-config.yaml
  fi
fi

# if it is master node and mater-config file exist then apply it
if [[ "$NODE_MASTER" == true ]]; then
  if [ -f $CUSTOM_CONFIG_DIR/master-config.yml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/master-config.yml
  elif [ -f $CUSTOM_CONFIG_DIR/master-config.yaml ]; then
    yq merge -i --overwrite $CONFIG_FILE $CUSTOM_CONFIG_DIR/master-config.yaml
  fi
fi

# restore original permission of elasticsearh.yml file
if [[ "$ORIGINAL_PERMISSION" != "" ]]; then
  chmod $ORIGINAL_PERMISSION $CONFIG_FILE
fi
`,
		},
		VolumeMounts: volumeMounts,
	}

	return append(initCon, configMerger)
}

func (es *Elasticsearch) checkStatefulSet(sName string) error {
	elasticsearchName := es.elasticsearch.OffshootName()

	// StatefulSet for Elasticsearch database
	statefulSet, err := es.kClient.AppsV1().StatefulSets(es.elasticsearch.Namespace).Get(sName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if statefulSet.Labels[api.LabelDatabaseKind] != api.ResourceKindElasticsearch ||
		statefulSet.Labels[api.LabelDatabaseName] != elasticsearchName {
		return fmt.Errorf(`intended statefulSet "%v/%v" already exists`, es.elasticsearch.Namespace, sName)
	}

	return nil
}

func (es *Elasticsearch) upsertContainerEnv(envList []corev1.EnvVar) []corev1.EnvVar {

	envList = core_util.UpsertEnvVars(envList, []corev1.EnvVar{
		{
			Name: "node.name",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "cluster.name",
			Value: es.elasticsearch.Name,
		},
		{
			Name:  "network.host",
			Value: "0.0.0.0",
		},
		{
			Name: "ELASTIC_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: es.elasticsearch.Spec.DatabaseSecret.SecretName,
					},
					Key: KeyAdminUserName,
				},
			},
		},
		{
			Name: "ELASTIC_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: es.elasticsearch.Spec.DatabaseSecret.SecretName,
					},
					Key: KeyAdminPassword,
				},
			},
		},
	}...)

	if !es.elasticsearch.Spec.DisableSecurity {
		envList = core_util.UpsertEnvVars(envList, []corev1.EnvVar{
			{
				Name: "KEY_PASS",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: es.elasticsearch.Spec.CertificateSecret.SecretName,
						},
						Key: "key_pass",
					},
				},
			},
			{
				Name:  "xpack.security.http.ssl.enabled",
				Value: fmt.Sprintf("%v", es.elasticsearch.Spec.EnableSSL),
			},
		}...)
	}

	if strings.HasPrefix(es.esVersion.Spec.Version, "7.") {
		envList = core_util.UpsertEnvVars(envList, corev1.EnvVar{
			Name:  "discovery.seed_hosts",
			Value: es.elasticsearch.MasterServiceName(),
		})
	} else {
		envList = core_util.UpsertEnvVars(envList, corev1.EnvVar{
			Name:  "discovery.zen.ping.unicast.hosts",
			Value: es.elasticsearch.MasterServiceName(),
		})
	}

	return envList
}
