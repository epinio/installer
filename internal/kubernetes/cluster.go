package kubernetes

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"

	kubeconfig "github.com/epinio/epinio/helpers/kubernetes/config"
	"github.com/epinio/epinio/helpers/termui"

	appsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	restclient "k8s.io/client-go/rest"

	// https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// Memoization of GetCluster
var clusterMemo *Cluster

type Cluster struct {
	//	InternalIPs []string
	//	Ingress     bool
	Kubectl    *kubernetes.Clientset
	RestConfig *restclient.Config
}

// GetCluster returns the Cluster needed to talk to it. On first call it
// creates it from a Kubernetes rest client config and cli arguments /
// environment variables.
func GetCluster(ctx context.Context) (*Cluster, error) {
	if clusterMemo != nil {
		return clusterMemo, nil
	}

	c := &Cluster{}

	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, err
	}

	// copy to avoid mutating the passed-in config
	config := restclient.CopyConfig(restConfig)
	// set the warning handler for this client to ignore warnings
	config.WarningHandler = restclient.NoWarnings{}

	c.RestConfig = restConfig
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c.Kubectl = clientset

	clusterMemo = c

	return clusterMemo, nil
}

// ClientCertManager returns a dynamic namespaced client for the cert manager resource
func (c *Cluster) ClientCertManager() (dynamic.NamespaceableResourceInterface, error) {
	gvr := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "clusterissuers",
	}

	dynamicClient, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return nil, err
	}
	return dynamicClient.Resource(gvr), nil
}

// IsPodRunning returns a condition function that indicates whether the given pod is
// currently running
func (c *Cluster) IsPodRunning(ctx context.Context, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.Kubectl.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

// IsJobCompleted returns a condition function that indicates whether the given
// Job is in Completed state.
func (c *Cluster) IsJobCompleted(ctx context.Context, client *typedbatchv1.BatchV1Client, jobName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		job, err := client.Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range job.Status.Conditions {
			if condition.Type == apibatchv1.JobComplete && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

func (c *Cluster) PodExists(ctx context.Context, namespace, selector string) wait.ConditionFunc {
	return func() (bool, error) {
		podList, err := c.ListPods(ctx, namespace, selector)
		if err != nil {
			return false, err
		}
		if len(podList.Items) == 0 {
			return false, nil
		}
		return true, nil
	}
}

func (c *Cluster) DeploymentExists(ctx context.Context, namespace, deploymentName string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := c.Kubectl.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
}

func (c *Cluster) PodDoesNotExist(ctx context.Context, namespace, selector string) wait.ConditionFunc {
	return func() (bool, error) {
		podList, err := c.ListPods(ctx, namespace, selector)
		if err != nil {
			return true, nil
		}
		if len(podList.Items) > 0 {
			return false, nil
		}
		return true, nil
	}
}

// WaitForCRD wait for a custom resource definition to exist in the cluster.
// It will wait until the CRD reaches the condition "established".
// This method should be used when installing a Deployment that is supposed to
// provide that CRD and want to make sure the CRD is ready for consumption before
// continuing deploying things that will consume it.
func (c *Cluster) WaitForCRD(ctx context.Context, CRDName string, timeout time.Duration) error {
	clientset, err := apiextensions.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		_, err = clientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, CRDName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	// Now wait until the CRD is "established"
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		crd, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, CRDName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitForSecret waits until the specified secret exists. If timeout is reached,
// an error is returned.
// It should be used when something is expected to create a Secret and the code
// needs to wait until that happens.
func (c *Cluster) WaitForSecret(ctx context.Context, namespace, secretName string, timeout time.Duration) (*v1.Secret, error) {
	var secret *v1.Secret
	waitErr := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		var err error
		secret, err = c.GetSecret(ctx, namespace, secretName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})

	return secret, waitErr
}

// Poll up to timeout for pod to enter running state.
// Returns an error if the pod never enters the running state.
func (c *Cluster) WaitForPodRunning(ctx context.Context, namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, c.IsPodRunning(ctx, podName, namespace))
}

func (c *Cluster) WaitForJobCompleted(ctx context.Context, namespace, jobName string, timeout time.Duration) error {
	client, err := typedbatchv1.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}
	return wait.PollImmediate(time.Second, timeout, c.IsJobCompleted(ctx, client, jobName, namespace))
}

// IsDeploymentCompleted returns a condition function that indicates whether the given
// Deployment is in Completed state or not.
func (c *Cluster) IsDeploymentCompleted(ctx context.Context, deploymentName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		deployment, err := c.Kubectl.AppsV1().Deployments(namespace).Get(ctx,
			deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

func (c *Cluster) WaitForDeploymentCompleted(ctx context.Context, ui *termui.UI, namespace, deploymentName string, timeout time.Duration) error {
	if ui != nil {
		s := ui.Progressf("Waiting for deployment %s in %s to be ready", deploymentName, namespace)
		defer s.Stop()
	}

	return wait.PollImmediate(time.Second, timeout, c.IsDeploymentCompleted(ctx, deploymentName, namespace))
}

// ListPods returns the list of currently scheduled or running pods in `namespace` with the given selector
func (c *Cluster) ListPods(ctx context.Context, namespace, selector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}
	podList, err := c.Kubectl.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// WaitForNamespace waits up to timeout for namespace to appear
// Returns an error if the Namespace is not found within the allotted time.
func (c *Cluster) WaitForNamespace(ctx context.Context, ui *termui.UI, namespace string, timeout time.Duration) error {
	s := ui.Progressf("Waiting for namespace %s to be appear", namespace)
	defer s.Stop()

	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		exists, err := c.NamespaceExists(ctx, namespace)
		return exists, err
	})
}

// Wait up to timeout for pod to be removed.
// WaitUntilDeploymentExist waits up to timeout for the specified deployment to exist.
// The Deployment is specified by its name.
func (c *Cluster) WaitUntilDeploymentExists(ctx context.Context, ui *termui.UI, namespace, deploymentName string, timeout time.Duration) error {
	s := ui.Progressf("Waiting for deployment %s in %s to appear", deploymentName, namespace)
	defer s.Stop()

	return wait.PollImmediate(time.Second, timeout, c.DeploymentExists(ctx, namespace, deploymentName))
}

func (c *Cluster) WaitUntilServiceHasLoadBalancer(ctx context.Context, namespace, serviceName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		service, err := c.Kubectl.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return false, nil
		}

		return true, nil
	})
}

