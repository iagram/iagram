// Package layout arranges a diagram automatically. Every algorithm works per
// container, bottom-up: the children of a container are arranged as a graph
// (arrows between descendants count as arrows between the children that hold
// them), the container grows to fit, then the parent is arranged in turn.
//
// Algorithms: flow (layered / hierarchical, the reference-architecture look,
// with Availability Zones as a row of columns), tree, radial tree, organic
// (force-directed), org chart, circle and grid.
package layout

import (
	"math"
	"sort"
	"strings"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

// Algo names an arrangement algorithm.
type Algo string

const (
	Flow     Algo = "flow"
	Tree     Algo = "tree"
	Radial   Algo = "radial"
	Organic  Algo = "organic"
	OrgChart Algo = "orgchart"
	Circle   Algo = "circle"
	Grid     Algo = "grid"
)

// Direction of a flow or tree: left to right or top to bottom.
type Direction string

const (
	LeftToRight Direction = "LR"
	TopToBottom Direction = "TB"
)

// Options tune an arrangement.
type Options struct {
	Algo        Algo      `json:"algo"`
	Dir         Direction `json:"dir"`
	NodeSpacing float64   `json:"node_spacing"` // between siblings in a rank / branch
	RankSpacing float64   `json:"rank_spacing"` // between ranks / tree levels
	// ResizeContainers grows containers to fit; off keeps their current size.
	ResizeContainers bool `json:"resize_containers"`
	// PreserveOrigin keeps the top-left corner of the arranged set where it was.
	PreserveOrigin bool `json:"preserve_origin"`
}

// Defaults is the reference-architecture arrangement.
func Defaults() Options {
	return Options{Algo: Flow, Dir: LeftToRight, NodeSpacing: 36, RankSpacing: 44, ResizeContainers: true, PreserveOrigin: true}
}

const (
	leafW, leafH = 120, 100
	padX, padTop = 30, 56
	padBottom    = 24
	minW, minH   = 240, 150
	// a component box with nothing inside (an empty cluster) is drawn as a
	// header-only card, the way the reference diagrams show a bare service
	compactW, compactH = 220, 64
)

// Auto lays out every node in d in place, reference style, left to right.
func Auto(c *catalog.Catalog, d *document.Document) { Arrange(c, d, Defaults()) }

// AutoWith lays out every node in d in place, reference style, in dir.
func AutoWith(c *catalog.Catalog, d *document.Document, dir Direction) {
	o := Defaults()
	o.Dir = dir
	Arrange(c, d, o)
}

// Arrange lays out every node in d in place with the given options.
func Arrange(c *catalog.Catalog, d *document.Document, o Options) {
	if o.NodeSpacing <= 0 {
		o.NodeSpacing = 36
	}
	if o.RankSpacing <= 0 {
		o.RankSpacing = 44
	}
	if o.Algo == "" {
		o.Algo = Flow
	}
	if o.Dir == "" {
		o.Dir = LeftToRight
	}
	l := &layouter{c: c, d: d, o: o, byID: d.Index(), children: map[string][]*document.Node{}}
	var roots []*document.Node
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if e, ok := c.Get(n.Type); ok && e.Attachment {
			continue // configured inside their owner, not drawn
		}
		if len(l.coveredBy(n)) > 0 {
			l.bands = append(l.bands, n) // drawn over the containers it covers, after they are placed
			continue
		}
		if n.Parent != "" {
			if _, ok := l.byID[n.Parent]; ok {
				l.children[n.Parent] = append(l.children[n.Parent], n)
				continue
			}
		}
		roots = append(roots, n)
	}
	ox, oy := math.Inf(1), math.Inf(1)
	for _, r := range roots {
		ox, oy = math.Min(ox, r.Layout.X), math.Min(oy, r.Layout.Y)
	}
	l.place(roots, "")
	l.placeBands()
	if o.PreserveOrigin && len(roots) > 0 && !math.IsInf(ox, 0) {
		nx, ny := math.Inf(1), math.Inf(1)
		for _, r := range roots {
			nx, ny = math.Min(nx, r.Layout.X), math.Min(ny, r.Layout.Y)
		}
		for _, r := range roots {
			r.Layout.X += ox - nx
			r.Layout.Y += oy - ny
		}
	}
}

type layouter struct {
	c        *catalog.Catalog
	d        *document.Document
	o        Options
	byID     map[string]*document.Node
	children map[string][]*document.Node
	bands    []*document.Node // spanning groups (Auto Scaling groups) laid over their containers
}

