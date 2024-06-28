// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cert_test

import (
	"context"
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/certmanagement/cert"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Cert", func() {
	var (
		ctx                              = context.Background()
		namespace                        = "some-namespace"
		fakeOps                          *retryfake.Ops
		c                                client.Client
		virtualGardenClient              client.Client
		values                           Values
		managedResourceCertificate       *resourcesv1alpha1.ManagedResource
		managedResourceCertificateSecret *corev1.Secret
		managedResourceSeedCert          *resourcesv1alpha1.ManagedResource
		managedResourceSeedCertSecret    *corev1.Secret

		consistOf func(...client.Object) types.GomegaMatcher

		certificate     *certv1alpha1.Certificate
		seedCertificate *certv1alpha1.Certificate
		unmanagedSeed   *gardencorev1beta1.Seed

		newComponent = func(values Values) Interface {
			return New(c, values)
		}
		checkCertificate  func(expectSeedCertMRToExist bool)
		createSeedObjects func()
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		virtualGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		resetVar := testutils.WithVars(
			&retry.Until, fakeOps.Until,
		)
		DeferCleanup(func() {
			resetVar()
		})

		values = Values{
			Namespace: namespace,
		}

		dnsNameList := []string{fmt.Sprintf("*.%s", values.DNS.PrimaryDomain.Name), fmt.Sprintf("*.ingress.%s", values.DNS.PrimaryDomain.Name)}

		for _, domain := range values.DNS.SecondaryDomains {
			dnsNamesSecondaryDomainIngress := fmt.Sprintf("*.ingress.%s", domain.Name)
			dnsNamesSecondaryDomain := fmt.Sprintf("*.%s", domain.Name)
			dnsNameList = append(dnsNameList, dnsNamesSecondaryDomain, dnsNamesSecondaryDomainIngress)
		}

		certificate = &certv1alpha1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultCertName,
				Namespace: namespace,
				Labels:    map[string]string{"app.kubernetes.io/name": "cert"},
			},
			Spec: certv1alpha1.CertificateSpec{
				DNSNames: dnsNameList,
				SecretRef: &corev1.SecretReference{
					Name:      DefaultCertName,
					Namespace: namespace,
				},
				SecretLabels: map[string]string{"gardener.cloud/role": "controlplane-cert"},
			},
		}

		unmanagedSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: "mysoil"},
			Spec: gardencorev1beta1.SeedSpec{
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "my-soil.example.com",
				},
			},
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: ptr.To("v1.30.1"),
			},
		}

		seedCertificate = &certv1alpha1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unmanaged-seed-mysoil-ingress",
				Namespace: namespace,
				Labels:    map[string]string{"app.kubernetes.io/name": "cert"},
			},
			Spec: certv1alpha1.CertificateSpec{
				DNSNames: []string{"*." + unmanagedSeed.Spec.Ingress.Domain},
				SecretRef: &corev1.SecretReference{
					Name:      "unmanaged-seed-mysoil-ingress",
					Namespace: namespace,
				},
			},
		}

		managedResourceCertificate = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-certificates",
				Namespace: "garden",
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert",
				},
			},
		}
		managedResourceCertificateSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceCertificate.Name,
				Namespace: "garden",
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert",
				},
			},
		}

		managedResourceSeedCert = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-unmanaged-seed-certificates",
				Namespace: "garden",
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert",
				},
			},
		}
		managedResourceSeedCertSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSeedCert.Name,
				Namespace: "garden",
				Labels: map[string]string{
					"app.kubernetes.io/name": "cert",
				},
			},
		}

		checkCertificate = func(expectSeedCertMRToExist bool) {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCertificate), managedResourceCertificate)).To(Succeed())
			expectedMrCertificate := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "cert-certificates",
					Namespace:       "garden",
					ResourceVersion: "1",
					Labels:          map[string]string{"app.kubernetes.io/name": "cert"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:        ptr.To("seed"),
					InjectLabels: map[string]string{"app.kubernetes.io/name": "cert"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceCertificate.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMrCertificate))
			Expect(managedResourceCertificate).To(DeepEqual(expectedMrCertificate))

			managedResourceCertificateSecret.Name = managedResourceCertificate.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCertificateSecret), managedResourceCertificateSecret)).To(Succeed())
			Expect(managedResourceCertificateSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceCertificateSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceCertificateSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(managedResourceCertificate).To(consistOf(certificate))

			err := c.Get(ctx, client.ObjectKeyFromObject(managedResourceSeedCert), managedResourceSeedCert)
			switch expectSeedCertMRToExist {
			case false:
				Expect(errors.IsNotFound(err)).To(BeTrue())
			case true:
				Expect(err).NotTo(HaveOccurred())
				Expect(managedResourceSeedCert).To(consistOf(seedCertificate))
			}
		}

		createSeedObjects = func() {
			err := virtualGardenClient.Create(ctx, &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{Name: "managed"},
			})
			Expect(err).NotTo(HaveOccurred())
			err = virtualGardenClient.Create(ctx, &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: "managed"},
			})
			Expect(err).NotTo(HaveOccurred())
			err = virtualGardenClient.Create(ctx, unmanagedSeed)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy", func() {
			comp := newComponent(values)

			Expect(comp.Deploy(ctx)).To(Succeed())
			checkCertificate(false)
		})

		It("should successfully deploy unmanaged seed certificates", func() {
			comp := newComponent(values)
			createSeedObjects()

			Expect(comp.Deploy(ctx)).To(Succeed())
			Expect(comp.DeployCertUnmanagedSeeds(ctx, virtualGardenClient)).To(Succeed())
			checkCertificate(true)
		})

	})

	Describe("Wait", func() {
		It("should check MR is ready", func() {
			comp := newComponent(values)
			certificate.Status = certv1alpha1.CertificateStatus{State: "Ready"}
			Expect(c.Create(ctx, certificate)).To(Succeed())

			managedResourceCertificate.Generation = 1
			managedResourceCertificate.Status = resourcesv1alpha1.ManagedResourceStatus{
				ObservedGeneration: 1,
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			}
			Expect(c.Create(ctx, managedResourceCertificate)).To(Succeed())

			Expect(comp.Wait(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(certificate), certificate)).To(Succeed())
		})

		It("should check both MRs are ready", func() {
			comp := newComponent(values)

			certificate.Status = certv1alpha1.CertificateStatus{State: "Ready"}
			Expect(c.Create(ctx, certificate)).To(Succeed())
			seedCertificate.Status = certv1alpha1.CertificateStatus{State: "Ready"}
			Expect(c.Create(ctx, seedCertificate)).To(Succeed())

			managedResourceCertificate.Generation = 1
			managedResourceCertificate.Status = resourcesv1alpha1.ManagedResourceStatus{
				ObservedGeneration: 1,
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			}
			Expect(c.Create(ctx, managedResourceCertificate)).To(Succeed())
			managedResourceSeedCert.Generation = 1
			managedResourceSeedCert.Status = resourcesv1alpha1.ManagedResourceStatus{
				ObservedGeneration: 1,
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			}
			Expect(c.Create(ctx, managedResourceSeedCert)).To(Succeed())

			Expect(comp.Wait(ctx)).To(Succeed())
		})

		It("should fail when MR is not healthy", func() {
			fakeOps.MaxAttempts = 2

			comp := newComponent(values)
			Expect(c.Create(ctx, managedResourceCertificate)).To(Succeed())
			Expect(comp.Wait(ctx)).To(MatchError(`retry failed with max attempts reached, last error: managed resource garden/cert-certificates is not healthy`))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should fail when the wait for the managed resource deletion times out", func() {
			fakeOps.MaxAttempts = 2

			comp := newComponent(values)
			Expect(c.Create(ctx, managedResourceCertificate)).To(Succeed())
			Expect(comp.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		It("should not return an error when it's already removed", func() {
			comp := newComponent(values)
			Expect(comp.WaitCleanup(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			comp := newComponent(values)
			Expect(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}})).To(Succeed())
			Expect(c.Create(ctx, managedResourceCertificate)).To(Succeed())
			Expect(c.Create(ctx, managedResourceCertificateSecret)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSeedCert)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSeedCertSecret)).To(Succeed())

			Expect(comp.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCertificate), managedResourceCertificate)).To(BeNotFoundError())
		})
	})
})