// Wait up to timeout for all pods in 'namespace' with given 'selector' to enter running state.
// Returns an error if no pods are found or not all discovered pods enter running state.
func (c *Cluster) WaitUntilPodBySelectorExist(ctx context.Context, namespace, selector string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, c.PodExists(ctx, namespace, selector))
}

// WaitForPodBySelectorRunning waits timeout for all pods in 'namespace'
// with given 'selector' to enter running state. Returns an error if no pods are
// found or not all discovered pods enter running state.
func (c *Cluster) WaitForPodBySelectorRunning(ctx context.Context, namespace, selector string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, c.ArePodsRunning(ctx, selector, namespace))
}

// ArePodsRunning checks that all pods are ready and running for this selector
func (c *Cluster) ArePodsRunning(ctx context.Context, selector string, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		podList, err := c.ListPods(ctx, namespace, selector)
		if err != nil {
			return false, errors.Wrapf(err, "failed listingpods with selector %s", selector)
		}

		// at least one pod must be running
		if len(podList.Items) == 0 {
			return false, nil
		}

		for _, pod := range podList.Items {
			podReady := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady {
					podReady = condition.Status == v1.ConditionTrue
					if !podReady {
						// exit early, podReady conditions's status is false
						return false, nil
					}

					// move to next pod, ignore other conditions
					break
				}
			}
			// exit early, in case the pod was missing the v1.PodReady condition
			if !podReady {
				return false, nil
			}
		}

		return true, nil
	}
}

