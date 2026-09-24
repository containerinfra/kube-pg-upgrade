//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	// test/e2e -> repo root
	return filepath.Clean(filepath.Join(wd, "../.."))
}

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "test", "e2e", "testdata", name)
}

func binaryPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("KUBE_PG_UPGRADE_BIN"); p != "" {
		return p
	}
	return filepath.Join(repoRoot(t), "bin", "kube-pg-upgrade")
}

func runCmd(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	t.Logf("+ %s %s", name, strings.Join(args, " "))
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	if strings.TrimSpace(out) != "" {
		t.Logf("output:\n%s", out)
	}
	if err != nil {
		return out, fmt.Errorf("%s %v: %w\n%s", name, args, err, out)
	}
	return out, nil
}

func mustRun(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := runCmd(t, name, args...)
	require.NoError(t, err, out)
	return out
}

func kubectl(t *testing.T, args ...string) string {
	t.Helper()
	return mustRun(t, "kubectl", args...)
}

func createNamespace(t *testing.T, name string) {
	t.Helper()
	mustRun(t, "kubectl", "create", "namespace", name)
	t.Cleanup(func() {
		_, _ = runCmd(t, "kubectl", "delete", "namespace", name, "--ignore-not-found=true", "--wait=false")
	})
}

func waitForPodReady(t *testing.T, namespace, labelSelector string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		out, err := runCmd(t, "kubectl", "-n", namespace, "get", "pods", "-l", labelSelector,
			"-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.status.phase}{' '}{.status.containerStatuses[0].ready}{'|'}{end}")
		last = out
		if err == nil && strings.Contains(out, "Running") && strings.Contains(out, "true") {
			_, err = runCmd(t, "kubectl", "-n", namespace, "wait", "--for=condition=ready", "pod",
				"-l", labelSelector, "--timeout=30s")
			if err == nil {
				// Confirm the container is actually exec-able (avoids Ready/restart races).
				pod := strings.Fields(strings.ReplaceAll(out, "|", " "))[0]
				if _, err := runCmd(t, "kubectl", "-n", namespace, "exec", pod, "-c", "postgres", "--",
					"pg_isready", "-U", "postgres"); err == nil {
					return
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
	dumpPodDiagnostics(t, namespace, labelSelector)
	t.Fatalf("timed out waiting for ready pod with selector %q in %s: last=%s", labelSelector, namespace, last)
}

func dumpPodDiagnostics(t *testing.T, namespace, labelSelector string) {
	t.Helper()
	out, _ := runCmd(t, "kubectl", "-n", namespace, "get", "pods", "-l", labelSelector, "-o", "wide")
	t.Logf("pods:\n%s", out)
	out, _ = runCmd(t, "kubectl", "-n", namespace, "describe", "pods", "-l", labelSelector)
	t.Logf("describe:\n%s", out)
	out, _ = runCmd(t, "kubectl", "-n", namespace, "logs", "-l", labelSelector, "--all-containers", "--tail=100")
	t.Logf("logs:\n%s", out)
}

func firstPodName(t *testing.T, namespace, labelSelector string) string {
	t.Helper()
	out := kubectl(t, "-n", namespace, "get", "pods", "-l", labelSelector, "-o", "jsonpath={.items[0].metadata.name}")
	require.NotEmpty(t, out, "expected at least one pod")
	return out
}

func execPSQL(t *testing.T, namespace, pod, user, password, database, sql string) string {
	t.Helper()
	args := []string{"-n", namespace, "exec", pod, "-c", "postgres", "--",
		"env", "PGPASSWORD=" + password,
		"psql", "-U", user, "-d", database, "-v", "ON_ERROR_STOP=1", "-t", "-A", "-c", sql}
	out, err := runCmd(t, "kubectl", args...)
	if err != nil {
		dumpPodDiagnostics(t, namespace, "app=postgres")
		require.NoError(t, err, out)
	}
	return out
}

func applySQLFile(t *testing.T, namespace, pod, user, password, database, sqlFile string) {
	t.Helper()
	content, err := os.ReadFile(sqlFile)
	require.NoError(t, err)
	cmd := exec.Command("kubectl", "-n", namespace, "exec", "-i", pod, "-c", "postgres", "--",
		"env", "PGPASSWORD="+password,
		"psql", "-U", user, "-d", database, "-v", "ON_ERROR_STOP=1")
	cmd.Stdin = bytes.NewReader(content)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	t.Logf("+ kubectl exec -i %s -- psql < %s", pod, sqlFile)
	err = cmd.Run()
	require.NoError(t, err, stdout.String()+stderr.String())
}

func runUpgradeCLI(t *testing.T, args ...string) {
	t.Helper()
	bin := binaryPath(t)
	require.FileExists(t, bin)
	mustRun(t, bin, args...)
}

type dockerHubSTSParams struct {
	PostgresVersion string
	Replicas        int32
	// MountPath is where the PVC is mounted. PGDATA is MountPath/Subpath.
	MountPath string
	// Subpath is the cluster directory under the PVC mount (and on the PVC).
	Subpath string
	// Pod security context (Docker Hub default is 999).
	RunAsUser    int64
	RunAsGroup   int64
	FSGroup      int64
	RunAsNonRoot bool
}

// applyDockerHubSTS applies Secret+Service+StatefulSet for official postgres.
// The PVC is mounted at MountPath; PGDATA is a subdirectory (MountPath/Subpath)
// so initdb can chmod it (chmod on a volume mount point returns EPERM).
func applyDockerHubSTS(t *testing.T, namespace string, p dockerHubSTSParams) {
	t.Helper()
	require.NotEmpty(t, p.PostgresVersion)
	require.NotEmpty(t, p.Subpath)
	require.True(t, p.Replicas == 0 || p.Replicas == 1, "replicas must be 0 or 1")
	if p.MountPath == "" {
		p.MountPath = defaultMountPath
	}
	if p.RunAsUser == 0 {
		p.RunAsUser = 999
	}
	if p.RunAsGroup == 0 {
		p.RunAsGroup = 999
	}
	if p.FSGroup == 0 {
		p.FSGroup = 999
	}
	pgdata := strings.TrimSuffix(p.MountPath, "/") + "/" + strings.TrimPrefix(p.Subpath, "/")

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: postgres-secret
type: Opaque
stringData:
  POSTGRES_PASSWORD: e2e-changeme
  POSTGRES_DB: app
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  ports:
    - port: 5432
      name: postgres
  clusterIP: None
  selector:
    app: postgres
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: %d
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      securityContext:
        fsGroup: %d
        runAsUser: %d
        runAsGroup: %d
        runAsNonRoot: %t
      containers:
        - name: postgres
          image: postgres:%s
          ports:
            - containerPort: 5432
              name: postgres
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-secret
                  key: POSTGRES_PASSWORD
            - name: POSTGRES_DB
              valueFrom:
                secretKeyRef:
                  name: postgres-secret
                  key: POSTGRES_DB
            - name: PGDATA
              value: %s
          volumeMounts:
            - name: tmp
              mountPath: /tmp
            - name: data
              mountPath: %s
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "postgres"]
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
      volumes:
        - name: tmp
          emptyDir: {}
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 2Gi
`, p.Replicas, p.FSGroup, p.RunAsUser, p.RunAsGroup, p.RunAsNonRoot, p.PostgresVersion, pgdata, p.MountPath)

	cmd := exec.Command("kubectl", "-n", namespace, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	t.Logf("+ kubectl -n %s apply -f - (postgres:%s replicas=%d mount=%s pgdata=%s uid=%d)",
		namespace, p.PostgresVersion, p.Replicas, p.MountPath, pgdata, p.RunAsUser)
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	if strings.TrimSpace(out) != "" {
		t.Logf("output:\n%s", out)
	}
	require.NoError(t, err, out)
}

func assertPodRunsAs(t *testing.T, namespace, pod string, wantUID int64) {
	t.Helper()
	out := kubectl(t, "-n", namespace, "exec", pod, "-c", "postgres", "--", "id", "-u")
	require.Equal(t, fmt.Sprintf("%d", wantUID), strings.TrimSpace(out), "postgres container UID")
}

// assertPVCClusterOwnedBy scales the STS down and checks PG_VERSION ownership on the PVC.
func assertPVCClusterOwnedBy(t *testing.T, namespace, pvcName, subpath string, wantUID int64) {
	t.Helper()
	kubectl(t, "-n", namespace, "scale", "sts/postgres", "--replicas=0")
	kubectl(t, "-n", namespace, "wait", "--for=delete", "pod/postgres-0", "--timeout=2m")

	inspectName := fmt.Sprintf("pvc-owner-%d", time.Now().UnixNano()%100000)
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  restartPolicy: Never
  securityContext:
    runAsUser: %d
    runAsGroup: %d
    fsGroup: %d
    runAsNonRoot: true
  containers:
    - name: inspect
      image: busybox:1.36
      command: ["sh", "-c", "stat -c %%u /data/%s/PG_VERSION"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: %s
`, inspectName, wantUID, wantUID, wantUID, subpath, pvcName)

	cmd := exec.Command("kubectl", "-n", namespace, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), stdout.String()+stderr.String())

	deadline := time.Now().Add(2 * time.Minute)
	var last string
	for time.Now().Before(deadline) {
		phase, _ := runCmd(t, "kubectl", "-n", namespace, "get", "pod", inspectName, "-o", "jsonpath={.status.phase}")
		last = phase
		if strings.TrimSpace(phase) == "Succeeded" || strings.TrimSpace(phase) == "Failed" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.Equal(t, "Succeeded", strings.TrimSpace(last), "inspect pod phase")
	uidOut := kubectl(t, "-n", namespace, "logs", inspectName)
	require.Equal(t, fmt.Sprintf("%d", wantUID), strings.TrimSpace(uidOut),
		"upgraded cluster files should be owned by the configured runAsUser")
	_, _ = runCmd(t, "kubectl", "-n", namespace, "delete", "pod", inspectName, "--wait=false")
}
