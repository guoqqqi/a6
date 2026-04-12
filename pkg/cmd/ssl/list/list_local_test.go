package list

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSNI(t *testing.T) {
	assert.Equal(t, "one.example.com, two.example.com", formatSNI(nil, []string{"one.example.com", "two.example.com"}))

	sni := "single.example.com"
	assert.Equal(t, "single.example.com", formatSNI(&sni, nil))
}

func TestFormatStatus(t *testing.T) {
	assert.Equal(t, "enabled", formatStatus(nil))
	enabled := 1
	disabled := 0
	assert.Equal(t, "enabled", formatStatus(&enabled))
	assert.Equal(t, "disabled", formatStatus(&disabled))
}

func TestParseCertValidity(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	modRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))

	certBytes, err := os.ReadFile(filepath.Join(modRoot, "test/e2e/testdata/test.crt"))
	require.NoError(t, err)

	assert.NotEqual(t, "-", parseCertValidity(string(certBytes)))
	assert.Equal(t, "-", parseCertValidity("invalid-cert"))
}
