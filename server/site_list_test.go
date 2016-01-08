package server

import (
	"testing"

	"github.com/cosiner/gohper/testing2"
)

func TestSiteList(t *testing.T) {
	list := NewList(LIST_TUNNEL)

	sites := []string{
		"www.google.com",
		"www.github.com",
		"www.reddit.com",
	}
	sites2 := []string{
		"a.google.com",
		"b.github.com",
		"c.reddit.com",
	}
	sites3 := []string{
		"aoogle.com",
		"bithub.com",
		"ceddit.com",
	}

	list.Add(sites...)

	for _, site := range sites {
		testing2.True(t, list.Contains(site))
	}
	for _, site := range sites2 {
		testing2.True(t, list.Contains(site))
	}
	for _, site := range sites3 {
		testing2.False(t, list.Contains(site))
	}
}

func TestSuffixList(t *testing.T) {
	list := NewList(LIST_DIRECT_SUFFIXES)
	list.Add(".cn")

	sites2 := []string{
		"a.cn",
		".cn",
	}
	for _, site := range sites2 {
		testing2.True(t, list.Contains(site))
	}
}
