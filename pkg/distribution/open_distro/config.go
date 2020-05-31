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
package open_distro

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
)

const (
	ConfigFileName          = "elasticsearch.yml"
	ConfigFileMountPath     = "/usr/share/elasticsearch/config"
	TempConfigFileMountPath = "/elasticsearch/temp-config"
	DatabaseConfigMapSuffix = "config"
	SecurityConfigPath      = "/usr/share/elasticsearch/plugins/opendistro_security/securityconfig"
	InternalUserFileName    = "internal_users.yml"
)

var internalUserConfigFile = `
---
# This is the internal user database
# The hash value is a bcrypt hash and can be generated with plugin/tools/hash.sh

_meta:
  type: "internalusers"
  config_version: 2

# Define your internal users here

admin:
  hash: "%s"
  reserved: true
  backend_roles:
  - "admin"
  description: "Admin user"

kibanaserver:
  hash: "%s"
  reserved: true
  description: "Kibanaserver user"

## Demo users

kibanaro:
  hash: "%s"
  reserved: false
  backend_roles:
  - "kibanauser"
  - "readall"
  attributes:
    attribute1: "value1"
    attribute2: "value2"
    attribute3: "value3"
  description: "Demo kibanaro user"

logstash:
  hash: "%s"
  reserved: false
  backend_roles:
  - "logstash"
  description: "Demo logstash user"

readall:
  hash: "%s"
  reserved: false
  backend_roles:
  - "readall"
  description: "Demo readall user"

snapshotrestore:
  hash: "%s"
  reserved: false
  backend_roles:
  - "snapshotrestore"
  description: "Demo snapshotrestore user"`

var xpack_config = `
xpack.security.enabled: true

xpack.security.transport.ssl.enabled: true
xpack.security.transport.ssl.verification_mode: certificate
xpack.security.transport.ssl.keystore.path: /usr/share/elasticsearch/config/certs/node.jks
xpack.security.transport.ssl.keystore.password: ${KEY_PASS}
xpack.security.transport.ssl.truststore.path: /usr/share/elasticsearch/config/certs/root.jks
xpack.security.transport.ssl.truststore.password: ${KEY_PASS}

xpack.security.http.ssl.keystore.path: /usr/share/elasticsearch/config/certs/client.jks
xpack.security.http.ssl.keystore.password: ${KEY_PASS}
xpack.security.http.ssl.truststore.path: /usr/share/elasticsearch/config/certs/root.jks
xpack.security.http.ssl.truststore.password: ${KEY_PASS}
`

func (es *Elasticsearch) EnsureDefaultConfig() error {
	if err := es.findDefaultConfig(); err != nil {
		return err
	}

	secretMeta := metav1.ObjectMeta{
		Name:      fmt.Sprintf("%v-%v", es.elasticsearch.OffshootName(), DatabaseConfigMapSuffix),
		Namespace: es.elasticsearch.Namespace,
	}

	// set owner reference for the secret.
	// let, elasticsearch object be the owner.
	owner := metav1.NewControllerRef(es.elasticsearch, api.SchemeGroupVersion.WithKind(api.ResourceKindElasticsearch))

	// password for default users: admin, kibanaserver
	inUserConfig, err := es.getInternalUserConfig()
	if err != nil {
		return errors.Wrap(err, "failed to generate default internal user config")
	}

	data := map[string][]byte{
		InternalUserFileName: []byte(inUserConfig),
	}
	_, _, err = core_util.CreateOrPatchSecret(es.kClient, secretMeta, func(in *corev1.Secret) *corev1.Secret {
		in.Labels = core_util.UpsertMap(in.Labels, es.elasticsearch.OffshootLabels())
		core_util.EnsureOwnerReference(&in.ObjectMeta, owner)
		in.Data = data
		return in
	})

	return err
}

func (es *Elasticsearch) findDefaultConfig() error {
	cmName := fmt.Sprintf("%v-%v", es.elasticsearch.OffshootName(), DatabaseConfigMapSuffix)

	configMap, err := es.kClient.CoreV1().ConfigMaps(es.elasticsearch.Namespace).Get(cmName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	if configMap.Labels[api.LabelDatabaseKind] != api.ResourceKindElasticsearch &&
		configMap.Labels[api.LabelDatabaseName] != es.elasticsearch.Name {
		return fmt.Errorf(`intended configMap "%v/%v" already exists`, es.elasticsearch.Namespace, cmName)
	}

	return nil
}

func (es *Elasticsearch) getInternalUserConfig() (string, error) {
	dbSecret := es.elasticsearch.Spec.DatabaseSecret
	if dbSecret == nil {
		return "", errors.New("database secret is empty")
	}

	secret, err := es.kClient.CoreV1().Secrets(es.elasticsearch.GetNamespace()).Get(dbSecret.SecretName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get database secret")
	}

	adminPH, err := generatePasswordHash("admin")
	if err != nil {
		return "", err
	}
	if value, ok := secret.Data[KeyAdminPassword]; ok {
		adminPH, err = generatePasswordHash(string(value))
		if err != nil {
			return "", err
		}
	}

	kibanaserverPH, err := generatePasswordHash("kibanaserver")
	if err != nil {
		return "", err
	}
	if value, ok := secret.Data[KeyKibanaServerPassword]; ok {
		kibanaserverPH, err = generatePasswordHash(string(value))
		if err != nil {
			return "", err
		}
	}

	kibanaroPH, err := generatePasswordHash("kibanaro")
	if err != nil {
		return "", nil
	}

	logstashPH, err := generatePasswordHash("logstash")
	if err != nil {
		return "", nil
	}

	readallPH, err := generatePasswordHash("readall")
	if err != nil {
		return "", nil
	}

	snapshotrestorePH, err := generatePasswordHash("snapshotrestore")
	if err != nil {
		return "", nil
	}

	return fmt.Sprintf(internalUserConfigFile,
		adminPH,
		kibanaserverPH,
		kibanaroPH,
		logstashPH,
		readallPH,
		snapshotrestorePH), nil
}

func generatePasswordHash(password string) (string, error) {
	pHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(pHash), nil
}