// coveredBy returns the containers a spanning band stretches over: the
// targets of its reference edges, or the containers holding what it points
// at (an Auto Scaling group "scales" instances that live in subnets).
func (l *layouter) coveredBy(band *document.Node) []*document.Node {
	e := l.entry(band)
	if e == nil || e.Span == nil {
		return nil
	}
	allowed := map[string]bool{}
	for _, t := range e.Span.Elements {
		allowed[t] = true
	}
	seen := map[string]bool{}
	var out []*document.Node
	add := func(n *document.Node) {
		if n != nil && allowed[n.Type] && !seen[n.ID] {
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	for _, ed := range l.d.Edges {
		if ed.Source != band.ID {
			continue
		}
		t := l.byID[ed.Target]
		if t == nil {
			continue
		}
		if ed.Kind == "references" {
			add(t)
			continue
		}
		for cur := t; cur != nil; cur = l.byID[cur.Parent] {
			if allowed[cur.Type] {
				add(cur)
				break
			}
		}
	}
	return out
}

// absPos returns a node's position on the canvas (parents' offsets summed).
func (l *layouter) absPos(n *document.Node) (float64, float64) {
	x, y := 0.0, 0.0
	for cur := n; cur != nil; cur = l.byID[cur.Parent] {
		x += cur.Layout.X
		y += cur.Layout.Y
	}
	return x, y
}

// Band insets: inside the covered containers' borders, below their headers,
// around the elements they hold, like the dashed Auto Scaling box in the
// AWS diagrams.
const (
	bandInsetX   = 22
	bandInsetTop = 44
	bandInsetBot = 18
)

// placeBands stretches every spanning band over the containers it covers.
func (l *layouter) placeBands() {
	for _, b := range l.bands {
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, cn := range l.coveredBy(b) {
			x, y := l.absPos(cn)
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x+cn.Layout.W), math.Max(maxY, y+cn.Layout.H)
		}
		if math.IsInf(minX, 0) {
			continue
		}
		px, py := 0.0, 0.0
		if p := l.byID[b.Parent]; p != nil {
			px, py = l.absPos(p)
		}
		b.Layout.X = minX - px + bandInsetX
		b.Layout.Y = minY - py + bandInsetTop
		b.Layout.W = math.Max(compactW, maxX-minX-2*bandInsetX)
		b.Layout.H = math.Max(compactH, maxY-minY-bandInsetTop-bandInsetBot)
	}
}

func (l *layouter) entry(n *document.Node) *catalog.Entry {
	e, _ := l.c.Get(n.Type)
	return e
}

func (l *layouter) isContainer(n *document.Node) bool {
	e := l.entry(n)
	return e != nil && e.Kind == catalog.KindContainer && !l.collapsed(n)
}

// collapsed reports a composite service (a cluster) with nothing inside: drawn
// and placed as a plain icon.
func (l *layouter) collapsed(n *document.Node) bool {
	e := l.entry(n)
	if e == nil || e.Kind != catalog.KindContainer {
		return false
	}
	switch n.View {
	case "icon":
		return true
	case "box":
		return false
	}
	return e.Composite && len(l.children[n.ID]) == 0
}

func (l *layouter) isZone(n *document.Node) bool {
	e := l.entry(n)
	return e != nil && len(e.Provides) > 0 && e.Transparent
}

func (l *layouter) isActor(n *document.Node) bool {
	return strings.HasPrefix(n.Type, "common.") && !l.isContainer(n)
}

// size arranges n's children (recursively) and returns n's size.
func (l *layouter) size(n *document.Node) (float64, float64) {
	if !l.isContainer(n) || l.collapsed(n) {
		n.Layout.W, n.Layout.H = 0, 0
		return leafW, leafH
	}
	kids := l.children[n.ID]
	if len(kids) == 0 {
		if !l.o.ResizeContainers && n.Layout.W > 0 {
			return n.Layout.W, n.Layout.H
		}
		if !strings.HasPrefix(n.Type, "common.") {
			// an empty box (a cluster without nodes, a region or subnet with
			// nothing drawn in it) is a header-only card
			n.Layout.W, n.Layout.H = compactW, compactH
			return compactW, compactH
		}
		n.Layout.W, n.Layout.H = minW, minH
		return minW, minH
	}
	w, h := l.place(kids, n.ID)
	if !l.o.ResizeContainers && n.Layout.W > 0 {
		return n.Layout.W, n.Layout.H
	}
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

// graph is the sibling graph of one container.
type graph struct {
	kids  []*document.Node
	sz    map[string][2]float64
	out   map[string]map[string]bool // arrows between kids (through descendants)
	in    map[string]int
	guard map[string]string // "protects" lines: guard -> protected kid; adjacency, not flow
	order []*document.Node  // reading order: actors, data centers, then names
}

func (l *layouter) graphOf(kids []*document.Node, parent string) *graph {
	g := &graph{kids: kids, sz: map[string][2]float64{}, out: map[string]map[string]bool{}, in: map[string]int{}, guard: map[string]string{}}
	idx := map[string]bool{}
	for _, k := range kids {
		w, h := l.size(k)
		g.sz[k.ID] = [2]float64{w, h}
		idx[k.ID] = true
		g.in[k.ID] = 0
	}
	for _, e := range l.d.Edges {
		a, b := l.ancestorUnder(e.Source, parent), l.ancestorUnder(e.Target, parent)
		if a == "" || b == "" || a == b || !idx[a] || !idx[b] {
			continue
		}
		switch e.Kind {
		case "references":
			// hidden span bookkeeping, never drawn as an arrow
			continue
		case "protects":
			// a security group / firewall rule / NSG is drawn next to what it
			// protects, the way the reference diagrams do; it does not rank it
			if _, dup := g.guard[a]; !dup {
				g.guard[a] = b
			}
			continue
		}
		if g.out[a] == nil {
			g.out[a] = map[string]bool{}
		}
		if !g.out[a][b] {
			g.out[a][b] = true
			g.in[b]++
		}
	}
	g.order = append([]*document.Node{}, kids...)
	sort.SliceStable(g.order, func(i, j int) bool {
		ai, aj := l.isActor(g.order[i]), l.isActor(g.order[j])
		if ai != aj {
			return ai
		}
		di := strings.HasPrefix(g.order[i].Type, "common.datacenter")
		dj := strings.HasPrefix(g.order[j].Type, "common.datacenter")
		if di != dj {
			return di
		}
		return sourceBefore(g.order[i], g.order[j])
	})
	return g
}

// sourceBefore keeps the author's reading order (rows, then columns) when the
// nodes carry positions, so a mirror or a re-arrangement does not shuffle
// siblings; unplaced nodes fall back to their names.
func sourceBefore(a, b *document.Node) bool {
	placed := func(n *document.Node) bool { return n.Layout.X != 0 || n.Layout.Y != 0 }
	if placed(a) && placed(b) {
		ra, rb := math.Floor(a.Layout.Y/60), math.Floor(b.Layout.Y/60)
		if ra != rb {
			return ra < rb
		}
		if a.Layout.X != b.Layout.X {
			return a.Layout.X < b.Layout.X
		}
	} else if placed(a) != placed(b) {
		return placed(a)
	}
	return a.Name < b.Name
}

func (g *graph) connected(id string) bool {
	if len(g.out[id]) > 0 || g.in[id] > 0 {
		return true
	}
	if _, ok := g.guard[id]; ok {
		return true
	}
	for _, t := range g.guard {
		if t == id {
			return true
		}
	}
	return false
}

// pullGuards moves every guard right before the kid it protects when both
// sit in the same column.
func (g *graph) pullGuards(col []*document.Node) []*document.Node {
	if len(g.guard) == 0 {
		return col
	}
	pos := map[string]int{}
	for i, k := range col {
		pos[k.ID] = i
	}
	out := make([]*document.Node, 0, len(col))
	placed := map[string]bool{}
	for _, k := range col {
		if placed[k.ID] {
			continue
		}
		if t, ok := g.guard[k.ID]; ok {
			if _, same := pos[t]; same {
				continue // emitted right before its target
			}
		}
		for _, other := range col {
			if g.guard[other.ID] == k.ID && !placed[other.ID] {
				out = append(out, other)
				placed[other.ID] = true
			}
		}
		out = append(out, k)
		placed[k.ID] = true
	}
	return out
}

// place arranges kids inside parent ("" = canvas) and returns the bounding box.
func (l *layouter) place(kids []*document.Node, parent string) (float64, float64) {
	g := l.graphOf(kids, parent)
	if len(kids) == 0 {
		return 0, 0
	}
	// Cells read off the reference diagram win over any heuristic (flow only:
	// an explicitly chosen algorithm is the user's call).
	if l.o.Algo == Flow && hasGrid(kids) {
		return l.gridCells(g)
	}
	// Inside a zone: subnets stacked top to bottom, public ones first.
	if pn, ok := l.byID[parent]; ok && l.isZone(pn) {
		return l.stackZone(g)
	}
	var w, h float64
	switch l.o.Algo {
	case Tree:
		w, h = l.tree(g, false)
	case OrgChart:
		w, h = l.tree(g, true)
	case Radial:
		w, h = l.radial(g)
	case Organic:
		w, h = l.organic(g)
	case Circle:
		w, h = l.circle(g)
	case Grid:
		w, h = l.grid(g, g.order)
	default:
		w, h = l.flow(g)
	}
	return w, h
}

func hasGrid(kids []*document.Node) bool {
	for _, k := range kids {
		if k.Layout.Row > 0 && k.Layout.Col > 0 {
			return true
		}
	}
	return false
}

// gridCells places kids on the grid their Row/Col cells describe: columns
// are as wide as their widest cell and rows as tall as their tallest, a
// spanning box takes the cells it covers, containers stretch to fill their
// cell (uniform Availability Zone boxes), leaves sit centred in theirs.
// Kids without a cell go in an extra row under the grid, in reading order.
func (l *layouter) gridCells(g *graph) (float64, float64) {
	gapX, gapY := l.o.RankSpacing, l.o.NodeSpacing
	type cell struct {
		n            *document.Node
		r, c, rs, cs int
	}
	var cells []cell
	var loose []*document.Node
	maxR, maxC := 0, 0
	for _, k := range g.order {
		if k.Layout.Row <= 0 || k.Layout.Col <= 0 {
			loose = append(loose, k)
			continue
		}
		rs, cs := k.Layout.RowSpan, k.Layout.ColSpan
		if rs < 1 {
			rs = 1
		}
		if cs < 1 {
			cs = 1
		}
		cells = append(cells, cell{k, k.Layout.Row, k.Layout.Col, rs, cs})
		if k.Layout.Row+rs-1 > maxR {
			maxR = k.Layout.Row + rs - 1
		}
		if k.Layout.Col+cs-1 > maxC {
			maxC = k.Layout.Col + cs - 1
		}
	}
	if len(loose) > 0 {
		maxR++
		for i, k := range loose {
			cells = append(cells, cell{k, maxR, i + 1, 1, 1})
			if i+1 > maxC {
				maxC = i + 1
			}
		}
	}
	colW := make([]float64, maxC+1)
	rowH := make([]float64, maxR+1)
	// single-cell sizes first, then spanning boxes widen their last cell if needed
	for _, c := range cells {
		s := g.sz[c.n.ID]
		if c.cs == 1 {
			colW[c.c] = math.Max(colW[c.c], s[0])
		}
		if c.rs == 1 {
			rowH[c.r] = math.Max(rowH[c.r], s[1])
		}
	}
	for _, c := range cells {
		s := g.sz[c.n.ID]
		if c.cs > 1 {
			have := float64(c.cs-1) * gapX
			for i := c.c; i < c.c+c.cs; i++ {
				have += colW[i]
			}
			if s[0] > have {
				colW[c.c+c.cs-1] += s[0] - have
			}
		}
		if c.rs > 1 {
			have := float64(c.rs-1) * gapY
			for i := c.r; i < c.r+c.rs; i++ {
				have += rowH[i]
			}
			if s[1] > have {
				rowH[c.r+c.rs-1] += s[1] - have
			}
		}
	}
	x := make([]float64, maxC+2)
	x[1] = padX
	for c := 1; c <= maxC; c++ {
		x[c+1] = x[c] + colW[c] + gapX
	}
	y := make([]float64, maxR+2)
	y[1] = padTop
	for r := 1; r <= maxR; r++ {
		y[r+1] = y[r] + rowH[r] + gapY
	}
	for _, c := range cells {
		s := g.sz[c.n.ID]
		cw := x[c.c+c.cs] - x[c.c] - gapX
		ch := y[c.r+c.rs] - y[c.r] - gapY
		if l.isContainer(c.n) && l.o.ResizeContainers {
			c.n.Layout.X, c.n.Layout.Y = x[c.c], y[c.r]
			c.n.Layout.W, c.n.Layout.H = cw, ch
			g.sz[c.n.ID] = [2]float64{cw, ch}
			continue
		}
		c.n.Layout.X = x[c.c] + (cw-s[0])/2
		c.n.Layout.Y = y[c.r] + (ch-s[1])/2
	}
	return x[maxC+1] - gapX - padX, y[maxR+1] - gapY - padTop
}

func (l *layouter) stackZone(g *graph) (float64, float64) {
	kids := append([]*document.Node{}, g.kids...)
	sort.SliceStable(kids, func(i, j int) bool {
		pi, _ := kids[i].Props["public"].(bool)
		pj, _ := kids[j].Props["public"].(bool)
		if pi != pj {
			return pi
		}
		return sourceBefore(kids[i], kids[j])
	})
	y := float64(padTop)
	w := 0.0
	for _, k := range kids {
		s := g.sz[k.ID]
		k.Layout.X, k.Layout.Y = padX, y
		y += s[1] + l.o.NodeSpacing
		w = math.Max(w, s[0])
	}
	return w, y - l.o.NodeSpacing - padTop
}

// normalise shifts kids so the arrangement starts at the padding origin and
// returns its width and height.
func normalise(kids []*document.Node, sz map[string][2]float64) (float64, float64) {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, k := range kids {
		minX, minY = math.Min(minX, k.Layout.X), math.Min(minY, k.Layout.Y)
	}
	maxX, maxY := 0.0, 0.0
	for _, k := range kids {
		k.Layout.X += padX - minX
		k.Layout.Y += padTop - minY
		s := sz[k.ID]
		maxX, maxY = math.Max(maxX, k.Layout.X+s[0]), math.Max(maxY, k.Layout.Y+s[1])
	}
	return maxX - padX, maxY - padTop
}

// ---- flow (layered) -------------------------------------------------------

func (l *layouter) ranks(g *graph, nodes []*document.Node) map[string]int {
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
		for _, k := range g.kids {
			if g.out[k.ID][id] {
				if rr := rankOf(k.ID) + 1; rr > r {
					r = rr
				}
			}
		}
		delete(visiting, id)
		rank[id] = r
		return r
	}
	for _, k := range nodes {
		rankOf(k.ID)
	}
	return rank
}

func (l *layouter) flow(g *graph) (float64, float64) {
	gapX, gapY := l.o.RankSpacing, l.o.NodeSpacing
	tb := l.o.Dir == TopToBottom
	var zones, others []*document.Node
	for _, k := range g.order {
		if l.isZone(k) {
			zones = append(zones, k)
		} else {
			others = append(others, k)
		}
	}
	sort.SliceStable(zones, func(i, j int) bool {
		// AZ columns in order; inside one AZ (or without AZs) keep the author's order
		if ki, kj := zoneKey(zones[i]), zoneKey(zones[j]); ki != kj {
			return ki < kj
		}
		return sourceBefore(zones[i], zones[j])
	})
	rank := l.ranks(g, others)
	for a, b := range g.guard {
		if r, ok := rank[b]; ok {
			rank[a] = r
		}
	}
	// The zone row is one block in the flow: feeders left of it, consumers right.
	zoneRank := 0
	if len(zones) > 0 {
		zoneSet := map[string]bool{}
		for _, z := range zones {
			zoneSet[z.ID] = true
		}
		for _, k := range others {
			for _, z := range zones {
				if g.out[k.ID][z.ID] && rank[k.ID]+1 > zoneRank {
					zoneRank = rank[k.ID] + 1
				}
			}
		}
		seen := map[string]bool{}
		var bump func(id string, r int)
		bump = func(id string, r int) {
			if seen[id] || rank[id] >= r {
				return
			}
			seen[id] = true
			rank[id] = r
			for t := range g.out[id] {
				if !zoneSet[t] {
					bump(t, r+1)
				}
			}
		}
		for _, z := range zones {
			for t := range g.out[z.ID] {
				if !zoneSet[t] {
					bump(t, zoneRank+1)
				}
			}
		}
	}
	cols := map[int][]*document.Node{}
	var loose []*document.Node
	maxRank := zoneRank
	for _, k := range others {
		if !g.connected(k.ID) && !l.isContainer(k) && !l.isActor(k) {
			loose = append(loose, k)
			continue
		}
		r := rank[k.ID]
		cols[r] = append(cols[r], k)
		if r > maxRank {
			maxRank = r
		}
	}
	// Fewer crossings: order every rank by the average position of the
	// elements that point at it in the previous ranks (barycentre heuristic).
	position := map[string]float64{}
	for r := 0; r <= maxRank; r++ {
		col := cols[r]
		if r > 0 {
			bary := map[string]float64{}
			for _, k := range col {
				sum, cnt := 0.0, 0
				for _, p := range others {
					if g.out[p.ID][k.ID] && rank[p.ID] < r {
						if pos, ok := position[p.ID]; ok {
							sum += pos
							cnt++
						}
					}
				}
				if cnt > 0 {
					bary[k.ID] = sum / float64(cnt)
				} else {
					bary[k.ID] = math.MaxFloat64 // unconnected from the left: last
				}
			}
			sort.SliceStable(col, func(i, j int) bool { return bary[col[i].ID] < bary[col[j].ID] })
		}
		col = g.pullGuards(col)
		cols[r] = col
		for i, k := range col {
			position[k.ID] = float64(i)
		}
	}
	x, y := float64(padX), float64(padTop)
	maxW := 0.0
	// Straight arrows: a node sits level with the mean of the neighbours it is
	// connected to in other columns (forward pass), then feeders are pulled
	// level with what they feed (backward pass). Nodes never overlap: a node
	// is pushed down (right, top-to-bottom) when the wanted spot is taken.
	colOf := map[string]int{}
	for r, col := range cols {
		for _, k := range col {
			colOf[k.ID] = r
		}
	}
	centre := map[string]float64{} // cross-axis centre of placed kids (zones included)
	meanOf := func(k *document.Node, r int) (float64, bool) {
		sum, cnt := 0.0, 0
		for _, p := range g.kids {
			if p == k {
				continue
			}
			if c, ok := centre[p.ID]; ok && colOf[p.ID] != r && (g.out[p.ID][k.ID] || g.out[k.ID][p.ID]) {
				sum += c
				cnt++
			}
		}
		return sum / math.Max(1, float64(cnt)), cnt > 0
	}
	if tb {
		rowTop := map[int]float64{}
		placeRow := func(r int, left float64) float64 {
			cx := left
			for _, k := range cols[r] {
				s := g.sz[k.ID]
				want := cx
				if m, ok := meanOf(k, r); ok {
					want = math.Max(cx, m-s[0]/2)
				}
				k.Layout.X = want
				centre[k.ID] = want + s[0]/2
				cx = want + s[0] + gapY
			}
			return cx
		}
		for r := 0; r <= maxRank; r++ {
			rowH := 0.0
			cx := float64(padX)
			if len(zones) > 0 && r == zoneRank {
				zh := 0.0
				for _, z := range zones {
					s := g.sz[z.ID]
					z.Layout.X, z.Layout.Y = cx, y
					centre[z.ID] = cx + s[0]/2
					cx += s[0] + gapY
					zh = math.Max(zh, s[1])
				}
				for _, z := range zones {
					z.Layout.H = zh
				}
				rowH = zh
			}
			rowTop[r] = cx
			for _, k := range cols[r] {
				k.Layout.Y = y
				rowH = math.Max(rowH, g.sz[k.ID][1])
			}
			cx = placeRow(r, cx)
			if rowH == 0 {
				continue
			}
			maxW = math.Max(maxW, cx-gapY)
			y += rowH + gapX
		}
		for r := maxRank; r >= 0; r-- {
			if len(cols[r]) > 0 {
				maxW = math.Max(maxW, placeRow(r, rowTop[r])-gapY)
			}
		}
	} else {
		flowTop := y
		colBottom := flowTop
		colTop := map[int]float64{}
		colX := map[int]float64{}
		placeCol := func(r int, top float64) float64 {
			cy := top
			for _, k := range cols[r] {
				s := g.sz[k.ID]
				want := cy
				if m, ok := meanOf(k, r); ok {
					want = math.Max(cy, m-s[1]/2)
				}
				k.Layout.X, k.Layout.Y = colX[r], want
				centre[k.ID] = want + s[1]/2
				cy = want + s[1] + gapY
			}
			return cy
		}
		for r := 0; r <= maxRank; r++ {
			colW := 0.0
			cy := flowTop
			colX[r] = x
			if len(zones) > 0 && r == zoneRank {
				zx := x
				rowH := 0.0
				for _, z := range zones {
					s := g.sz[z.ID]
					z.Layout.X, z.Layout.Y = zx, cy
					zx += s[0] + gapY
					rowH = math.Max(rowH, s[1])
				}
				for _, z := range zones {
					z.Layout.H = rowH
					centre[z.ID] = cy + rowH/2
				}
				colW = zx - gapY - x
				cy += rowH + gapY
			}
			colTop[r] = cy
			for _, k := range cols[r] {
				colW = math.Max(colW, g.sz[k.ID][0])
			}
			cy = placeCol(r, cy)
			if colW == 0 {
				continue
			}
			colBottom = math.Max(colBottom, cy-gapY)
			x += colW + gapX
		}
		for r := maxRank; r >= 0; r-- {
			if len(cols[r]) > 0 {
				colBottom = math.Max(colBottom, placeCol(r, colTop[r])-gapY)
			}
		}
		if x > padX {
			maxW = math.Max(maxW, x-gapX)
			y = colBottom + gapY
		}
	}
	// unconnected leaves: a compact grid below the flow
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
			s := g.sz[k.ID]
			k.Layout.X, k.Layout.Y = x, y
			x += s[0] + gapY
			rowH = math.Max(rowH, s[1])
			maxW = math.Max(maxW, x-gapY)
		}
		y += rowH
	} else if y > padTop {
		y -= gapY
	}
	return maxW - padX, y - padTop
}

