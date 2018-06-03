#!/usr/bin/env bash

set -eou pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

TRIALS_DIR="${DIR}/trials"
LIBRI_CLOUD_DIR="${HOME}/.go/src/github.com/drausin/libri/deploy/cloud"
EXP_NUM="03"
GCP_BUCKET="libri-clusters"
GCP_PROJECT="libri-170711"
export TF_VAR_credentials_file="${HOME}/.gcloud/keys/libri.experimeter.json"

TRIAL_NUM="${1}"
CLUSTER_NAME="exp${EXP_NUM}-trial${TRIAL_NUM}"
CLUSTER_DIR="${DIR}/trials/trial${TRIAL_NUM}"

pushd ${LIBRI_CLOUD_DIR} >/dev/null 2>&1
mkdir -p "${CLUSTER_DIR}"
go run cluster.go init gcp \
    --flagsFilepath "${DIR}/trials/template/terraform.tfvars" \
    --clusterDir "${CLUSTER_DIR}" \
    --clusterName "${CLUSTER_NAME}" \
    --bucket "${GCP_BUCKET}" \
    --gcpProject "${GCP_PROJECT}"

echo "wrote standard experiment flags to ${CLUSTER_DIR}/terraform.tfvars; edit manually and press any key to continue"
read

pushd "${DIR}/../../deploy/cloud" >/dev/null 2>&1
go run gen.go -e "${CLUSTER_DIR}/terraform.tfvars" -d ${CLUSTER_DIR}
popd >/dev/null 2>&1
popd >/dev/null 2>&1

echo -e "cluster ${CLUSTER_NAME} initialized successfully; run the following to start trial\n"
echo "  pushd ${LIBRI_CLOUD_DIR} && go run cluster.go apply --clusterDir ${CLUSTER_DIR}"
echo -e '\nonce all the librarians are up, start the simulator with\n'
echo -e "   kubectl apply -f ${CLUSTER_DIR}/libri-sim.yml\n"

