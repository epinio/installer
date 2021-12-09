package installer

import (
	"context"

	"github.com/epinio/installer/internal/kubernetes"
	"github.com/go-logr/logr"
)

type Upgrade struct {
	cluster *kubernetes.Cluster
	log     logr.Logger
	ca      *ComponentActions
}

var _ Action = &Upgrade{}

func NewUpgrade(cluster *kubernetes.Cluster, log logr.Logger, ca *ComponentActions) *Upgrade {
	return &Upgrade{
		ca:      ca,
		cluster: cluster,
		log:     log,
	}
}

func (u Upgrade) Apply(ctx context.Context, c Component) error {
	log := u.log.WithValues("component", c.ID, "type", c.Type)
	log.Info("apply upgrade")

	for _, chk := range c.PreUpgrade {
		log.V(2).Info("pre upgrade", "checkType", string(chk.Type))
		if err := u.ca.Run(ctx, c, chk); err != nil {
			return err
		}
	}

	switch c.Type {
	case Helm:
		{
			if err := helmUpdate(log.V(1).WithName("helm"), c); err != nil {
				return err
			}
		}

	case YAML:
		{
			if err := yamlApply(log.V(1).WithName("yaml"), c); err != nil {
				return err
			}
		}

	case Namespace:
		{
			if err := namespaceUpsert(ctx, u.cluster, c); err != nil {
				return err
			}
		}
	}

	for _, chk := range c.WaitComplete {
		log.V(2).Info("wait complete", "checkType", string(chk.Type))

		if err := u.ca.Run(ctx, c, chk); err != nil {
			return err
		}
	}

	return nil
}
