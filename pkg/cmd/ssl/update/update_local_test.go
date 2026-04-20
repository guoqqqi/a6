package update

import (
	"testing"

	"github.com/api7/a6/pkg/cmd"
	"github.com/api7/a6/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSLUpdate_NoArgsNonTTY(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	err := updateRun(&Options{IO: ios})
	require.Error(t, err)
	assert.Equal(t, "id argument is required (or run interactively in a terminal)", err.Error())
}

func TestSSLUpdate_NoFile(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmd.Factory{IOStreams: ios}

	c := NewCmdUpdate(f)
	c.SetArgs([]string{"ssl-1"})
	err := c.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}
