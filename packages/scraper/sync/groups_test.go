package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroups(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		var g groups
		require.NoError(t, g.parse("1-10,11-20"))
		assert.Equal(t, groups{newGroup(1, 10), newGroup(11, 20)}, g)
	})

	t.Run("pop", func(t *testing.T) {
		type testCase struct {
			grps     groups
			n        int
			expected groups
		}

		grps := groups{newGroup(1, 10), newGroup(13, 101), newGroup(1239, 1241)}

		cases := []testCase{
			{
				grps:     append(groups{}, grps...),
				n:        50,
				expected: groups{newGroup(1239, 1241), newGroup(55, 101)},
			},
		}

		for _, c := range cases {
			result := c.grps.pop(c.n)
			assert.Equal(t, c.expected, result)
		}
	})
}