// ---- trees ----------------------------------------------------------------

// forest builds parent->children lists from the sibling graph: roots are the
// nodes nothing points at (or the first in reading order when everything is
// in a cycle); each node gets one parent, BFS order.
func (l *layouter) forest(g *graph) (roots []*document.Node, kidsOf map[string][]*document.Node) {
	kidsOf = map[string][]*document.Node{}
	byID := map[string]*document.Node{}
	for _, k := range g.kids {
		byID[k.ID] = k
	}
	placed := map[string]bool{}
	var visit func(n *document.Node)
	visit = func(n *document.Node) {
		queue := []*document.Node{n}
		placed[n.ID] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			var ts []string
			for t := range g.out[cur.ID] {
				ts = append(ts, t)
			}
			sort.Strings(ts)
			for _, t := range ts {
				if placed[t] {
					continue
				}
				placed[t] = true
				kidsOf[cur.ID] = append(kidsOf[cur.ID], byID[t])
				queue = append(queue, byID[t])
			}
		}
	}
	for _, k := range g.order {
		if g.in[k.ID] == 0 && !placed[k.ID] {
			roots = append(roots, k)
			visit(k)
		}
	}
	for _, k := range g.order {
		if !placed[k.ID] {
			roots = append(roots, k)
			visit(k)
		}
	}
	return roots, kidsOf
}

