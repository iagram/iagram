// Package layout assigns positions to an imported diagram: leaves tile in a
// grid inside their container, containers grow to fit, siblings flow left to
// right and wrap. Good enough to open and tidy by hand.
package layout

import (
	"math"
	"sort"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

const (
	leafW, leafH = 120, 90
	gap          = 30
	padX, padTop = 30, 50
	minW, minH   = 240, 150
)

// Auto lays out every node in d in place.
func Auto(c *catalog.Catalog, d *document.Document) {
	children := map[string][]*document.Node{}
	byID := d.Index()
	var roots []*document.Node
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Parent != "" {
			if _, ok := byID[n.Parent]; ok {
				children[n.Parent] = append(children[n.Parent], n)
				continue
			}
		}
		roots = append(roots, n)
	}
	var size func(n *document.Node) (w, h float64)
	size = func(n *document.Node) (float64, float64) {
		e, _ := c.Get(n.Type)
		kids := children[n.ID]
		if e == nil || e.Kind != catalog.KindContainer {
			n.Layout.W, n.Layout.H = 0, 0
			return leafW, leafH
		}
		sort.Slice(kids, func(i, j int) bool {
			ei, _ := c.Get(kids[i].Type)
			ej, _ := c.Get(kids[j].Type)
			ci := ei != nil && ei.Kind == catalog.KindContainer
			cj := ej != nil && ej.Kind == catalog.KindContainer
			if ci != cj {
				return ci // containers first
			}
			return kids[i].Name < kids[j].Name
		})
		w, h := flow(kids, size)
		n.Layout.W = math.Max(minW, w+2*padX)
		n.Layout.H = math.Max(minH, h+padTop+gap)
		return n.Layout.W, n.Layout.H
	}
	flow(roots, size)
}

// flow places nodes in rows of at most 4, returns the bounding box.
func flow(nodes []*document.Node, size func(*document.Node) (float64, float64)) (float64, float64) {
	x, y := float64(padX), float64(padTop)
	rowH, maxW := 0.0, 0.0
	perRow := 4
	if len(nodes) > 12 {
		perRow = int(math.Ceil(math.Sqrt(float64(len(nodes)))))
	}
	for i, n := range nodes {
		w, h := size(n)
		if i > 0 && i%perRow == 0 {
			x = padX
			y += rowH + gap
			rowH = 0
		}
		n.Layout.X, n.Layout.Y = x, y
		x += w + gap
		rowH = math.Max(rowH, h)
		maxW = math.Max(maxW, x-gap)
	}
	return maxW - padX, y + rowH - padTop
}
