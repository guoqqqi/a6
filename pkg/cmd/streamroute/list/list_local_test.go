package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDerefStr(t *testing.T) {
	assert.Equal(t, "", derefStr(nil))
	v := "stream-1"
	assert.Equal(t, "stream-1", derefStr(&v))
}

func TestDerefInt(t *testing.T) {
	assert.Equal(t, "", derefInt(nil))
	v := 9100
	assert.Equal(t, "9100", derefInt(&v))
}
