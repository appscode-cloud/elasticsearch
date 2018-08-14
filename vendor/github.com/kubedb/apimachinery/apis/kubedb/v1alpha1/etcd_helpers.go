package v1alpha1

import (
	"fmt"

	crdutils "github.com/appscode/kutil/apiextensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

func (p Etcd) OffshootName() string {
	return p.Name
}

func (p Etcd) OffshootSelectors() map[string]string {
	return map[string]string{
		LabelDatabaseName: p.Name,
		LabelDatabaseKind: ResourceKindEtcd,
	}
}

func (p Etcd) OffshootLabels() map[string]string {
	return filterTags(p.OffshootSelectors(), p.Labels)
}

func (p Etcd) ResourceShortCode() string {
	return ResourceCodeEtcd
}

func (p Etcd) ResourceKind() string {
	return ResourceKindEtcd
}

func (p Etcd) ResourceSingular() string {
	return ResourceSingularEtcd
}

func (p Etcd) ResourcePlural() string {
	return ResourcePluralEtcd
}

func (p Etcd) ServiceName() string {
	return p.OffshootName()
}

func (p Etcd) ServiceMonitorName() string {
	return fmt.Sprintf("kubedb-%s-%s", p.Namespace, p.Name)
}

func (p Etcd) Path() string {
	return fmt.Sprintf("/metrics")
}

func (p Etcd) Scheme() string {
	return ""
}

func (p *Etcd) StatsAccessor() mona.StatsAccessor {
	return p
}

func (m *Etcd) GetMonitoringVendor() string {
	if m.Spec.Monitor != nil {
		return m.Spec.Monitor.Agent.Vendor()
	}
	return ""
}

func (p Etcd) CustomResourceDefinition() *apiextensions.CustomResourceDefinition {
	return crdutils.NewCustomResourceDefinition(crdutils.Config{
		Group:         SchemeGroupVersion.Group,
		Plural:        ResourcePluralEtcd,
		Singular:      ResourceSingularEtcd,
		Kind:          ResourceKindEtcd,
		ShortNames:    []string{ResourceCodeEtcd},
		ResourceScope: string(apiextensions.NamespaceScoped),
		Versions: []apiextensions.CustomResourceDefinitionVersion{
			{
				Name:    SchemeGroupVersion.Version,
				Served:  true,
				Storage: true,
			},
		},
		Labels: crdutils.Labels{
			LabelsMap: map[string]string{"app": "kubedb"},
		},
		SpecDefinitionName:      "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1.Etcd",
		EnableValidation:        true,
		GetOpenAPIDefinitions:   GetOpenAPIDefinitions,
		EnableStatusSubresource: EnableStatusSubresource,
		AdditionalPrinterColumns: []apiextensions.CustomResourceColumnDefinition{
			{
				Name:     "Version",
				Type:     "string",
				JSONPath: ".spec.version",
			},
			{
				Name:     "Status",
				Type:     "string",
				JSONPath: ".status.phase",
			},
			{
				Name:     "Age",
				Type:     "date",
				JSONPath: ".metadata.creationTimestamp",
			},
		},
	}, setNameSchema)
}