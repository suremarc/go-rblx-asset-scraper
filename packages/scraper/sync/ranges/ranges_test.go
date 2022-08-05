package ranges

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRangeUnsafe(start, end int64) Range {
	return Range{startInclusive: start, endExclusive: end + 1}
}

func TestRanges(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		var r Ranges
		require.NoError(t, r.UnmarshalText([]byte("1-10,11-20")))
		assert.Equal(t, Ranges{newRangeUnsafe(1, 10), newRangeUnsafe(11, 20)}, r)
	})

	t.Run("pop", func(t *testing.T) {
		type testCase struct {
			ranges   Ranges
			n        int
			expected Ranges
		}

		rngs := Ranges{newRangeUnsafe(1, 10), newRangeUnsafe(13, 101), newRangeUnsafe(1239, 1241)}

		cases := []testCase{
			{
				ranges:   append(Ranges{}, rngs...),
				n:        50,
				expected: Ranges{newRangeUnsafe(1239, 1241), newRangeUnsafe(55, 101)},
			},
		}

		for _, c := range cases {
			result := c.ranges.Pop(c.n)
			assert.Equal(t, c.expected, result)
		}
	})
}
