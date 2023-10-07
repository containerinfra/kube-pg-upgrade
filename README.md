# Kube PG Upgrade

CLI Tool to assist with upgrading a Postgres installation running on Kubernetes.

This is currently in an experimental state!

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
❯ kube-pg-upgrade pg upgrade -h
Usage:
  kube-pg-upgrade postgres upgrade <statefulset> [flags]

Examples:
# run pg_upgrade for a statefulset pod
kube-pg-upgrade postgres upgrade database-postgresql --target-version=15 --current-version=11


Flags:
      --current-version string     current version of the postgres database. Optional, will attempt auto discovery if left blank. For example: 9.6, 14, 15, 16, etc..
  -i, --extra-initdb-args string   provide any additional arguments for init-db. Use the same arguments that were provided when the database was originally created. See https://www.postgresql.org/docs/current/pgupgrade.html. Otherwise will attempt to auto detect.
  -h, --help                       help for upgrade
  -n, --namespace string           namespace of the postgres instance. Default is the configured namespace in your kubecontext.
      --size string                New size. Example: 10G
      --target-pvc-name string     Target name of the new pvc. Optional, will use current name by default
  -t, --target-version string      target postgres major version. For example: 14, 15, 16, etc..
      --timeout duration           The length of time to wait before giving up, zero means infinite
  -u, --user string                user used for initdb
```

### Documentation

- [documentation](docs/kube-pg-upgrade.md)
