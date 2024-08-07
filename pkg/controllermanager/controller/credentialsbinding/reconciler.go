// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles CredentialsBinding.
type Reconciler struct {
	Client   client.Client
	Config   config.CredentialsBindingControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile reconciles CredentialsBinding.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	credentialsBinding := &securityv1alpha1.CredentialsBinding{}
	if err := r.Client.Get(ctx, request.NamespacedName, credentialsBinding); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels a CredentialsBinding as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the CredentialsBinding anymore.
	// When this happens the controller will remove the finalizers from the CredentialsBinding so that it can be garbage collected.
	if credentialsBinding.DeletionTimestamp != nil {
		return reconcile.Result{}, r.delete(ctx, credentialsBinding, log)
	}

	return reconcile.Result{}, r.reconcile(ctx, credentialsBinding, log)
}

func (r *Reconciler) reconcile(ctx context.Context, credentialsBinding *securityv1alpha1.CredentialsBinding, log logr.Logger) error {
	if !controllerutil.ContainsFinalizer(credentialsBinding, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, credentialsBinding, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	// TODO(dimityrmirchev): this code should eventually handle workload identities as a valid credential ref
	// Add the Gardener finalizer to the referenced Secret
	// to protect it from deletion as long as the CredentialsBinding resource exists.
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: credentialsBinding.CredentialsRef.Namespace, Name: credentialsBinding.CredentialsRef.Name}, secret); err != nil {
		return err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.Client, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			return fmt.Errorf("could not add finalizer to secret: %w", err)
		}
	}

	providerTypeLabelKey := v1beta1constants.LabelShootProviderPrefix + credentialsBinding.Provider.Type
	hasProviderKeyLabel := metav1.HasLabel(secret.ObjectMeta, providerTypeLabelKey)
	hasCredentialsBindingRefLabel := metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference)
	if !hasProviderKeyLabel || !hasCredentialsBindingRefLabel {
		patch := client.MergeFrom(secret.DeepCopy())
		if !hasProviderKeyLabel {
			metav1.SetMetaDataLabel(&secret.ObjectMeta, providerTypeLabelKey, "true")
		}
		if !hasCredentialsBindingRefLabel {
			metav1.SetMetaDataLabel(&secret.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference, "true")
		}
		if err := r.Client.Patch(ctx, secret, patch); err != nil {
			return fmt.Errorf("failed to add provider type or/and referred labels to Secret referenced in CredentialsBinding: %w", err)
		}
	}

	for _, objRef := range credentialsBinding.Quotas {
		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, quota); err != nil {
			return err
		}

		// Add 'referred by a credentials binding' label
		if !metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
			patch := client.MergeFrom(quota.DeepCopy())
			metav1.SetMetaDataLabel(&quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference, "true")
			if err := r.Client.Patch(ctx, quota, patch); err != nil {
				return fmt.Errorf("failed to add referred label to the quota referenced in CredentialsBinding, quota: %s , namespace: %s : %w", quota.Name, quota.Namespace, err)
			}
		}
	}

	return nil
}