// tree is a tidy layered tree: each subtree gets the width of its children,
// parents centred over them. Org chart mode lists leaf children under their
// parent in one column instead of side by side.
func (l *layouter) tree(g *graph, orgChart bool) (float64, float64) {
	roots, kidsOf := l.forest(g)
	tb := l.o.Dir == TopToBottom || orgChart
	gapS, gapR := l.o.NodeSpacing, l.o.RankSpacing
	// breadth of a node along the sibling axis, depth along the rank axis
	breadth := func(id string) float64 {
		s := g.sz[id]
		if tb {
			return s[0]
		}
		return s[1]
	}
	depth := func(id string) float64 {
		s := g.sz[id]
		if tb {
			return s[1]
		}
		return s[0]
	}
	extent := map[string]float64{}
	var measure func(n *document.Node) float64
	measure = func(n *document.Node) float64 {
		kids := kidsOf[n.ID]
		if len(kids) == 0 {
			extent[n.ID] = breadth(n.ID)
			return extent[n.ID]
		}
		if orgChart && allLeaves(kids, kidsOf) {
			// subordinates listed in a column: breadth of the widest, indented
			w := 0.0
			for _, k := range kids {
				w = math.Max(w, breadth(k.ID))
			}
			extent[n.ID] = math.Max(breadth(n.ID), w+gapS)
			return extent[n.ID]
		}
		total := 0.0
		for i, k := range kids {
			if i > 0 {
				total += gapS
			}
			total += measure(k)
		}
		extent[n.ID] = math.Max(breadth(n.ID), total)
		return extent[n.ID]
	}
	// level depth = deepest node per level so ranks align
	levelDepth := map[int]float64{}
	var levels func(n *document.Node, lv int)
	levels = func(n *document.Node, lv int) {
		levelDepth[lv] = math.Max(levelDepth[lv], depth(n.ID))
		for _, k := range kidsOf[n.ID] {
			levels(k, lv+1)
		}
	}
	for _, r := range roots {
		measure(r)
		levels(r, 0)
	}
	levelPos := map[int]float64{}
	acc := 0.0
	for lv := 0; lv <= len(g.kids); lv++ {
		if _, ok := levelDepth[lv]; !ok {
			break
		}
		levelPos[lv] = acc
		acc += levelDepth[lv] + gapR
	}
	var setPos func(n *document.Node, along, lv float64)
	setPos = func(n *document.Node, along, lv float64) {
		kids := kidsOf[n.ID]
		ext := extent[n.ID]
		center := along + ext/2
		b := breadth(n.ID)
		pos := center - b/2
		if tb {
			n.Layout.X, n.Layout.Y = pos, levelPos[int(lv)]
		} else {
			n.Layout.X, n.Layout.Y = levelPos[int(lv)], pos
		}
		if len(kids) == 0 {
			return
		}
		if orgChart && allLeaves(kids, kidsOf) {
			y := levelPos[int(lv)] + depth(n.ID) + gapR
			for _, k := range kids {
				k.Layout.X, k.Layout.Y = along+gapS, y
				y += g.sz[k.ID][1] + gapS/2
			}
			return
		}
		total := 0.0
		for i, k := range kids {
			if i > 0 {
				total += gapS
			}
			total += extent[k.ID]
		}
		cur := center - total/2
		for _, k := range kids {
			setPos(k, cur, lv+1)
			cur += extent[k.ID] + gapS
		}
	}
	along := 0.0
	for _, r := range roots {
		setPos(r, along, 0)
		along += extent[r.ID] + gapS*2
	}
	return normalise(g.kids, g.sz)
}

