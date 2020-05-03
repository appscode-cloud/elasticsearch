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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cutil "kubedb.dev/elasticsearch/pkg/util/cert"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	"github.com/pkg/errors"
	"gomodules.xyz/envsubst"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutil "kmodules.xyz/client-go"
	app_util "kmodules.xyz/client-go/apps/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
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
	nodeRole string,
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

	affinity, err := parseAffinityTemplate(es.elasticsearch.Spec.PodTemplate.Spec.Affinity.DeepCopy(), nodeRole)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to parse the affinity template")
	}

	// Get default initContainers; i.e.
	initContainers, err := es.getInitContainers(esNode, initEnvList)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to get initContainers")
	}

	// Add/Overwrite user provided initContainers
	initContainers = core_util.UpsertContainers(initContainers, es.elasticsearch.Spec.PodTemplate.Spec.InitContainers)

	// Get elasticsearch container.
	// Also get monitoring sidecar if any.
	containers, err := es.getContainers(esNode, envList)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to get containers")
	}

	volumes, pvc, err := es.getVolumes(esNode)
	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to get volumes")
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

		in.Spec.Template.Spec.NodeSelector = es.elasticsearch.Spec.PodTemplate.Spec.NodeSelector
		in.Spec.Template.Spec.Affinity = affinity
		if es.elasticsearch.Spec.PodTemplate.Spec.SchedulerName != "" {
			in.Spec.Template.Spec.SchedulerName = es.elasticsearch.Spec.PodTemplate.Spec.SchedulerName
		}
		in.Spec.Template.Spec.Tolerations = es.elasticsearch.Spec.PodTemplate.Spec.Tolerations
		in.Spec.Template.Spec.ImagePullSecrets = es.elasticsearch.Spec.PodTemplate.Spec.ImagePullSecrets
		in.Spec.Template.Spec.PriorityClassName = es.elasticsearch.Spec.PodTemplate.Spec.PriorityClassName
		in.Spec.Template.Spec.Priority = es.elasticsearch.Spec.PodTemplate.Spec.Priority
		in.Spec.Template.Spec.SecurityContext = es.elasticsearch.Spec.PodTemplate.Spec.SecurityContext

		// securityContext for x-pack
		if in.Spec.Template.Spec.SecurityContext == nil {
			in.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
				FSGroup: types.Int64P(1000),
			}
		}

		in.Spec.Template.Spec.ServiceAccountName = es.elasticsearch.Spec.PodTemplate.Spec.ServiceAccountName

		// Upsert volumeClaimTemplates if any
		in.Spec.VolumeClaimTemplates = core_util.UpsertVolumeClaim(in.Spec.VolumeClaimTemplates, *pvc)
		// Upsert volumes
		in.Spec.Template.Spec.Volumes = core_util.UpsertVolume(in.Spec.Template.Spec.Volumes, volumes...)

		// Statefulset update strategy is set default to "OnDelete".
		// Any kind of modification on Elasticsearch will be performed via ElasticsearchModificationRequest CRD.
		// If user update the Elasticsearch object without ElasticsearchModificationRequest,
		// user will have delete the pods manually to encounter the changes.
		in.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}

		return in
	})

	if err != nil {
		return kutil.VerbUnchanged, errors.Wrap(err, "failed to create or patch statefulset")
	}

	if vt == kutil.VerbCreated || vt == kutil.VerbPatched {
		// Check whether StatefulSet's Pods are running.
		// Given timeout: 10 minutes
		// TODO: what if there is a huge number of replicas, is this timeout enough?
		if err := es.checkStatefulSetPodStatus(statefulSet); err != nil {
			return kutil.VerbUnchanged, err
		}
	}

	// ensure pdb
	if esNode.MaxUnavailable != nil {
		if err := es.createPodDisruptionBudget(statefulSet, esNode.MaxUnavailable); err != nil {
			return vt, errors.Wrap(err, "failed to create PodDisruptionBudget")
		}
	}

	return vt, nil
}

