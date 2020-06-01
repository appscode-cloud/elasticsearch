package open_distro

import (
	"fmt"
	"testing"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestElasticsearch_getInternalUserConfig(t *testing.T) {
	type fields struct {
		kClient       kubernetes.Interface
		elasticsearch *api.Elasticsearch
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Check output",
			fields: fields{
				kClient: &fake.Clientset{},
				elasticsearch: &api.Elasticsearch{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-jrjwe",
						Namespace: "test-23wefjds",
					},
					Spec: api.ElasticsearchSpec{
						DatabaseSecret: &corev1.SecretVolumeSource{
							SecretName: "db-secret",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &Elasticsearch{
				kClient:       tt.fields.kClient,
				elasticsearch: tt.fields.elasticsearch,
			}
			_, err := es.kClient.CoreV1().Secrets(es.elasticsearch.Namespace).Create(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-secret",
					Namespace: es.elasticsearch.Namespace,
				},
				Data: map[string][]byte{},
			})
			if err != nil {
				panic(err)
			}
			got, err := es.getInternalUserConfig()
			fmt.Println(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("getInternalUserConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
