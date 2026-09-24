package pgupgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoDiscoverImage(t *testing.T) {

	version, err := AutoDiscoverPostgresVersionFromImage("docker.io/bitnami/postgresql:11.7.0-debian-10-r90")

	require.NoError(t, err)
	assert.Equal(t, "11", version)

	version, err = AutoDiscoverPostgresVersionFromImage("docker.io/bitnami/postgresql:15.0.0-debian-10-r90")

	require.NoError(t, err)
	assert.Equal(t, "15", version)
}

func TestIsPostgresContainerImage(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{image: "postgres:13", want: true},
		{image: "postgres:18", want: true},
		{image: "docker.io/library/postgres:15", want: true},
		{image: "docker.io/bitnami/postgresql:11.7.0-debian-10-r90", want: true},
		{image: "bitnami/postgresql:15.0.0", want: true},
		{image: "nginx:1.25", want: false},
		{image: "ghcr.io/example/app:postgres-sidecar", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			assert.Equal(t, tc.want, isPostgresContainerImage(tc.image))
		})
	}
}