func (es *Elasticsearch) getVolumes(esNode *api.ElasticsearchNode) ([]corev1.Volume, *corev1.PersistentVolumeClaim, error) {
	if esNode == nil {
		return nil, nil, errors.New("elasticsearchNode is empty")
	}

	var volumes []corev1.Volume
	var pvc *corev1.PersistentVolumeClaim

	// Upsert Volume for configuration directory
	volumes = core_util.UpsertVolume(volumes, corev1.Volume{
		Name: "esconfig",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	// Upsert Volume for the default configuration provided for x-pack.
	// This configuration will also be copied as default elasticsearch configuration (i.e. elasticsearch.yaml)
	// from config-merger initContainer.
	if !es.elasticsearch.Spec.DisableSecurity {
		cmName := fmt.Sprintf("%v-%v", es.elasticsearch.OffshootName(), DatabaseConfigMapSuffix)
		_, err := es.kClient.CoreV1().ConfigMaps(es.elasticsearch.GetNamespace()).Get(cmName, metav1.GetOptions{})
		if err != nil {
			return nil, nil, errors.Wrap(err, fmt.Sprintf("failed to get configmap: %s/%s", es.elasticsearch.GetNamespace(), cmName))
		}

		volumes = core_util.UpsertVolume(volumes, corev1.Volume{
			Name: "temp-esconfig",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmName,
					},
				},
			},
		})
	}

	// Upsert Volume for user provided custom configuration.
	// These configuration will be merged to default config yaml (ie. elasticsearch.yaml)
	// from config-merger initContainer.
	if es.elasticsearch.Spec.ConfigSource != nil {
		volumes = core_util.UpsertVolume(volumes, corev1.Volume{
			Name:         "custom-config",
			VolumeSource: *es.elasticsearch.Spec.ConfigSource,
		})
	}

	// Upsert Volume for data directory.
	// If storageType is "Ephemeral", add volume of "EmptyDir" type.
	// The storageType is default to "Durable".
	if es.elasticsearch.Spec.StorageType == api.StorageTypeEphemeral {
		ed := corev1.EmptyDirVolumeSource{}
		if esNode.Storage != nil {
			if sz, found := esNode.Storage.Resources.Requests[corev1.ResourceStorage]; found {
				ed.SizeLimit = &sz
			}
		}
		volumes = core_util.UpsertVolume(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &ed,
			},
		})
	} else {
		if len(esNode.Storage.AccessModes) == 0 {
			esNode.Storage.AccessModes = []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			}
			log.Infof(`Using "%v" as AccessModes in "%v"`, corev1.ReadWriteOnce, esNode.Storage)
		}

		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: *esNode.Storage,
		}
		if esNode.Storage.StorageClassName != nil {
			pvc.Annotations = map[string]string{
				"volume.beta.kubernetes.io/storage-class": *esNode.Storage.StorageClassName,
			}
		}
	}

	// Upsert Volume for certificates
	if es.elasticsearch.Spec.CertificateSecret == nil && !es.elasticsearch.Spec.DisableSecurity {
		return nil, nil, errors.New("Certificate secret is missing")
	}
	if !es.elasticsearch.Spec.DisableSecurity {
		volumes = core_util.UpsertVolume(volumes, corev1.Volume{
			Name: "certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: es.elasticsearch.Spec.CertificateSecret.SecretName,
					Items: []corev1.KeyToPath{
						{
							Key:  cutil.RootKeyStore,
							Path: cutil.RootKeyStore,
						},
						{
							Key:  cutil.NodeKeyStore,
							Path: cutil.NodeKeyStore,
						},
						{
							Key:  cutil.ClientKeyStore,
							Path: cutil.ClientKeyStore,
						},
					},
				},
			},
		})
	}

	// Upsert Volume for monitoring sidecar
	if es.elasticsearch.GetMonitoringVendor() == mona.VendorPrometheus && es.elasticsearch.Spec.EnableSSL {
		volumes = core_util.UpsertVolume(volumes, corev1.Volume{
			Name: "exporter-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: es.elasticsearch.Spec.CertificateSecret.SecretName,
					Items: []corev1.KeyToPath{
						{
							Key:  "root.pem",
							Path: "root.pem",
						},
					},
				},
			},
		})
	}

	// Upsert temp Volume
	volumes = core_util.UpsertVolume(volumes, corev1.Volume{
		Name: "temp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	return volumes, pvc, nil
}

func (es *Elasticsearch) getContainers(esNode *api.ElasticsearchNode, envList []corev1.EnvVar) ([]corev1.Container, error) {
	if esNode == nil {
		return nil, errors.New("ElasticsearchNode is empty")
	}

	// Add volumeMounts for elasticsearch container
	// 		- data directory
	//		- configuration
	// 		- temp directory
	volumeMount := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: DataDir,
		},
		{
			Name:      "esconfig",
			MountPath: filepath.Join(ConfigFileMountPath, ConfigFileName),
			SubPath:   ConfigFileName,
		},
		{
			Name:      "temp",
			MountPath: "/tmp",
		},
	}

	if !es.elasticsearch.Spec.DisableSecurity {
		volumeMount = core_util.UpsertVolumeMount(volumeMount, corev1.VolumeMount{
			Name:      "certs",
			MountPath: filepath.Join(ConfigFileMountPath, "certs"),
		})
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
			VolumeMounts:   volumeMount,
			LivenessProbe:  es.elasticsearch.Spec.PodTemplate.Spec.LivenessProbe,
			ReadinessProbe: es.elasticsearch.Spec.PodTemplate.Spec.ReadinessProbe,
			Lifecycle:      es.elasticsearch.Spec.PodTemplate.Spec.Lifecycle,
		},
	}

	// upsert metrics exporter sidecar for monitoring purpose
	var err error
	if es.elasticsearch.Spec.Monitor != nil {
		containers, err = es.upsertMonitoringContainer(containers)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get monitoring container")
		}
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

	// mount path for custom configuration
	if es.elasticsearch.Spec.ConfigSource != nil {
		volumeMounts = core_util.UpsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      "custom-config",
			MountPath: CustomConfigMountPath,
		})
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

