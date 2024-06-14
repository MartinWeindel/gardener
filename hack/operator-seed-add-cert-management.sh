#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

PATH_KIND_KUBECONFIG=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --path-kind-kubeconfig)
      shift; PATH_KIND_KUBECONFIG="$1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

create_ca_secret() {
  # create root CA, following https://github.com/gardener/cert-management?tab=readme-ov-file#certificate-authority-ca
  openssl genrsa -out /tmp/CA-key.pem 4096
  export CONFIG="
[req]
distinguished_name=dn
[ dn ]
[ ext ]
basicConstraints=CA:TRUE,pathlen:0
"
  openssl req \
    -new -nodes -x509 -config <(echo "$CONFIG") -key /tmp/CA-key.pem \
    -subj "/CN=LocalGarden" -extensions ext -days 1000 -out /tmp/CA-cert.pem
  kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" -n garden create secret tls issuer-ca-secret \
    --cert=/tmp/CA-cert.pem --key=/tmp/CA-key.pem
}

kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" -n garden get secret issuer-ca-secret > /dev/null || create_ca_secret

cat <<EOF | kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" patch garden local --type merge --patch-file /dev/stdin
spec:
  runtimeCluster:
    certManagement:
      defaultIssuer:
        ca:
          secretRef:
            name: issuer-ca-secret
EOF