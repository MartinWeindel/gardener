package cert

import (
	"context"
	"fmt"
	"time"

	certmanv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// componentName is the name of the cert component.
	componentName = "cert"
	// certificatesManagedResourceName is the name of the certificates ManagedResource.
	certificatesManagedResourceName = "cert-certificates"
	// seedCertificatesManagedResourceName is the name of the unmanaged seed certificates ManagedResource.
	seedCertificatesManagedResourceName = "cert-unmanaged-seed-certificates"
	// DefaultCertName is the name of the controlplane cert.
	DefaultCertName = "tls"

	appName = "app.kubernetes.io/name"
)

// Interface contains functions for deploying the certificates
type Interface interface {
	component.DeployWaiter
	DeployCertUnmanagedSeeds(ctx context.Context, virtualClient client.Client) error
}

type cert struct {
	values        Values
	runtimeClient client.Client
}

// Values is a set of configuration values for the cert component.
type Values struct {
	DNS       operatorv1alpha1.GardenDNS
	Namespace string
	Disabled  bool
}

var injectedLabels = map[string]string{appName: componentName}

// New creates a new Deployer for the cert component.
func New(
	runtimeClient client.Client,
	values Values,
) Interface {
	return &cert{
		values:        values,
		runtimeClient: runtimeClient,
	}
}

var _ component.DeployWaiter = &cert{}

func (c *cert) Deploy(ctx context.Context) error {
	if c.values.Disabled {
		return nil
	}
	return c.deployCertRuntimeCluster(ctx)
}

func (c *cert) Destroy(ctx context.Context) error {
	if err := deleteManagedResource(ctx, c.runtimeClient, seedCertificatesManagedResourceName); err != nil {
		return err
	}
	return deleteManagedResource(ctx, c.runtimeClient, certificatesManagedResourceName)
}

func (c *cert) Wait(ctx context.Context) error {
	if err := managedresources.WaitUntilHealthy(ctx, c.runtimeClient, v1beta1constants.GardenNamespace, certificatesManagedResourceName); err != nil {
		return err
	}

	exists, err := hasManagedResource(ctx, c.runtimeClient, seedCertificatesManagedResourceName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return managedresources.WaitUntilHealthy(ctx, c.runtimeClient, v1beta1constants.GardenNamespace, seedCertificatesManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *cert) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, c.runtimeClient, v1beta1constants.GardenNamespace, seedCertificatesManagedResourceName); err != nil {
		return err
	}

	return managedresources.WaitUntilDeleted(timeoutCtx, c.runtimeClient, v1beta1constants.GardenNamespace, certificatesManagedResourceName)
}

func (c *cert) deployCertRuntimeCluster(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	var domainList []string
	for _, domain := range append(c.values.DNS.SecondaryDomains, c.values.DNS.PrimaryDomain) {
		for _, prefix := range []string{"*", "*.ingress"} {
			domainList = append(domainList, fmt.Sprintf("%s.%s", prefix, domain.Name))
		}
	}

	certObj := &certmanv1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultCertName,
			Namespace: c.values.Namespace,
			Labels:    injectedLabels,
		},
		Spec: certmanv1alpha1.CertificateSpec{
			DNSNames: domainList,
			SecretRef: &corev1.SecretReference{
				Name:      DefaultCertName,
				Namespace: c.values.Namespace,
			},
			SecretLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert},
		},
	}

	resources, err := registry.AddAllAndSerialize(certObj)
	if err != nil {
		return err
	}

	if err := createManagedResource(ctx, c.runtimeClient, certificatesManagedResourceName, false, resources); err != nil {
		return fmt.Errorf("creating certificate managedresource failed: %w", err)
	}

	return nil
}

// DeployCertUnmanagedSeeds deploys certificate resources for unmanaged seeds.
func (c *cert) DeployCertUnmanagedSeeds(ctx context.Context, virtualClient client.Client) error {
	if c.values.Disabled {
		return nil
	}
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	managedSeedList := seedmanagementv1alpha1.ManagedSeedList{}
	if err := virtualClient.List(ctx, &managedSeedList); err != nil {
		return err
	}
	managedSeedNames := sets.New[string]()
	for _, managedSeed := range managedSeedList.Items {
		managedSeedNames.Insert(managedSeed.Name)
	}

	seedList := gardencorev1beta1.SeedList{}
	if err := virtualClient.List(ctx, &seedList); err != nil {
		return err
	}

	var objects []client.Object
	for _, seed := range seedList.Items {
		if managedSeedNames.Has(seed.Name) {
			continue
		}
		if seed.Spec.Ingress == nil || seed.Status.KubernetesVersion == nil {
			continue
		}

		certName := fmt.Sprintf("unmanaged-seed-%s-ingress", seed.Name)
		certObj := &certmanv1alpha1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certName,
				Namespace: c.values.Namespace,
				Labels:    injectedLabels,
			},
			Spec: certmanv1alpha1.CertificateSpec{
				DNSNames: []string{"*." + seed.Spec.Ingress.Domain},
				SecretRef: &corev1.SecretReference{
					Name:      certName,
					Namespace: c.values.Namespace,
				},
			},
		}
		objects = append(objects, certObj)
	}

	if len(objects) == 0 {
		return deleteManagedResource(ctx, c.runtimeClient, seedCertificatesManagedResourceName)
	}

	resources, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	if err := createManagedResource(ctx, c.runtimeClient, seedCertificatesManagedResourceName, false, resources); err != nil {
		return fmt.Errorf("creating unmanaged seed certificate managedresource failed: %w", err)
	}

	return nil
}

func hasManagedResource(ctx context.Context, c client.Client, name string) (bool, error) {
	mr := &resourcesv1alpha1.ManagedResource{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: name}, mr); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createManagedResource(ctx context.Context, client client.Client, name string, keepObjects bool, data map[string][]byte) error {
	return managedresources.Create(ctx, client, v1beta1constants.GardenNamespace, name, map[string]string{appName: componentName},
		true, v1beta1constants.SeedResourceManagerClass, data, &keepObjects, map[string]string{appName: componentName}, nil)
}

func deleteManagedResource(ctx context.Context, client client.Client, name string) error {
	return managedresources.Delete(ctx, client, v1beta1constants.GardenNamespace, name, true)
}
