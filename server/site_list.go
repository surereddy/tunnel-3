package server

import (
	"strings"
	"sync"
)

const (
	LIST_WHITE = iota + 1 // only sites in list doesn't using tunnel
	LIST_BLACK            // only sites in list using tunnel
)

type node struct {
	children map[byte]*node
}

type SiteList struct {
	mode int
	root node

	mu sync.RWMutex
}

func NewList(mode int) *SiteList {
	if mode != LIST_WHITE && mode != LIST_BLACK {
		panic("invalid list mode")
	}

	return &SiteList{
		mode: mode,
	}
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
	site = l.excludeSubDomain(site)
	l.mu.RLock()
	has := l.contains(site)
	l.mu.RUnlock()
	return has
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
	l.mu.Lock()
	for _, site := range sites {
		site = l.excludeSubDomain(site)
		l.add(site)
	}
	l.mu.Unlock()
}

func (l *SiteList) add(site string) {
	curr := &l.root
	for i := range site {
		b := site[i]

		if curr.children == nil {
			curr.children = make(map[byte]*node)
		}
		child := curr.children[b]
		if child == nil {
			child = &node{}
			curr.children[b] = child
		}
		curr = child
	}
}

func (l *SiteList) IsWhiteMode() bool {
	return l.mode == LIST_WHITE
}
