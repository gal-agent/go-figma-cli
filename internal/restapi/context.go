package restapi

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// RenderContext renders a design-context style text summary of a node
// subtree: geometry, auto-layout, fills/strokes, text styling, component
// references. It is a REST-derived design representation, not Dev Mode code
// generation; the consumer (agent) translates it to the target stack.
func RenderContext(n *Node, maxDepth int) string {
	if maxDepth <= 0 {
		maxDepth = 6
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<design-context node=%q name=%q type=%q backend=rest>\n", n.ID, n.Name, n.Type)
	writeContextNode(&b, n, 0, maxDepth)
	b.WriteString("</design-context>\n")
	return b.String()
}

func writeContextNode(b *strings.Builder, n *Node, depth, maxDepth int) {
	indent := strings.Repeat("  ", depth)
	if depth >= maxDepth {
		fmt.Fprintf(b, "%s- [%s] %q %s (+%d children, truncated at depth %d)\n",
			indent, n.ID, n.Name, n.Type, len(n.Children), maxDepth)
		return
	}
	fmt.Fprintf(b, "%s- [%s] %q %s", indent, n.ID, n.Name, n.Type)
	if bb := n.AbsoluteBoundingBox; bb != nil {
		fmt.Fprintf(b, " %sx%s @(%s,%s)", ftoa(bb.Width), ftoa(bb.Height), ftoa(bb.X), ftoa(bb.Y))
	}
	if n.ComponentID != "" {
		fmt.Fprintf(b, " instance-of=%s", n.ComponentID)
	}
	if n.Opacity > 0 && n.Opacity < 1 {
		fmt.Fprintf(b, " opacity=%.2f", n.Opacity)
	}
	b.WriteString("\n")

	sub := indent + "  "
	if n.LayoutMode != "" && n.LayoutMode != "NONE" {
		b.WriteString(sub + "layout: " + strings.ToLower(n.LayoutMode))
		if n.ItemSpacing > 0 {
			fmt.Fprintf(b, " gap=%s", ftoa(n.ItemSpacing))
		}
		if n.PaddingLeft > 0 || n.PaddingTop > 0 || n.PaddingRight > 0 || n.PaddingBottom > 0 {
			fmt.Fprintf(b, " padding=[l%s t%s r%s b%s]",
				ftoa(n.PaddingLeft), ftoa(n.PaddingTop), ftoa(n.PaddingRight), ftoa(n.PaddingBottom))
		}
		if n.PrimaryAxisAlignItems != "" {
			fmt.Fprintf(b, " primary=%s", strings.ToLower(n.PrimaryAxisAlignItems))
		}
		if n.CounterAxisAlignItems != "" {
			fmt.Fprintf(b, " counter=%s", strings.ToLower(n.CounterAxisAlignItems))
		}
		if n.LayoutSizingHorizontal != "" || n.LayoutSizingVertical != "" {
			fmt.Fprintf(b, " sizing=%s/%s", sizeOrFixed(n.LayoutSizingHorizontal), sizeOrFixed(n.LayoutSizingVertical))
		}
		b.WriteString("\n")
	}
	for _, p := range visiblePaints(n.Fills) {
		fmt.Fprintf(b, "%sfill  : %s\n", sub, paintString(p))
	}
	for _, p := range visiblePaints(n.Strokes) {
		w := ""
		if n.StrokeWeight > 0 {
			w = " " + ftoa(n.StrokeWeight)
		}
		align := ""
		if n.StrokeAlign != "" {
			align = " (" + strings.ToLower(n.StrokeAlign) + ")"
		}
		fmt.Fprintf(b, "%sstroke: %s%s%s\n", sub, paintString(p), w, align)
	}
	if n.CornerRadius > 0 {
		fmt.Fprintf(b, "%sradius: %s\n", sub, ftoa(n.CornerRadius))
	} else if len(n.RectangleCornerRadii) == 4 {
		fmt.Fprintf(b, "%sradius: [%s %s %s %s]\n", sub,
			ftoa(n.RectangleCornerRadii[0]), ftoa(n.RectangleCornerRadii[1]),
			ftoa(n.RectangleCornerRadii[2]), ftoa(n.RectangleCornerRadii[3]))
	}
	for _, e := range n.Effects {
		if e.Visible != nil && !*e.Visible {
			continue
		}
		fmt.Fprintf(b, "%seffect: %s", sub, strings.ToLower(e.Type))
		if e.Radius > 0 {
			fmt.Fprintf(b, " radius=%s", ftoa(e.Radius))
		}
		if e.Color != nil {
			fmt.Fprintf(b, " %s", colorString(e.Color))
		}
		b.WriteString("\n")
	}
	if n.Type == "TEXT" {
		fmt.Fprintf(b, "%stext  : %q\n", sub, truncateRunes(n.Characters, 200))
		if st := n.Style; st != nil {
			fmt.Fprintf(b, "%sstyle : %s/%s %spx", sub, st.FontFamily, weightString(st.FontWeight), ftoa(st.FontSize))
			if st.LineHeightPx > 0 {
				fmt.Fprintf(b, " lh=%s", ftoa(st.LineHeightPx))
			}
			if st.LetterSpacing != 0 {
				fmt.Fprintf(b, " ls=%s", ftoa(st.LetterSpacing))
			}
			if st.TextCase != "" && st.TextCase != "ORIGINAL" {
				fmt.Fprintf(b, " case=%s", strings.ToLower(st.TextCase))
			}
			if st.TextAlignHorizontal != "" && st.TextAlignHorizontal != "LEFT" {
				fmt.Fprintf(b, " align=%s", strings.ToLower(st.TextAlignHorizontal))
			}
			b.WriteString("\n")
		}
	}
	if len(n.BoundVariables) > 0 {
		ids := make([]string, 0, len(n.BoundVariables))
		for prop, bvs := range n.BoundVariables {
			parts := make([]string, 0, len(bvs))
			for _, bv := range bvs {
				parts = append(parts, bv.ID)
			}
			ids = append(ids, prop+"->"+strings.Join(parts, "|"))
		}
		fmt.Fprintf(b, "%sbound : %s\n", sub, strings.Join(ids, " "))
	}
	if n.Constraints != nil {
		fmt.Fprintf(b, "%sconstrain: %s/%s\n", sub,
			strings.ToLower(n.Constraints.Horizontal),
			strings.ToLower(n.Constraints.Vertical))
	}
	if len(n.ComponentProperties) > 0 {
		props := make([]string, 0, len(n.ComponentProperties))
		for name, cp := range n.ComponentProperties {
			val := fmt.Sprintf("%v", cp.Value)
			if cp.Type == "VARIANT" {
				vals := make([]string, 0, len(cp.PreferredValues))
				for _, pv := range cp.PreferredValues {
					vals = append(vals, pv.Name)
				}
				if len(vals) > 0 {
					val = strings.Join(vals, "|")
				}
			}
			props = append(props, fmt.Sprintf("%s=%s", name, val))
		}
		sort.Strings(props)
		fmt.Fprintf(b, "%sprops : %s\n", sub, strings.Join(props, " "))
	}
	if len(n.LayoutGrids) > 0 {
		for _, g := range n.LayoutGrids {
			s := strings.ToLower(g.Pattern)
			if g.Alignment != "" {
				s += " " + strings.ToLower(g.Alignment)
			}
			if g.Count > 0 {
				s += fmt.Sprintf(" count=%s", ftoa(g.Count))
			}
			if g.GutterSize > 0 {
				s += fmt.Sprintf(" gutter=%s", ftoa(g.GutterSize))
			}
			fmt.Fprintf(b, "%sgrid : %s\n", sub, s)
		}
	}
	for i := range n.Children {
		if n.Children[i].Visible != nil && !*n.Children[i].Visible {
			continue
		}
		writeContextNode(b, &n.Children[i], depth+1, maxDepth)
	}
}

func visiblePaints(ps []Paint) []Paint {
	var out []Paint
	for _, p := range ps {
		if p.Visible != nil && !*p.Visible {
			continue
		}
		out = append(out, p)
	}
	return out
}

func paintString(p Paint) string {
	switch p.Type {
	case "SOLID":
		if p.Color != nil {
			return colorString(p.Color)
		}
		return "solid"
	case "IMAGE":
		s := "image"
		if p.ScaleMode != "" {
			s += " (" + strings.ToLower(p.ScaleMode) + ")"
		}
		return s
	default:
		return strings.ToLower(p.Type) // gradient variants
	}
}

func colorString(c *Color) string {
	hex := fmt.Sprintf("#%02X%02X%02X", roundByte(c.R), roundByte(c.G), roundByte(c.B))
	if c.A < 1 {
		return fmt.Sprintf("%s alpha=%s", hex, ftoa(c.A))
	}
	return hex
}

func roundByte(v float64) byte {
	return byte(math.Round(v*255)) & 0xFF
}

func weightString(w float64) string {
	switch w {
	case 100:
		return "Thin"
	case 200:
		return "ExtraLight"
	case 300:
		return "Light"
	case 400:
		return "Regular"
	case 500:
		return "Medium"
	case 600:
		return "SemiBold"
	case 700:
		return "Bold"
	case 800:
		return "ExtraBold"
	case 900:
		return "Black"
	default:
		return ftoa(w)
	}
}

func sizeOrFixed(s string) string {
	if s == "" {
		return "fixed"
	}
	return strings.ToLower(s)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
