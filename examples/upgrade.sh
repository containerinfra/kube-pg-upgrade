#!/bin/bash
set -euo pipefail

# Example: upgrade a Bitnami PostgreSQL Helm release from 11 to 15.
# Bitnami runs as UID/GID 1001; pass matching security context flags.
#
# Prerequisites: kubectl, helm, and kube-pg-upgrade on PATH.
# Create the namespace first if needed: kubectl create namespace db-upgrade-test

helm -n db-upgrade-test upgrade --wait -i test-db \
  -f examples/values-pg-11.yaml \
  --version=11.9.13 \
  bitnami/postgresql

kube-pg-upgrade upgrade sts -n db-upgrade-test \
  --version=15 \
  --target-pvc-name data-test-db-postgresql-0 \
  --size 10Gi \
  --run-as-user-id=1001 \
  --run-as-group-id=1001 \
  --fs-group=1001 \
  test-db-postgresql

helm -n db-upgrade-test upgrade --wait -i test-db \
  -f examples/values-pg-15.yaml \
  --version=12.12.10 \
  bitnami/postgresql
