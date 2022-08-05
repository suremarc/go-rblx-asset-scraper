package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroups(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		var g Groups
		require.NoError(t, g.UnmarshalText([]byte("1-10,11-20")))
		assert.Equal(t, Groups{NewGroup(1, 10), NewGroup(11, 20)}, g)
	})

	t.Run("pop", func(t *testing.T) {
		type testCase struct {
			grps     Groups
			n        int
			expected Groups
		}

		grps := Groups{NewGroup(1, 10), NewGroup(13, 101), NewGroup(1239, 1241)}

		cases := []testCase{
			{
				grps:     append(Groups{}, grps...),
				n:        50,
				expected: Groups{NewGroup(1239, 1241), NewGroup(55, 101)},
			},
		}

		for _, c := range cases {
			result := c.grps.Pop(c.n)
			assert.Equal(t, c.expected, result)
		}
	})
}
