// Package layout assigns positions to a diagram the way reference
// architectures are drawn: actors on the left, clouds to the right, elements
// ordered left to right along the arrows, Availability Zones as side-by-side
// columns with their subnets stacked, containers grown to fit their content.
package layout

import (
	"math"
	"sort"
	"strings"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

const (
	leafW, leafH = 120, 100
	gapX, gapY   = 44, 36
	padX, padTop = 30, 56
	padBottom    = 24
	minW, minH   = 240, 150
)

// Direction of the flow: left to right (default) or top to bottom.
type Direction string

const (
	LeftToRight Direction = "LR"
	TopToBottom Direction = "TB"
)

// Auto lays out every node in d in place, left to right.
func Auto(c *catalog.Catalog, d *document.Document) { AutoWith(c, d, LeftToRight) }

// AutoWith lays out every node in d in place in the given direction.
func AutoWith(c *catalog.Catalog, d *document.Document, dir Direction) {
	l := &layouter{c: c, d: d, byID: d.Index(), children: map[string][]*document.Node{}, tb: dir == TopToBottom}
	var roots []*document.Node
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if e, ok := c.Get(n.Type); ok && e.Attachment {
			continue // configured inside their owner, not drawn
		}
		if n.Parent != "" {
			if _, ok := l.byID[n.Parent]; ok {
				l.children[n.Parent] = append(l.children[n.Parent], n)
				continue
			}
		}
		roots = append(roots, n)
	}
	l.place(roots, "")
}

type layouter struct {
	c        *catalog.Catalog
	d        *document.Document
	byID     map[string]*document.Node
	children map[string][]*document.Node
	tb       bool // ranks as rows (top to bottom) instead of columns
}

func (l *layouter) entry(n *document.Node) *catalog.Entry {
	e, _ := l.c.Get(n.Type)
	return e
}

func (l *layouter) isContainer(n *document.Node) bool {
	e := l.entry(n)
	return e != nil && e.Kind == catalog.KindContainer
}

func (l *layouter) isZone(n *document.Node) bool {
	e := l.entry(n)
	return e != nil && len(e.Provides) > 0 && e.Transparent
}

func (l *layouter) isActor(n *document.Node) bool {
	return strings.HasPrefix(n.Type, "common.") && !l.isContainer(n)
}

// size lays out n's children (recursively) and returns n's size.
func (l *layouter) size(n *document.Node) (float64, float64) {
	if !l.isContainer(n) {
		n.Layout.W, n.Layout.H = 0, 0
		return leafW, leafH
	}
	kids := l.children[n.ID]
	if len(kids) == 0 {
		n.Layout.W, n.Layout.H = minW, minH
		return minW, minH
	}
	w, h := l.place(kids, n.ID)
	n.Layout.W = math.Max(minW, w+2*padX)
	n.Layout.H = math.Max(minH, h+padTop+padBottom)
	return n.Layout.W, n.Layout.H
}

// ancestorUnder maps a node to the child of parent that contains it.
func (l *layouter) ancestorUnder(id, parent string) string {
	cur := id
	for i := 0; i < 64; i++ {
		n, ok := l.byID[cur]
		if !ok {
			return ""
		}
		if n.Parent == parent {
			return cur
		}
		if n.Parent == "" {
			return ""
		}
		cur = n.Parent
	}
	return ""
}