func allLeaves(kids []*document.Node, kidsOf map[string][]*document.Node) bool {
	for _, k := range kids {
		if len(kidsOf[k.ID]) > 0 {
			return false
		}
	}
	return len(kids) > 1
}

// radial places the root at the centre and each tree level on a ring; the
// angular span of a subtree is proportional to its leaf count.
func (l *layouter) radial(g *graph) (float64, float64) {
	roots, kidsOf := l.forest(g)
	// ring radius per level: half the biggest box of the previous level plus
	// half the biggest of this one, plus the rank spacing
	levelMax := map[int]float64{}
	var measure func(n *document.Node, lv int)
	measure = func(n *document.Node, lv int) {
		s := g.sz[n.ID]
		levelMax[lv] = math.Max(levelMax[lv], math.Max(s[0], s[1]))
		for _, k := range kidsOf[n.ID] {
			measure(k, lv+1)
		}
	}
	for _, r := range roots {
		measure(r, 0)
	}
	radius := map[int]float64{}
	acc := 0.0
	for lv := 1; lv <= len(g.kids); lv++ {
		if _, ok := levelMax[lv]; !ok {
			break
		}
		acc += levelMax[lv-1]/2 + levelMax[lv]/2 + l.o.RankSpacing
		radius[lv] = acc
	}
	leaves := map[string]int{}
	var count func(n *document.Node) int
	count = func(n *document.Node) int {
		kids := kidsOf[n.ID]
		if len(kids) == 0 {
			leaves[n.ID] = 1
			return 1
		}
		t := 0
		for _, k := range kids {
			t += count(k)
		}
		leaves[n.ID] = t
		return t
	}
	total := 0
	for _, r := range roots {
		total += count(r)
	}
	cx, cy := 0.0, 0.0
	var placeArc func(n *document.Node, lv int, a0, a1 float64)
	placeArc = func(n *document.Node, lv int, a0, a1 float64) {
		s := g.sz[n.ID]
		if lv == 0 && len(roots) == 1 {
			n.Layout.X, n.Layout.Y = cx-s[0]/2, cy-s[1]/2
		} else {
			r := radius[lv]
			if len(roots) > 1 {
				r = radius[lv] + levelMax[0]/2 + l.o.RankSpacing
			}
			a := (a0 + a1) / 2
			n.Layout.X, n.Layout.Y = cx+r*math.Cos(a)-s[0]/2, cy+r*math.Sin(a)-s[1]/2
		}
		kids := kidsOf[n.ID]
		if len(kids) == 0 {
			return
		}
		cur := a0
		span := a1 - a0
		for _, k := range kids {
			part := span * float64(leaves[k.ID]) / float64(leaves[n.ID])
			placeArc(k, lv+1, cur, cur+part)
			cur += part
		}
	}
	cur := -math.Pi / 2
	for _, r := range roots {
		part := 2 * math.Pi * float64(leaves[r.ID]) / float64(total)
		placeArc(r, 0, cur, cur+part)
		cur += part
	}
	return normalise(g.kids, g.sz)
}

