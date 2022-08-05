package ranges

import (
	"encoding"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Range is an non-negative integer range whose text representation is an inclusive range, e.g. 1-10.
// All Range objects have startInclusive < endExclusive by construction, except for the empty Range.
type Range struct {
	startInclusive, endExclusive int64
}

var _ encoding.TextUnmarshaler = &Range{}
var _ encoding.TextMarshaler = &Range{}

var ErrInvalidRange = errors.New("invalid range")

func NewRange(start, end int64) (Range, error) {
	if end < start && (start != 0 || end != -1) || end < 0 || start < 0 {
		return Range{}, ErrInvalidRange
	}

	return Range{startInclusive: start, endExclusive: end + 1}, nil
}

func (r Range) Start() int64 {
	return r.startInclusive
}

func (r Range) End() int64 {
	return r.endExclusive
}

func (r *Range) UnmarshalText(text []byte) error {
	s := string(text)

	if s == "" {
		*r = Range{}
		return nil
	}

	results := strings.Split(s, "-")
	if len(results) == 0 || len(results) > 2 {
		return fmt.Errorf("%w: %s", ErrInvalidRange, s)
	}

	num1, err := strconv.ParseInt(results[0], 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidRange, s)
	}
	num2 := num1

	if len(results) == 2 {
		num2, err = strconv.ParseInt(results[1], 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidRange, s)
		}
	}

	if num2 < num1 {
		return fmt.Errorf("%w: %s", ErrInvalidRange, s)
	}

	*r = Range{
		startInclusive: num1,
		endExclusive:   num2 + 1,
	}

	return nil
}

func (r Range) MarshalText() ([]byte, error) {
	if r.Len() == 0 {
		return []byte(""), nil
	}

	return []byte(fmt.Sprintf("%d-%d", r.startInclusive, r.endExclusive)), nil
}

func (r *Range) Pop(n int) Range {
	if r.Len() < int64(n) {
		// consume the entire Range
		rCopy := *r
		*r = Range{}
		return rCopy
	}

	oldEnd := r.endExclusive
	r.endExclusive -= int64(n)
	return Range{
		startInclusive: r.endExclusive,
		endExclusive:   oldEnd,
	}
}

func (r *Range) Len() int64 {
	return r.endExclusive - r.startInclusive
}

func (r Range) AsIntSlice() []int64 {
	var entries []int64
	for i := r.startInclusive; i < r.endExclusive; i++ {
		entries = append(entries, i)
	}

	return entries
}

type Ranges []Range

var _ encoding.TextUnmarshaler = &Ranges{}
var _ encoding.TextMarshaler = &Ranges{}

func (r *Ranges) UnmarshalText(text []byte) error {
	s := string(text)

	list := strings.Split(s, ",")
	*r = make(Ranges, len(list))

	for i := range list {
		if err := (*r)[i].UnmarshalText([]byte(list[i])); err != nil {
			return err
		}
	}

	return nil
}

func (r Ranges) MarshalText() ([]byte, error) {
	results := make([]string, 0, len(r))
	for _, rng := range r {
		buf, _ := rng.MarshalText() // the error will always be nil in our implementation
		results = append(results, string(buf))
	}

	return []byte(strings.Join(results, ",")), nil
}

func (r *Ranges) Pop(n int) Ranges {
	var newRange Ranges
	var numEntries int64
	for numEntries < int64(n) && len(*r) > 0 {
		last := &(*r)[len(*r)-1]
		newRange = append(newRange, last.Pop(n-int(numEntries)))
		numEntries += newRange[len(newRange)-1].Len()
		if last.Len() == 0 {
			*r = (*r)[:len(*r)-1]
		}
	}

	return newRange
}

func (r Ranges) AsIntSlice() []int64 {
	var entries []int64
	for i := range r {
		entries = append(entries, r[i].AsIntSlice()...)
	}

	return entries
}