// WaitForPodBySelector combines WaitUntilPodBySelectorExist and
// WaitForPodBySelectorRunning, as one doesn't check if the pod is ready and
// the other would fail immediately if the pod does not exist.
func (c *Cluster) WaitForPodBySelector(ctx context.Context, namespace, selector string, timeout time.Duration) error {
	// We might hit the deployment in the middle of a rollout. We can't know if this an old pod or a new pod.
	if err := c.WaitUntilPodBySelectorExist(ctx, namespace, selector, timeout); err != nil {
		return err
	}
	// But now some time has passed and we can make sure all pods are running
	if err := c.WaitForPodBySelectorRunning(ctx, namespace, selector, timeout); err != nil {
		return err
	}
	return nil
}

// GetSecret gets a secret's values
func (c *Cluster) GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	return c.Kubectl.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Cluster) GetPodEvents(ctx context.Context, namespace, podName string) (string, error) {
	eventList, err := c.Kubectl.CoreV1().Events(namespace).List(ctx,
		metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + podName,
		})
	if err != nil {
		return "", err
	}

	events := []string{}
	for _, event := range eventList.Items {
		events = append(events, event.Message)
	}

	return strings.Join(events, "\n"), nil
}

// GetVersion get the kube server version
func (c *Cluster) GetVersion() (string, error) {
	v, err := c.Kubectl.ServerVersion()
	if err != nil {
		return "", errors.Wrap(err, "failed to get kube server version")
	}

	return v.String(), nil
}

// ListIngressRoutes returns a list of all routes for ingresses in `namespace` with the given selector
func (c *Cluster) ListIngressRoutes(ctx context.Context, namespace, name string) ([]string, error) {
	ingress, err := c.Kubectl.NetworkingV1().Ingresses(namespace).Get(
		ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ingresses")
	}

	result := []string{}

	for _, rule := range ingress.Spec.Rules {
		result = append(result, rule.Host)
	}

	return result, nil
}

// ListIngress returns the list of available ingresses in `namespace` with the given selector
func (c *Cluster) ListIngress(ctx context.Context, namespace, selector string) (*networkingv1.IngressList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}

	// TODO: Switch to networking v1 when we don't care about <1.18 clusters
	ingressList, err := c.Kubectl.NetworkingV1().Ingresses(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return ingressList, nil
}

// NamespaceExists checks if a namespace exists or not
func (c *Cluster) NamespaceExists(ctx context.Context, namespaceName string) (bool, error) {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// NamespaceLabelExists checks if a specific label exits on the namespace
func (c *Cluster) NamespaceLabelExists(ctx context.Context, namespaceName, labelKey string) (bool, error) {
	namespace, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if _, found := namespace.GetLabels()[labelKey]; found {
		return true, nil
	}

	return false, nil
}

// DeleteNamespace deletes the namepace
func (c *Cluster) DeleteNamespace(ctx context.Context, namespace string) error {
	err := c.Kubectl.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) CreateNamespace(ctx context.Context, name string, labels map[string]string, annotations map[string]string) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Create(
		ctx,
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Labels:      labels,
				Annotations: annotations,
			},
		},
		metav1.CreateOptions{},
	)

	return err
}

func (c *Cluster) GetNamespace(ctx context.Context, name string) (*v1.Namespace, error) {
	ns, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return ns, nil
}

func (c *Cluster) UpdateNamespace(ctx context.Context, name string, labels map[string]string, annotations map[string]string) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Update(
		ctx,
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Labels:      labels,
				Annotations: annotations,
			},
		},
		metav1.UpdateOptions{},
	)

	return err
}

// ClusterIssuerExists checks if the resource exists
func (c *Cluster) ClusterIssuerExists(ctx context.Context, name string) (bool, error) {
	client, err := c.ClientCertManager()
	if err != nil {
		return false, err
	}

	_, err = client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// WaitForClusterIssuer waits up to timeout for ClusterIssuer to exist
func (c *Cluster) WaitForClusterIssuer(ctx context.Context, ui *termui.UI, name string, timeout time.Duration) error {
	if ui != nil {
		s := ui.Progressf("Waiting for ClusterIssuer '%s'", name)
		defer s.Stop()
	}

	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		exists, err := c.ClusterIssuerExists(ctx, name)
		return exists, err
	})
}