// organic is a force-directed placement (Fruchterman-Reingold) with boxes
// repelling by their size and arrows pulling like springs.
func (l *layouter) organic(g *graph) (float64, float64) {
	n := len(g.kids)
	if n == 1 {
		g.kids[0].Layout.X, g.kids[0].Layout.Y = padX, padTop
		return g.sz[g.kids[0].ID][0], g.sz[g.kids[0].ID][1]
	}
	area := 0.0
	for _, s := range g.sz {
		area += (s[0] + l.o.NodeSpacing) * (s[1] + l.o.NodeSpacing)
	}
	side := math.Sqrt(area) * 1.4
	pos := map[string][2]float64{}
	for i, k := range g.order {
		a := 2 * math.Pi * float64(i) / float64(n)
		s := g.sz[k.ID]
		pos[k.ID] = [2]float64{side/2 + side/3*math.Cos(a) - s[0]/2, side/2 + side/3*math.Sin(a) - s[1]/2}
	}
	kf := 0.8 * math.Sqrt(side*side/float64(n))
	temp := side / 8
	center := func(id string) (float64, float64) {
		s := g.sz[id]
		return pos[id][0] + s[0]/2, pos[id][1] + s[1]/2
	}
	for it := 0; it < 300; it++ {
		disp := map[string][2]float64{}
		for _, a := range g.kids {
			ax, ay := center(a.ID)
			for _, b := range g.kids {
				if a == b {
					continue
				}
				bx, by := center(b.ID)
				dx, dy := ax-bx, ay-by
				dist := math.Max(1, math.Hypot(dx, dy))
				// box-aware repulsion: stronger while the boxes still overlap
				need := (g.sz[a.ID][0]+g.sz[b.ID][0])/2 + l.o.NodeSpacing
				f := kf * kf / dist
				if dist < need {
					f += (need - dist) * 2
				}
				disp[a.ID] = [2]float64{disp[a.ID][0] + dx/dist*f, disp[a.ID][1] + dy/dist*f}
			}
			// weak gravity keeps disconnected pieces together instead of at the walls
			gx, gy := side/2-ax, side/2-ay
			disp[a.ID] = [2]float64{disp[a.ID][0] + gx*0.08, disp[a.ID][1] + gy*0.08}
		}
		for a, ts := range g.out {
			for b := range ts {
				ax, ay := center(a)
				bx, by := center(b)
				dx, dy := ax-bx, ay-by
				dist := math.Max(1, math.Hypot(dx, dy))
				ideal := (g.sz[a][0]+g.sz[b][0])/2 + l.o.RankSpacing
				f := (dist - ideal) * dist / kf / 4
				disp[a] = [2]float64{disp[a][0] - dx/dist*f, disp[a][1] - dy/dist*f}
				disp[b] = [2]float64{disp[b][0] + dx/dist*f, disp[b][1] + dy/dist*f}
			}
		}
		for _, k := range g.kids {
			d := disp[k.ID]
			mag := math.Max(1, math.Hypot(d[0], d[1]))
			step := math.Min(mag, temp)
			s := g.sz[k.ID]
			// the whole box stays inside the target square
			nx := math.Max(0, math.Min(side-s[0], pos[k.ID][0]+d[0]/mag*step))
			ny := math.Max(0, math.Min(side-s[1], pos[k.ID][1]+d[1]/mag*step))
			pos[k.ID] = [2]float64{nx, ny}
		}
		temp = math.Max(1, temp*0.95)
	}
	for _, k := range g.kids {
		k.Layout.X, k.Layout.Y = pos[k.ID][0], pos[k.ID][1]
	}
	return normalise(g.kids, g.sz)
}

