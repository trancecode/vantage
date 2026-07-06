package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRngSameSeedSameSequence(t *testing.T) {
	a := NewRng(42, 1024)
	b := NewRng(42, 1024)
	for range 20 {
		assert.Equal(t, a.Uint64(), b.Uint64())
	}
}

func TestRngMarshalResumesSequence(t *testing.T) {
	g := NewRng(7, 9)
	for range 5 {
		g.Uint64() // advance
	}

	state, err := g.MarshalBinary()
	require.NoError(t, err)

	var want []uint64
	for range 10 {
		want = append(want, g.Uint64())
	}

	restored := NewRng(0, 0)
	require.NoError(t, restored.UnmarshalBinary(state))
	var got []uint64
	for range 10 {
		got = append(got, restored.Uint64())
	}
	assert.Equal(t, want, got)
}
