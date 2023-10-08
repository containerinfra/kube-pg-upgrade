FROM gcr.io/distroless/static:nonroot

COPY kube-pg-upgrade /usr/local/bin/kube-pg-upgrade
ENTRYPOINT ["/usr/local/bin/kube-pg-upgrade"]