// circle places the siblings on one ring, in reading order.
func (l *layouter) circle(g *graph) (float64, float64) {
	n := len(g.order)
	maxW, maxH := 0.0, 0.0
	for _, s := range g.sz {
		maxW, maxH = math.Max(maxW, s[0]), math.Max(maxH, s[1])
	}
	radius := 0.0
	if n > 1 {
		radius = math.Max(float64(n)*(math.Max(maxW, maxH)+l.o.NodeSpacing)/(2*math.Pi), maxW)
	}
	for i, k := range g.order {
		a := 2*math.Pi*float64(i)/float64(n) - math.Pi/2
		s := g.sz[k.ID]
		k.Layout.X, k.Layout.Y = radius*math.Cos(a)-s[0]/2, radius*math.Sin(a)-s[1]/2
	}
	return normalise(g.kids, g.sz)
}

// grid tiles the siblings in rows, containers first.
func (l *layouter) grid(g *graph, nodes []*document.Node) (float64, float64) {
	sorted := append([]*document.Node{}, nodes...)
	sort.SliceStable(sorted, func(i, j int) bool { return l.isContainer(sorted[i]) && !l.isContainer(sorted[j]) })
	perRow := int(math.Max(1, math.Min(4, math.Ceil(math.Sqrt(float64(len(sorted)))))))
	x, y, rowH := float64(padX), float64(padTop), 0.0
	maxW := 0.0
	for i, k := range sorted {
		if i > 0 && i%perRow == 0 {
			x = padX
			y += rowH + l.o.NodeSpacing
			rowH = 0
		}
		s := g.sz[k.ID]
		k.Layout.X, k.Layout.Y = x, y
		x += s[0] + l.o.NodeSpacing
		rowH = math.Max(rowH, s[1])
		maxW = math.Max(maxW, x-l.o.NodeSpacing)
	}
	return maxW - padX, y + rowH - padTop
}

// zoneKey orders zones a, b, c (or 1, 2, 3) by their zone property.
func zoneKey(n *document.Node) string {
	for _, k := range []string{"zone", "az"} {
		if v, ok := n.Props[k].(string); ok {
			return v
		}
	}
	return ""
}
