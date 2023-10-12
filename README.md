# Kube PG Upgrade

CLI Tool to assist with upgrading a Postgres installation running on Kubernetes.

> WARNING: This is currently in an early development state. Please use at your own risk!

## Problem statement

While it's really easy to start-up new Postgres deployments for applications, clusters, environments, etc, they all eventually require to be updated. Not every Postgres Deployment is managed by an operator and upgrading those deployments may involve a lot of manual work. Having encountered 100s of Postgres installations deployed using Bitnami Postgres Helm chart, and some Docker Hub library Postgres Statefulsets, for which Postgres major version upgrades often involve an export and import using pg_dump and some manual laber, I wanted to provide a quick and easy solution for anyone running Postgres on Kubernetes that wishes to upgrade their Postgres installation.

This solution is for anyone that runs the Bitnami or Docker Hub library Postgres images. It builds on-top of the postgres-upgrade images from [tianon/postgres-upgrade](https://github.com/tianon/docker-postgres-upgrade).

## Important notes

While this solution has been tested in various clusters and set-ups (all Bitnami & Docker Hub Postgres based), it is not fool-proof. 
1. Please make sure that you have made **proper back-ups** before you run the upgrade command against your Postgres installations on Kubernetes.
2. Please ensure you test your upgrade proces using this tool against a production-like setup in for example a staging or test environment before running it on your production clusters.

## Installation

```bash
{
    make build;
    cp bin/kube-pg-upgrade /usr/local/bin/kube-pg-upgrade;
    kube-pg-upgrade -h;
}
```

## Usage

```bash
❯ kube-pg-upgrade upgrade sts -h
Usage:
  kube-pg-upgrade pgupgrade statefulset <statefulset> [flags]

Aliases:
  statefulset, sts

Examples:
# run pg_upgrade for a statefulset pod
kube-pg-upgrade upgrade sts database-postgresql --version=15 --current-version=11

Flags:
      --current-version string     current version of the postgres database. Optional, will attempt auto discovery if left empty. For example: 9.6, 14, 15, 16, etc..
  -i, --extra-initdb-args string   provide any additional arguments for init-db. Use the same arguments that were provided when the database was originally created. See https://www.postgresql.org/docs/current/pgupgrade.html. Otherwise will attempt to auto detect.
  -h, --help                       help for statefulset
  -n, --namespace string           namespace of the postgres instance. Default is the configured namespace in your kubecontext.
      --size string                New size. Example: 10G
      --source-pvc-name string     The name of the Persistent Volume Claim with the current postgres data. Optional, will attempt auto discovery if left empty.
      --subpath string             subpath used for mounting the pvc
      --target-pvc-name string     Target name of Persistent Volume Claim that will serve as the target for the upgraded postgres data. This is an optional setting, will use the source PVC name by default.
      --timeout duration           The length of time to wait before giving up, zero means infinite
      --upgrade-image string       Container image used to run pg_upgrade. (default "tianon/postgres-upgrade")
  -u, --user string                user used for initdb
  -v, --version string             target postgres major version. For example: 14, 15, 16, etc..
```

### Example

```bash
helm -n db-upgrade-test upgrade --wait -i test-db -f values-pg-11.yaml --version=8.9.4 bitnami/postgresql

kube-pg-upgrade upgrade sts -n db-upgrade-test \
    --version=15 \
    --target-pvc-name data-test-db-postgresql-primary-0 \
    --size 10Gi \
    test-db-postgresql-master

helm -n db-upgrade-test upgrade --wait -i test-db -f values-pg-15.yaml --version=12.12.10 bitnami/postgresql
```

### Documentation

- [documentation](docs/kube-pg-upgrade.md)
