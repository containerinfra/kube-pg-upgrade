//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	pgPassword     = "e2e-changeme"
	expectedVerify = "1|alpha|first-row\n2|beta|second-row\n3|gamma|third-row"
	defaultSubpath = "pgdata"
)

type dockerHubUpgradeCase struct {
	name string
	// currentVersion / targetVersion drive tianon/postgres-upgrade:CURRENT-to-TARGET.
	currentVersion string
	targetVersion  string
	// subpath is the cluster directory on the PVC (and --subpath). Empty → pgdata.
	subpath string
	// extraInitDBArgs are passed to kube-pg-upgrade (e.g. PG18 checksum defaults).
	extraInitDBArgs string
}

func TestPGUpgradeE2E(t *testing.T) {
	if os.Getenv("KUBECONFIG") == "" {
		t.Fatal("KUBECONFIG must be set (make test-e2e exports kind kubeconfig)")
	}
	require.FileExists(t, binaryPath(t))

	// PG18 initdb enables data checksums by default; older official images do not.
	noChecksums := "--no-data-checksums"

	cases := []dockerHubUpgradeCase{
		// Legacy jump still covered on amd64 CI (upgrade image is amd64-only).
		{name: "11_to_15", currentVersion: "11", targetVersion: "15"},

		// Recent major jumps to PG18 (multi-arch upgrade images).
		{name: "13_to_18", currentVersion: "13", targetVersion: "18", extraInitDBArgs: noChecksums},
		{name: "14_to_18", currentVersion: "14", targetVersion: "18", extraInitDBArgs: noChecksums},
		{name: "15_to_18", currentVersion: "15", targetVersion: "18", extraInitDBArgs: noChecksums},
		{name: "16_to_18", currentVersion: "16", targetVersion: "18", extraInitDBArgs: noChecksums},
		{name: "17_to_18", currentVersion: "17", targetVersion: "18", extraInitDBArgs: noChecksums},

		// Intermediate majors (tianon images currently amd64-only).
		{name: "15_to_17", currentVersion: "15", targetVersion: "17"},
		{name: "16_to_17", currentVersion: "16", targetVersion: "17"},

		// Deployment layout variants.
		{
			name:            "16_to_18_custom_subpath",
			currentVersion:  "16",
			targetVersion:   "18",
			subpath:         "pgcluster",
			extraInitDBArgs: noChecksums,
		},
		{
			name:            "14_to_18_custom_subpath",
			currentVersion:  "14",
			targetVersion:   "18",
			subpath:         "db-files",
			extraInitDBArgs: noChecksums,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testDockerHubSTS(t, tc)
		})
	}
}

// skipUnlessUpgradeImageSupportsHostArch skips when tianon/postgres-upgrade
// lacks a native image for this arch (no binfmt/qemu). Tags targeting 18
// currently publish arm64; older jumps are typically amd64-only.
func skipUnlessUpgradeImageSupportsHostArch(t *testing.T, current, target string) {
	t.Helper()
	if runtime.GOARCH != "arm64" {
		return
	}
	tag := current + "-to-" + target
	arm64OK := map[string]struct{}{
		"13-to-18": {},
		"14-to-18": {},
		"15-to-18": {},
		"16-to-18": {},
		"17-to-18": {},
	}
	if _, ok := arm64OK[tag]; !ok {
		t.Skipf("tianon/postgres-upgrade:%s has no linux/arm64 image; covered on amd64 CI", tag)
	}
}

func testDockerHubSTS(t *testing.T, tc dockerHubUpgradeCase) {
	skipUnlessUpgradeImageSupportsHostArch(t, tc.currentVersion, tc.targetVersion)

	subpath := tc.subpath
	if subpath == "" {
		subpath = defaultSubpath
	}

	ns := fmt.Sprintf("e2e-dh-%s-%d", strings.ReplaceAll(tc.name, "_", "-"), time.Now().UnixNano()%1000000)
	stsName := "postgres"
	pvcName := "data-postgres-0"

	createNamespace(t, ns)

	applyDockerHubSTS(t, ns, dockerHubSTSParams{
		PostgresVersion: tc.currentVersion,
		Replicas:        1,
		Subpath:         subpath,
	})
	waitForPodReady(t, ns, "app=postgres", 5*time.Minute)
	pod := firstPodName(t, ns, "app=postgres")

	applySQLFile(t, ns, pod, "postgres", pgPassword, "app", testdataPath(t, "seed.sql"))

	args := []string{
		"upgrade", "sts",
		"-n", ns,
		"--version=" + tc.targetVersion,
		"--current-version=" + tc.currentVersion,
		"--size", "2Gi",
		"--source-pvc-name", pvcName,
		"--target-pvc-name", pvcName,
		"--subpath", subpath,
		"--timeout", "15m",
	}
	if tc.extraInitDBArgs != "" {
		args = append(args, "--extra-initdb-args="+tc.extraInitDBArgs)
	}
	args = append(args, stsName)
	runUpgradeCLI(t, args...)

	// Swap the pod template while still scaled down, then bring replicas back.
	applyDockerHubSTS(t, ns, dockerHubSTSParams{
		PostgresVersion: tc.targetVersion,
		Replicas:        0,
		Subpath:         subpath,
	})
	kubectl(t, "-n", ns, "scale", "sts/"+stsName, "--replicas=1")
	kubectl(t, "-n", ns, "rollout", "status", "sts/"+stsName, "--timeout=5m")
	waitForPodReady(t, ns, "app=postgres", 5*time.Minute)
	pod = firstPodName(t, ns, "app=postgres")

	out := execPSQL(t, ns, pod, "postgres", pgPassword, "app",
		"SELECT id, name, note FROM e2e_items ORDER BY id;")
	require.Equal(t, expectedVerify, strings.TrimSpace(out))
}
