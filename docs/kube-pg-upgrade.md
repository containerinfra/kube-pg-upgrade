# Documentation

## Introduction

`kube-pg-upgrade` is a CLI tool designed to make the upgrade of PostgreSQL databases within Kubernetes environments easier. By automating the upgrade process through pg_upgrade, it minimizes downtime and ensures data integrity. `kube-pg-upgrade` supports both PostgreSQL container images sourced from both Bitnami and Docker Hub. By automating the pg_ugprade step allowing DevOps teams to efficiently manage their PostgreSQL instances without the usual complexities and risks associated with database migrations.

## Background Concepts

- **PostgreSQL (postgres):** An open-source relational database management system (RDBMS).
- **pg_upgrade:** A PostgreSQL utility that simplifies the process of upgrading a PostgreSQL cluster to a new major version, avoiding the need for cumbersome dump and reload methods.
- **Kubernetes Persistent Volume Claims (PVCs):** A request for storage resources within a Kubernetes cluster, crucial for managing persistent disk storage required by stateful applications like databases, ensuring data continuity and reliability.


## Important notes (PLEASE READ)

1. **Backups**: Before initiating any upgrade, it's crucial to have a comprehensive backup of your PostgreSQL database. This ensures you have a fallback option should anything go wrong during the upgrade process.
2. **Test Upgrade**: Prior to upgrading a production database, it's highly recommended to perform a test upgrade on a non-production environment. This helps in identifying potential issues and ensures smooth transitions for production databases.

## Upgrade Flow

The upgrade process followed by `kube-pg-upgrade` involves the following steps:

1. Mount Existing Persistent Volume Claim (PVC): The tool mounts the existing PostgreSQL Persistent Volume Claim (PVC).
2. Validation: Before proceeding, kube-pg-upgrade performs validations to ensure compatibility and readiness for the upgrade process.
3. PVC Creation: A new PVC is created to host the upgraded PostgreSQL data.
4. Copy the Postgres data using `pg_upgrade`: The tool employs [pg_upgrade](https://www.postgresql.org/docs/current/pgupgrade.html) to copy and upgrade data from the old Postgres installation PVC to the new PVC.
5. PVC Name Switch: Post-upgrade, the new PVC assumes the name of the old PVC ensuring application continuity without the need for configuration changes.
6. Retention of Old PVC: Even after the upgrade, the old PVC isn't discarded. Instead, it remains available within the cluster as a Persistent Volume (PV) using a "Retain" delete policy, safeguarding your older data.

## Main Commands

Run the kube-pg-upgrade tool with the desired command to perform specific operations:

- completion: Generate the autocompletion script for a specified shell.
- help: Get help about any command.
- `pgupgrade statefulset`: Perform a PostgreSQL upgrade in Kubernetes.
- version: Print version information for the tool.

## Upgrade PostgreSQL Using pg_upgrade

To perform a PostgreSQL upgrade on a Kubernetes cluster:

```bash
kube-pg-upgrade upgrade sts [statefulset_name] [flags]
```

```bash
kube-pg-upgrade upgrade sts database-postgresql --version=15
```

Available flags:

- `--current-version`: Define the current version of the PostgreSQL database (e.g., 9.6, 14, 15). If left empty, the tool will attempt to auto-discover the version.
- `--extra-initdb-args`: If any additional arguments were used when the database was initially created using init-db, specify them here. Refer to the official pg_upgrade documentation for more details. If left blank, the tool will attempt auto-detection.
- `--fs-group`: Group ID that owns the mounted volume. Defaults to `999` (Docker Hub Postgres). Use `1001` for Bitnami.
- `--namespace`: Define the Kubernetes namespace of the PostgreSQL instance. By default, the namespace configured in your kubecontext will be used.
- `--run-as-group-id`: Group ID for the upgrade containers. Defaults to `999` (Docker Hub Postgres). Use `1001` for Bitnami.
- `--run-as-non-root`: Run upgrade containers as a non-root user (default `true`).
- `--run-as-user-id`: User ID for the upgrade containers. Defaults to `999` (Docker Hub Postgres). Use `1001` for Bitnami.
- `--size`: Specify the new size for the upgrade, e.g., 10G.
- `--source-pvc-name`: Name of the PVC with the current PostgreSQL data. If left empty, auto-discovery will be attempted.
- `--subpath`: Define the subpath used for mounting the PVC.
- `--target-pvc-name`: Optional. Specify the name of the target PVC for the upgraded PostgreSQL data. By default, the source PVC name will be used.
- `--timeout`: Set a timeout duration for the upgrade process. A value of zero implies an infinite wait.
- `--upgrade-image`: Define the container image to be used for running pg_upgrade. The default is tianon/postgres-upgrade.
- `--user`: Specify the user for initdb.
- `--version`: Define the target major version for PostgreSQL (e.g., 14, 15).

## Example

To run `kube-pg-upgrade` and perform a PostgreSQL upgrade within a Kubernetes namespace (Bitnami, UID 1001):

```bash
kube-pg-upgrade upgrade sts -n db-upgrade-test \
    --version=15 \
    --target-pvc-name=data-test-db-postgresql-0 \
    --run-as-user-id=1001 \
    --run-as-group-id=1001 \
    --fs-group=1001 \
    test-db-postgresql
```

A full Helm install → upgrade → chart bump flow is in [`examples/upgrade.sh`](../examples/upgrade.sh).