func (es *Elasticsearch) checkStatefulSetPodStatus(statefulSet *appsv1.StatefulSet) error {
	err := core_util.WaitUntilPodRunningBySelector(
		es.kClient,
		statefulSet.Namespace,
		statefulSet.Spec.Selector,
		int(types.Int32(statefulSet.Spec.Replicas)),
	)
	if err != nil {
		return err
	}
	return nil
}

func parseAffinityTemplate(affinity *corev1.Affinity, nodeRole string) (*corev1.Affinity, error) {
	if affinity == nil {
		return nil, errors.New("affinity is nil")
	}

	templateMap := map[string]string{
		api.ElasticsearchNodeAffinityTemplateVar: nodeRole,
	}

	jsonObj, err := json.Marshal(affinity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal affinity")
	}

	resolved, err := envsubst.EvalMap(string(jsonObj), templateMap)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(resolved), affinity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the affinity")
	}

	return affinity, nil
}

// INITIAL_MASTER_NODES value for >= ES7
func (es *Elasticsearch) getInitialMasterNodes() string {
	var value string
	stsName := es.elasticsearch.OffshootName()
	replicas := types.Int32(es.elasticsearch.Spec.Replicas)
	if es.elasticsearch.Spec.Topology != nil {
		// If replicas is not provided, default to 1
		if es.elasticsearch.Spec.Topology.Master.Replicas != nil {
			replicas = types.Int32(es.elasticsearch.Spec.Topology.Master.Replicas)
		} else {
			replicas = 1
		}

		// If master.prefix is provided, name will be "GivenPrefix-ESName".
		// The master.prefix is default to "master".
		if es.elasticsearch.Spec.Topology.Master.Prefix != "" {
			stsName = fmt.Sprintf("%s-%s", es.elasticsearch.Spec.Topology.Master.Prefix, stsName)
		} else {
			stsName = fmt.Sprintf("%s-%s", DefaultMasterNodePrefix, stsName)
		}
	}

	for i := int32(0); i < replicas; i++ {
		if i != 0 {
			value += ","
		}
		value += fmt.Sprintf("%v-%v", stsName, i)
	}

	return value
}
