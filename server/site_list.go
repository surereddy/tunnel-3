package server

import (
	"strings"
)

const (
	LIST_DIRECT          = iota + 1 // sites in list doesn't using tunnel
	LIST_DIRECT_SUFFIXES            // site has these suffixes doesn't using tunnel
	LIST_TUNNEL                     // sites in list using tunnel
)

type node struct {
	children map[byte]*node
}

type SiteList struct {
	mode int
	root node
}

func NewList(mode int, sites ...string) *SiteList {
	if mode != LIST_DIRECT && mode != LIST_TUNNEL && mode != LIST_DIRECT_SUFFIXES {
		panic("invalid list mode")
	}

	list := &SiteList{
		mode: mode,
	}
	list.Add(sites...)
	return list
}

func (l *SiteList) excludeSubDomain(site string) string {
	i := strings.LastIndexByte(site, '.')
	if i < 0 {
		return site
	}
	i2 := strings.LastIndexByte(site[:i], '.')
	if i2 < 0 {
		return site
	}
	return site[i2+1:]
}

func (l *SiteList) Contains(site string) bool {
	if l.mode == LIST_DIRECT_SUFFIXES {
		return l.containsSuffix(site)
	}

	site = l.excludeSubDomain(site)
	return l.contains(site)
}

func (l *SiteList) containsSuffix(site string) bool {
	curr := &l.root
	for i := len(site) - 1; i >= 0; i-- {
		b := site[i]

		if curr.children == nil {
			return true
		}

		curr = curr.children[b]
		if curr == nil {
			return false
		}
	}
	return curr.children == nil
}

func (l *SiteList) contains(site string) bool {
	curr := &l.root
	for i := range site {
		b := site[i]

		if curr.children == nil {
			return false
		}
		curr = curr.children[b]
		if curr == nil {
			return false
		}
	}
	return curr.children == nil
}

func (l *SiteList) Add(sites ...string) {
	if l.mode == LIST_DIRECT_SUFFIXES {
		for _, site := range sites {
			l.addSuffix(site)
		}
	}
	for _, site := range sites {
		site = l.excludeSubDomain(site)
		l.add(site)
	}
}

func (l *SiteList) addSuffix(suffix string) {
	curr := &l.root
	for i := len(suffix) - 1; i >= 0; i-- {
		curr = l.addNode(curr, suffix[i])
	}
}

func (l *SiteList) add(site string) {
	curr := &l.root
	for i := range site {
		curr = l.addNode(curr, site[i])
	}
}

func (l *SiteList) addNode(curr *node, b byte) *node {
	if curr.children == nil {
		curr.children = make(map[byte]*node)
	}
	child, has := curr.children[b]
	if !has {
		child = &node{}
		curr.children[b] = child
	}
	return child
}
