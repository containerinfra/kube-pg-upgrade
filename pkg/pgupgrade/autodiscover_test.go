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
