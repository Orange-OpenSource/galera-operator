#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})

sed -e "s|<TEST_NAMESPACE>|${TESTING_NAMESPACE}|g" ${SCRIPT_ROOT}/test-manifests-template/cluster-role-binding-template.yaml > ${SCRIPT_ROOT}/test-manifests/50-cluster-role-binding.yaml

#kubectl apply -f ${SCRIPT_ROOT}/test-manifests