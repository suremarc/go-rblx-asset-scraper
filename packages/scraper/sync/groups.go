package main

import (
	"fmt"
	"strconv"
	"strings"
)

type group struct {
	start, end int64
}

func newGroup(start, end int64) group {
	return group{start: start, end: end}
}

func (g *group) parse(s string) error {
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
		return fmt.Errorf("invalid group: %s", s)
	}

	*g = group{
		start: num1,
		end:   num2,
	}

	return nil
}

func (g *group) pop(n int) group {
	if g.len() < int64(n) {
		// consume the entire group
		gCopy := *g
		*g = group{end: -1}
		return gCopy
	}

	oldEnd := g.end
	g.end -= int64(n)
	return group{
		start: g.end + 1,
		end:   oldEnd,
	}
}

func (g *group) len() int64 {
	return g.end - g.start + 1
}

func (g group) asIntSlice() []int64 {
	var entries []int64
	for i := g.start; i <= g.end; i++ {
		entries = append(entries, i)
	}

	return entries
}

type groups []group

func (g *groups) parse(s string) error {
	list := strings.Split(s, ",")
	*g = make(groups, len(list))

	for i := range list {
		if err := (*g)[i].parse(list[i]); err != nil {
			return err
		}
	}

	return nil
}

func (g *groups) pop(n int) groups {
	var newG groups
	var numEntries int64
	for numEntries < int64(n) && len(*g) > 0 {
		last := &(*g)[len(*g)-1]
		newG = append(newG, last.pop(n-int(numEntries)))
		numEntries += newG[len(newG)-1].len()
		if last.len() == 0 {
			*g = (*g)[:len(*g)-1]
		}
	}

	return newG
}

func (g groups) asIntSlice() []int64 {
	var entries []int64
	for i := range g {
		entries = append(entries, g[i].asIntSlice()...)
	}

	return entries
}