func (r *Reconciler) delete(ctx context.Context, credentialsBinding *securityv1alpha1.CredentialsBinding, log logr.Logger) error {
	if !controllerutil.ContainsFinalizer(credentialsBinding, gardencorev1beta1.GardenerName) {
		return nil
	}

	associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, credentialsBinding)
	if err != nil {
		return err
	}

	if len(associatedShoots) == 0 {
		log.Info("No Shoots are referencing the CredentialsBinding, deletion accepted")

		mayReleaseCredentials, err := r.mayReleaseCredentials(ctx, credentialsBinding)
		if err != nil {
			return err
		}

		if mayReleaseCredentials {
			// TODO(dimityrmirchev): this code should eventually handle workload identities as a valid credential ref
			secret := &corev1.Secret{}
			if err := r.Client.Get(ctx, client.ObjectKey{Namespace: credentialsBinding.CredentialsRef.Namespace, Name: credentialsBinding.CredentialsRef.Name}, secret); err == nil {
				// Remove shoot provider label and 'referred by a credentials binding' label
				hasProviderLabel, providerLabel := getProviderLabel(secret.Labels)
				if hasProviderLabel || metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
					patch := client.MergeFrom(secret.DeepCopy())
					delete(secret.ObjectMeta.Labels, v1beta1constants.LabelCredentialsBindingReference)

					// The secret can be still referenced by a secretbinding so
					// only remove the provider label if there is no secretbinding reference label
					if !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
						delete(secret.ObjectMeta.Labels, providerLabel)
					}

					if err := r.Client.Patch(ctx, secret, patch); err != nil {
						return fmt.Errorf("failed to remove referred label from Secret: %w", err)
					}
				}

				// Remove finalizer from referenced secret
				// only if the secret does not have a secretbinding reference label
				if controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) && !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
					log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
					if err := controllerutils.RemoveFinalizers(ctx, r.Client, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
						return fmt.Errorf("failed to remove finalizer from secret: %w", err)
					}
				}
			} else if !apierrors.IsNotFound(err) {
				return err
			}
		}

		if err := r.removeLabelFromQuotas(ctx, credentialsBinding); err != nil {
			return err
		}

		// Remove finalizer from CredentialsBinding
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, credentialsBinding, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return nil
	}

	message := fmt.Sprintf("Cannot delete CredentialsBinding, because the following Shoots are still referencing it: %+v", associatedShoots)
	r.Recorder.Event(credentialsBinding, corev1.EventTypeWarning, v1beta1constants.EventResourceReferenced, message)
	return errors.New(message)
}

// We may only release a credential if there is no other credentials binding that references it (maybe in a different namespace).
func (r *Reconciler) mayReleaseCredentials(ctx context.Context, binding *securityv1alpha1.CredentialsBinding) (bool, error) {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Client.List(ctx, credentialsBindingList); err != nil {
		return false, err
	}

	for _, cb := range credentialsBindingList.Items {
		// skip if it is one and the same credentials binding
		if cb.Namespace == binding.Namespace && cb.Name == binding.Name {
			continue
		}
		if cb.CredentialsRef.APIVersion == binding.CredentialsRef.APIVersion &&
			cb.CredentialsRef.Kind == binding.CredentialsRef.Kind &&
			cb.CredentialsRef.Namespace == binding.CredentialsRef.Namespace &&
			cb.CredentialsRef.Name == binding.CredentialsRef.Name {
			return false, nil
		}
	}

	return true, nil
}

// Remove the label from the quota only if there is no other credentialsbindings that reference it (maybe in a different namespace).
func (r *Reconciler) removeLabelFromQuotas(ctx context.Context, binding *securityv1alpha1.CredentialsBinding) error {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Client.List(ctx, credentialsBindingList); err != nil {
		return err
	}

	for _, q := range binding.Quotas {
		if quotaHasOtherRef(q, credentialsBindingList, binding.Namespace, binding.Name) {
			continue
		}

		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: q.Namespace, Name: q.Name}, quota); err != nil {
			return err
		}

		// Remove 'referred by a credentials binding' label
		if metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
			patch := client.MergeFromWithOptions(quota.DeepCopy(), client.MergeFromWithOptimisticLock{})
			delete(quota.ObjectMeta.Labels, v1beta1constants.LabelCredentialsBindingReference)
			if err := r.Client.Patch(ctx, quota, patch); err != nil {
				return fmt.Errorf("failed to remove referred label from Quota: %w", err)
			}
		}
	}

	return nil
}

func quotaHasOtherRef(
	quota corev1.ObjectReference,
	credentialsBindingList *securityv1alpha1.CredentialsBindingList,
	credentialsBindingNamespace,
	credentialsBindingName string,
) bool {
	for _, cb := range credentialsBindingList.Items {
		if cb.Namespace == credentialsBindingNamespace && cb.Name == credentialsBindingName {
			continue
		}
		for _, q := range cb.Quotas {
			if q.Name == quota.Name && q.Namespace == quota.Namespace {
				return true
			}
		}
	}

	return false
}

func getProviderLabel(labels map[string]string) (bool, string) {
	for label := range labels {
		if strings.HasPrefix(label, v1beta1constants.LabelShootProviderPrefix) {
			return true, label
		}
	}

	return false, ""
}
