package restapi

import (
	"fmt"
	"math"
	"strings"
)

// RenderTreeXML renders a node subtree as the sparse XML shape the official
// MCP get_metadata tool emits (id/name/type/bounds attributes only), so the
// xmlscan-based pipeline works unchanged over the REST backend.
func RenderTreeXML(n *Node) string {
	var b strings.Builder
	writeNodeXML(&b, n)
	return b.String()
}

func writeNodeXML(b *strings.Builder, n *Node) {
	if n == nil {
		return
	}
	tag := strings.ToLower(n.Type)
	if tag == "" {
		tag = "node"
	}
	b.WriteString("<" + tag)
	xmlAttr(b, "id", n.ID)
	xmlAttr(b, "name", n.Name)
	xmlAttr(b, "type", n.Type)
	if bb := n.AbsoluteBoundingBox; bb != nil {
		xmlAttr(b, "x", ftoa(bb.X))
		xmlAttr(b, "y", ftoa(bb.Y))
		xmlAttr(b, "width", ftoa(bb.Width))
		xmlAttr(b, "height", ftoa(bb.Height))
	}
	if len(n.Children) > 0 {
		b.WriteString(">")
		for i := range n.Children {
			writeNodeXML(b, &n.Children[i])
		}
		b.WriteString("</" + tag + ">")
	} else {
		b.WriteString("/>")
	}
}

// RenderPagesXML renders the file header plus one element per page.
func RenderPagesXML(key string, f *File) string {
	var b strings.Builder
	b.WriteString("<file")
	xmlAttr(&b, "key", key)
	xmlAttr(&b, "name", f.Name)
	xmlAttr(&b, "lastModified", f.LastModified)
	b.WriteString("/>")
	for i := range f.Document.Children {
		p := &f.Document.Children[i]
		b.WriteString("\n<page")
		xmlAttr(&b, "id", p.ID)
		xmlAttr(&b, "name", p.Name)
		xmlAttr(&b, "type", p.Type)
		xmlAttr(&b, "frames", fmt.Sprint(len(p.Children)))
		b.WriteString("/>")
	}
	return b.String()
}

func xmlAttr(b *strings.Builder, k, v string) {
	if v == "" {
		return
	}
	b.WriteString(" " + k + `="` + xmlEscape(v) + `"`)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

// ftoa prints a float without trailing zeros.
func ftoa(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}
