package groups

import (
	"encoding"
	"fmt"
	"strconv"
	"strings"
)

type Group struct {
	Start, End int64
}

var _ encoding.TextUnmarshaler = &Group{}
var _ encoding.TextMarshaler = &Group{}

func NewGroup(start, end int64) Group {
	return Group{Start: start, End: end}
}

func (g *Group) UnmarshalText(text []byte) error {
	s := string(text)

	results := strings.Split(s, "-")
	num1, err := strconv.ParseInt(results[0], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing input arg '%s': %w", results[0], err)
	}
	num2 := num1

	if len(results) == 2 {
		num2, err = strconv.ParseInt(results[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing input arg '%s': %w", results[1], err)
		}
	}

	if num2 < num1 {
		return fmt.Errorf("invalid Group: %s", s)
	}

	*g = Group{
		Start: num1,
		End:   num2,
	}

	return nil
}

func (g Group) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%d-%d", g.Start, g.End)), nil
}

func (g *Group) Pop(n int) Group {
	if g.Len() < int64(n) {
		// consume the entire Group
		gCopy := *g
		*g = Group{End: -1}
		return gCopy
	}

	oldEnd := g.End
	g.End -= int64(n)
	return Group{
		Start: g.End + 1,
		End:   oldEnd,
	}
}

func (g *Group) Len() int64 {
	return g.End - g.Start + 1
}

func (g Group) AsIntSlice() []int64 {
	var entries []int64
	for i := g.Start; i <= g.End; i++ {
		entries = append(entries, i)
	}

	return entries
}

type Groups []Group

var _ encoding.TextUnmarshaler = &Groups{}
var _ encoding.TextMarshaler = &Groups{}

func (g *Groups) UnmarshalText(text []byte) error {
	s := string(text)

	list := strings.Split(s, ",")
	*g = make(Groups, len(list))

	for i := range list {
		if err := (*g)[i].UnmarshalText([]byte(list[i])); err != nil {
			return err
		}
	}

	return nil
}

func (g Groups) MarshalText() ([]byte, error) {
	results := make([]string, 0, len(g))
	for _, grp := range g {
		buf, _ := grp.MarshalText() // the error will always be nil in our implementation
		results = append(results, string(buf))
	}

	return []byte(strings.Join(results, ",")), nil
}

func (g *Groups) Pop(n int) Groups {
	var newG Groups
	var numEntries int64
	for numEntries < int64(n) && len(*g) > 0 {
		last := &(*g)[len(*g)-1]
		newG = append(newG, last.Pop(n-int(numEntries)))
		numEntries += newG[len(newG)-1].Len()
		if last.Len() == 0 {
			*g = (*g)[:len(*g)-1]
		}
	}

	return newG
}

func (g Groups) AsIntSlice() []int64 {
	var entries []int64
	for i := range g {
		entries = append(entries, g[i].AsIntSlice()...)
	}

	return entries
}
