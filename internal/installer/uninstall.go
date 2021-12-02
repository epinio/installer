package installer

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/epinio/installer/internal/kubernetes"
	"github.com/go-logr/logr"
)

type Uninstall struct {
	cluster *kubernetes.Cluster
	log     logr.Logger
	ca      *ComponentActions
}

var _ Action = &Uninstall{}

func NewUninstall(cluster *kubernetes.Cluster, log logr.Logger, ca *ComponentActions) *Uninstall {
	return &Uninstall{
		ca:      ca,
		cluster: cluster,
		log:     log,
	}
}

func (u Uninstall) Apply(ctx context.Context, c Component) error {
	log := u.log.WithValues("component", c.ID, "type", c.Type)
	log.Info("apply uninstall")

	for _, chk := range c.PreDelete {
		log.V(2).Info("pre deploy", "checkType", string(chk.Type))
		if err := u.ca.Run(ctx, c, chk); err != nil {
			return err
		}
	}

	switch c.Type {
	case Helm:
		{
			if err := helmUninstall(log.V(1).WithName("helm"), c); err != nil {
				return err
			}
		}

	case YAML:
		{
			if err := yamlDelete(log.V(1).WithName("yaml"), c); err != nil {
				return err
			}
		}

	case Namespace:
		{
			if err := u.cluster.DeleteNamespace(ctx, c.Namespace); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
	}

	return nil
}