// place positions kids inside parent (or on the canvas when parent is "")
// and returns the bounding box of the arrangement.
func (l *layouter) place(kids []*document.Node, parent string) (float64, float64) {
	// Sizes first (recursion), so columns know how tall/wide things are.
	sz := map[string][2]float64{}
	for _, k := range kids {
		w, h := l.size(k)
		sz[k.ID] = [2]float64{w, h}
	}
	// Inside a zone: subnets stacked top to bottom, public ones first, the
	// way every reference three-tier diagram reads.
	if pn, ok := l.byID[parent]; ok && l.isZone(pn) {
		sort.SliceStable(kids, func(i, j int) bool {
			pi, _ := kids[i].Props["public"].(bool)
			pj, _ := kids[j].Props["public"].(bool)
			if pi != pj {
				return pi
			}
			return kids[i].Name < kids[j].Name
		})
		y := float64(padTop)
		w := 0.0
		for _, k := range kids {
			s := sz[k.ID]
			k.Layout.X, k.Layout.Y = padX, y
			y += s[1] + gapY
			w = math.Max(w, s[0])
		}
		return w, y - gapY - padTop
	}
	idx := map[string]bool{}
	for _, k := range kids {
		idx[k.ID] = true
	}
	// Arrows between kids (through their descendants) define the reading order.
	out := map[string]map[string]bool{}
	for _, e := range l.d.Edges {
		a, b := l.ancestorUnder(e.Source, parent), l.ancestorUnder(e.Target, parent)
		if a == "" || b == "" || a == b || !idx[a] || !idx[b] {
			continue
		}
		if out[a] == nil {
			out[a] = map[string]bool{}
		}
		out[a][b] = true
	}
	// Zones sit side by side in one row; everything else flows by rank.
	var zones, others []*document.Node
	for _, k := range kids {
		if l.isZone(k) {
			zones = append(zones, k)
		} else {
			others = append(others, k)
		}
	}
	sort.SliceStable(others, func(i, j int) bool {
		ai, aj := l.isActor(others[i]), l.isActor(others[j])
		if ai != aj {
			return ai // actors and data centers first
		}
		di := strings.HasPrefix(others[i].Type, "common.datacenter")
		dj := strings.HasPrefix(others[j].Type, "common.datacenter")
		if di != dj {
			return di
		}
		return others[i].Name < others[j].Name
	})
	sort.SliceStable(zones, func(i, j int) bool { return zoneKey(zones[i]) < zoneKey(zones[j]) })

	rank := map[string]int{}
	visiting := map[string]bool{}
	var rankOf func(id string) int
	rankOf = func(id string) int {
		if r, ok := rank[id]; ok {
			return r
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		r := 0
		for _, k := range kids {
			if out[k.ID][id] {
				if rr := rankOf(k.ID) + 1; rr > r {
					r = rr
				}
			}
		}
		delete(visiting, id)
		rank[id] = r
		return r
	}
	for _, k := range others {
		rankOf(k.ID)
	}
	// The zone row is one block in the flow: elements feeding the zones sit
	// left of it (load balancers, gateways), elements fed by them to the right.
	zoneRank := 0
	if len(zones) > 0 {
		zoneSet := map[string]bool{}
		for _, z := range zones {
			zoneSet[z.ID] = true
		}
		for _, k := range others {
			for _, z := range zones {
				if out[k.ID][z.ID] && rank[k.ID]+1 > zoneRank {
					zoneRank = rank[k.ID] + 1
				}
			}
		}
		// push everything the zones feed to the right of the block
		var bump func(id string, r int)
		seen := map[string]bool{}
		bump = func(id string, r int) {
			if seen[id] || rank[id] >= r {
				return
			}
			seen[id] = true
			rank[id] = r
			for t := range out[id] {
				if !zoneSet[t] {
					bump(t, r+1)
				}
			}
		}
		for _, z := range zones {
			for t := range out[z.ID] {
				if !zoneSet[t] {
					bump(t, zoneRank+1)
				}
			}
		}
	}
	// Elements with no arrows at all go after the flow, so a flow stays readable.
	connected := func(id string) bool {
		if len(out[id]) > 0 {
			return true
		}
		for _, m := range out {
			if m[id] {
				return true
			}
		}
		return false
	}

	x, y := float64(padX), float64(padTop)
	maxW, maxH := 0.0, 0.0
	// Ranked flow: one column per rank, stacked within the column; the zone
	// row takes the column of its rank as a single block.
	cols := map[int][]*document.Node{}
	var loose []*document.Node
	maxRank := zoneRank
	for _, k := range others {
		if !connected(k.ID) && !l.isContainer(k) && !l.isActor(k) {
			loose = append(loose, k)
			continue
		}
		r := rank[k.ID]
		cols[r] = append(cols[r], k)
		if r > maxRank {
			maxRank = r
		}
	}
	flowTop := y
	if l.tb {
		// ranks as rows: each rank is a row of items placed left to right
		for r := 0; r <= maxRank; r++ {
			rowH := 0.0
			cx := float64(padX)
			if len(zones) > 0 && r == zoneRank {
				zh := 0.0
				for _, z := range zones {
					s := sz[z.ID]
					z.Layout.X, z.Layout.Y = cx, y
					cx += s[0] + gapX
					zh = math.Max(zh, s[1])
				}
				for _, z := range zones {
					z.Layout.H = zh
				}
				rowH = zh
			}
			for _, k := range cols[r] {
				s := sz[k.ID]
				k.Layout.X, k.Layout.Y = cx, y
				cx += s[0] + gapX
				rowH = math.Max(rowH, s[1])
			}
			if rowH == 0 {
				continue
			}
			maxW = math.Max(maxW, cx-gapX)
			y += rowH + gapY
		}
		x = padX
	} else {
		colBottom := flowTop
		for r := 0; r <= maxRank; r++ {
			colW := 0.0
			cy := flowTop
			if len(zones) > 0 && r == zoneRank {
				zx := x
				rowH := 0.0
				for _, z := range zones {
					s := sz[z.ID]
					z.Layout.X, z.Layout.Y = zx, cy
					zx += s[0] + gapX
					rowH = math.Max(rowH, s[1])
				}
				for _, z := range zones {
					z.Layout.H = rowH // equal columns, like the reference diagrams
				}
				colW = zx - gapX - x
				cy += rowH + gapY
			}
			for _, k := range cols[r] {
				s := sz[k.ID]
				k.Layout.X, k.Layout.Y = x, cy
				cy += s[1] + gapY
				colW = math.Max(colW, s[0])
			}
			if colW == 0 {
				continue
			}
			colBottom = math.Max(colBottom, cy-gapY)
			x += colW + gapX
		}
		if x > padX {
			maxW = math.Max(maxW, x-gapX)
			y = colBottom + gapY
		}
	}
	// 3. unconnected leaves: a compact grid below
	if len(loose) > 0 {
		perRow := int(math.Max(3, math.Ceil(math.Sqrt(float64(len(loose))))))
		x = padX
		rowH := 0.0
		for i, k := range loose {
			if i > 0 && i%perRow == 0 {
				x = padX
				y += rowH + gapY
				rowH = 0
			}
			s := sz[k.ID]
			k.Layout.X, k.Layout.Y = x, y
			x += s[0] + gapX
			rowH = math.Max(rowH, s[1])
			maxW = math.Max(maxW, x-gapX)
		}
		y += rowH
	} else if y > flowTop {
		y -= gapY
	}
	maxH = y
	return maxW - padX, maxH - padTop
}

// zoneKey orders zones a, b, c (or 1, 2, 3) by their zone property.
func zoneKey(n *document.Node) string {
	for _, k := range []string{"zone", "az"} {
		if v, ok := n.Props[k].(string); ok {
			return v
		}
	}
	return n.Name
}
