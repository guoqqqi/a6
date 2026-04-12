package delete

import (
	"testing"

	"github.com/api7/a6/pkg/cmd"
	"github.com/api7/a6/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerDelete_NoArgsNonTTY(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	err := deleteRun(&Options{IO: ios})
	require.Error(t, err)
	assert.Equal(t, "username argument is required (or run interactively in a terminal)", err.Error())
}

func TestConsumerDelete_AllAndLabelMutuallyExclusive(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmd.Factory{IOStreams: ios}

	c := NewCmdDelete(f)
	c.SetArgs([]string{"--all", "--label", "env=test"})
	err := c.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all and --label are mutually exclusive")
}

func TestConsumerDelete_AllWithID(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmd.Factory{IOStreams: ios}

	c := NewCmdDelete(f)
	c.SetArgs([]string{"jack", "--all"})
	err := c.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all cannot be used with a specific ID")
}

func TestConsumerDelete_LabelWithID(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmd.Factory{IOStreams: ios}

	c := NewCmdDelete(f)
	c.SetArgs([]string{"jack", "--label", "env=test"})
	err := c.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--label cannot be used with a specific ID")
}
