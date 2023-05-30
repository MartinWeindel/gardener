#!/usr/bin/env bash

PROJECT=local
SHOOT=local

NS=shoot--$PROJECT--$SHOOT
PROJECT_NS=garden-$PROJECT

NODE=$1

kubectl -n $NS exec -i $NODE -- /bin/sh -c "echo 127.0.0.1 api.local.local.internal.local.gardener.cloud >> /etc/hosts"
kubectl -n $NS exec -i $NODE -- /bin/sh -c "echo 127.0.0.1 api.local.local.external.local.gardener.cloud >> /etc/hosts"

echo -n '{
  "apiVersion": "authentication.gardener.cloud/v1alpha1",
  "kind": "AdminKubeconfigRequest",
  "spec": {
    "expirationSeconds": 36000
  }
}' | kubectl create -f - --raw /apis/core.gardener.cloud/v1beta1/namespaces/${PROJECT_NS}/shoots/${SHOOT}/adminkubeconfig | jq -r ".status.kubeconfig" | base64 -d > kubeconfig

kubectl -n $NS cp ./kubeconfig $NODE:/kubeconfig

for staticpod in staticpod-kube-apiserver staticpod-kube-controller-manager staticpod-kube-scheduler staticpod-etcd-main-0 staticpod-etcd-events-0; do
  kubectl -n $NS exec $NODE -- /bin/sh -c "$(kubectl -n $NS get configmap $staticpod -o yaml | yq .data.script)"
done

kubectl -n $NS exec -i $NODE -- /bin/sh -c "cp /kubeconfig /var/lib/staticpods/volumes/kube-controller-manager/kubeconfig/kubeconfig"
kubectl -n $NS exec -i $NODE -- /bin/sh -c "cp /kubeconfig /var/lib/staticpods/volumes/kube-scheduler/kubeconfig/kubeconfig"
kubectl -n $NS exec -i $NODE -- /bin/sh -c "cp /kubeconfig /var/lib/kubelet/kubeconfig-real"
