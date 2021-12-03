package installer

import (
	"context"

	"github.com/epinio/installer/internal/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func namespaceUpsert(ctx context.Context, cluster *kubernetes.Cluster, c Component) error {
	labels := map[string]string{}
	annotations := map[string]string{}
	for _, val := range c.Values {
		switch val.Type {
		case Annotation:
			annotations[val.Name] = val.Value
		case Label:
			labels[val.Name] = val.Value
		}
	}
	if err := cluster.CreateNamespace(ctx, c.Namespace, labels, annotations); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ns, err := cluster.GetNamespace(ctx, c.Namespace)
			if err != nil {
				return err
			}

			if ns.Labels == nil {
				ns.Labels = map[string]string{}
			}
			for n, v := range labels {
				ns.Labels[n] = v
			}

			if ns.Annotations == nil {
				ns.Annotations = map[string]string{}
			}
			for n, v := range annotations {
				ns.Annotations[n] = v
			}
			return cluster.UpdateNamespace(ctx, c.Namespace, ns.Labels, ns.Annotations)
		}
		return err
	}
	return nil

}
