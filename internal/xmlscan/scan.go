// Package xmlscan extracts a node tree from the sparse XML returned by the
// official MCP get_metadata tool (id/name/type/bounds attributes only),
// without needing the full Figma REST document.
package xmlscan

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Node is one element of the sparse tree.
type Node struct {
	ID          string
	Name        string
	Type        string
	ComponentID string
	LayoutMode  string
	Visible     *bool
	Children    []*Node
}

// DirectChildren returns the root's immediate children.
func (t *Node) DirectChildren() []*Node {
	if t == nil {
		return nil
	}
	return t.Children
}

// Find returns the first node (depth-first) with the given id.
func (t *Node) Find(id string) *Node {
	if t == nil {
		return nil
	}
	if t.ID == id {
		return t
	}
	for _, c := range t.Children {
		if n := c.Find(id); n != nil {
			return n
		}
	}
	return nil
}

// Frames returns all nodes whose type looks like a container/frame
// (frame, group, component, instance, section), depth-first.
func (t *Node) Frames() []*Node {
	var out []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if isFrameType(n.Type) {
			out = append(out, n)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(t)
	return out
}

func isFrameType(t string) bool {
	switch strings.ToLower(t) {
	case "frame", "group", "component", "instance", "section":
		return true
	}
	return false
}

// Parse builds a tree from a sparse-XML get_metadata payload. The root of
// the returned tree is a synthetic "document" whose children are the top
// level elements found in the text.
func Parse(text string) (*Node, error) {
	dec := xml.NewDecoder(strings.NewReader(text))
	dec.Strict = false
	root := &Node{Type: "document"}
	stack := []*Node{root}

	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" || strings.Contains(err.Error(), "EOF") {
				break
			}
			// Tolerate trailing garbage but not structural breakage mid-tree.
			if len(stack) == 1 {
				break
			}
			return nil, fmt.Errorf("xmlscan: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			n := &Node{Type: el.Name.Local}
			for _, a := range el.Attr {
				switch strings.ToLower(a.Name.Local) {
				case "id":
					n.ID = a.Value
				case "name":
					n.Name = a.Value
				case "type":
					// Element-local type attr wins over tag name when present.
					if a.Value != "" {
						n.Type = a.Value
					}
				case "componentid":
					n.ComponentID = a.Value
				case "layoutmode":
					n.LayoutMode = a.Value
				case "visible":
					if a.Value == "false" {
						f := false
						n.Visible = &f
					}
				}
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(root.Children) == 0 {
		return nil, fmt.Errorf("xmlscan: no elements found in metadata payload")
	}
	return root, nil
}

// Root returns the single root element if the document has exactly one,
// otherwise the synthetic document node.
func Root(t *Node) *Node {
	if t != nil && t.Type == "document" && len(t.Children) == 1 {
		return t.Children[0]
	}
	return t
}

// Grep filters the tree to nodes whose Name or Type contains pattern
// (case-insensitive). Each match includes its full ancestor path so
// the caller knows where it lives in the tree. Returns nil if no match.
func (t *Node) Grep(pattern string) []*Node {
	if t == nil || pattern == "" {
		return nil
	}
	lc := strings.ToLower(pattern)
	var out []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if strings.Contains(strings.ToLower(n.Name), lc) ||
			strings.Contains(strings.ToLower(n.Type), lc) {
			out = append(out, n)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(t)
	return out
}

// Path returns the ancestor chain from root to this node (inclusive),
// useful for displaying where a grep hit lives in the tree.
func (t *Node) Path(target *Node) []*Node {
	if t == nil || target == nil {
		return nil
	}
	var path []*Node
	var search func(n *Node) bool
	search = func(n *Node) bool {
		if n == nil {
			return false
		}
		path = append(path, n)
		if n == target {
			return true
		}
		for _, c := range n.Children {
			if search(c) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	search(t)
	return path
}
