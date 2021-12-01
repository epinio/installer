package installer

import (
	"context"
	"time"

	"github.com/epinio/installer/internal/duration"
	"github.com/epinio/installer/internal/kubernetes"
	"github.com/go-logr/logr"
)

type ComponentActions struct {
	cluster *kubernetes.Cluster
	log     logr.Logger
	timeout time.Duration
}

// NewComponentActions returns the runner for component actions, like checks and waitFors
func NewComponentActions(cluster *kubernetes.Cluster, log logr.Logger, timeout time.Duration) *ComponentActions {
	return &ComponentActions{
		cluster: cluster,
		log:     log,
		timeout: timeout,
	}
}

func (ca ComponentActions) Run(ctx context.Context, c Component, chk ComponentAction) error {
	namespace := c.Namespace
	if chk.Namespace != "" {
		namespace = chk.Namespace
	}
	switch chk.Type {
	case Pod:
		if err := ca.cluster.WaitForPodBySelector(ctx, namespace, chk.Selector, duration.ToPodReady()); err != nil {
			return err
		}
	case Loadbalancer:
		return ca.cluster.WaitUntilServiceHasLoadBalancer(ctx, namespace, chk.Selector, duration.ToServiceLoadBalancer())
	case CRD:
		return ca.cluster.WaitForCRD(ctx, chk.Selector, ca.timeout)
	case Job:
		if err := ca.cluster.WaitForJobCompleted(ctx, namespace, chk.Selector, ca.timeout); err != nil {
			return err
		}
	}

	return nil
}
