// Package restapi is a minimal Figma REST API (api.figma.com) client plus
// renderers that shape REST payloads into the same output forms the official
// MCP server produces (sparse metadata XML, design-context text, variable
// definitions, screenshot image blocks), so the CLI's command surface works
// unchanged over either backend.
package restapi

// Rect is a bounding box in document coordinates.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Color is an sRGB float color (components 0..1).
type Color struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

// Paint is one fill/stroke entry (solid, gradient or image).
type Paint struct {
	Type      string  `json:"type"`
	Visible   *bool   `json:"visible,omitempty"`
	Opacity   float64 `json:"opacity,omitempty"`
	Color     *Color  `json:"color,omitempty"`
	ImageRef  string  `json:"imageRef,omitempty"`
	ScaleMode string  `json:"scaleMode,omitempty"`
}

// TypeStyle carries text styling of a text node.
type TypeStyle struct {
	FontFamily          string  `json:"fontFamily"`
	FontPostScriptName  string  `json:"fontPostScriptName,omitempty"`
	FontWeight          float64 `json:"fontWeight"`
	FontSize            float64 `json:"fontSize"`
	LineHeightPx        float64 `json:"lineHeightPx,omitempty"`
	LetterSpacing       float64 `json:"letterSpacing,omitempty"`
	TextCase            string  `json:"textCase,omitempty"`
	TextAlignHorizontal string  `json:"textAlignHorizontal,omitempty"`
	TextAlignVertical   string  `json:"textAlignVertical,omitempty"`
}

// Effect is a shadow/blur effect.
type Effect struct {
	Type    string  `json:"type"`
	Visible *bool   `json:"visible,omitempty"`
	Color   *Color  `json:"color,omitempty"`
	Radius  float64 `json:"radius,omitempty"`
	Spread  float64 `json:"spread,omitempty"`
}

// BoundVar references a Figma variable bound to a node property.
type BoundVar struct {
	Type string `json:"type"` // "VARIABLE_ALIAS" | "BINDING"
	ID   string `json:"id"`
}

// Node is the subset of the Figma document node the renderers need.
type Node struct {
	ID                     string                   `json:"id"`
	Name                   string                   `json:"name"`
	Type                   string                   `json:"type"`
	Visible                *bool                    `json:"visible,omitempty"`
	Opacity                float64                  `json:"opacity,omitempty"`
	Children               []Node                   `json:"children,omitempty"`
	AbsoluteBoundingBox    *Rect                    `json:"absoluteBoundingBox,omitempty"`
	Fills                  []Paint                  `json:"fills,omitempty"`
	Strokes                []Paint                  `json:"strokes,omitempty"`
	StrokeWeight           float64                  `json:"strokeWeight,omitempty"`
	StrokeAlign            string                   `json:"strokeAlign,omitempty"`
	CornerRadius           float64                  `json:"cornerRadius,omitempty"`
	RectangleCornerRadii   []float64                `json:"rectangleCornerRadii,omitempty"`
	LayoutMode             string                   `json:"layoutMode,omitempty"`
	LayoutWrap             string                   `json:"layoutWrap,omitempty"`
	ItemSpacing            float64                  `json:"itemSpacing,omitempty"`
	CounterAxisSpacing     float64                  `json:"counterAxisSpacing,omitempty"`
	PaddingLeft            float64                  `json:"paddingLeft,omitempty"`
	PaddingRight           float64                  `json:"paddingRight,omitempty"`
	PaddingTop             float64                  `json:"paddingTop,omitempty"`
	PaddingBottom          float64                  `json:"paddingBottom,omitempty"`
	PrimaryAxisAlignItems  string                   `json:"primaryAxisAlignItems,omitempty"`
	CounterAxisAlignItems  string                   `json:"counterAxisAlignItems,omitempty"`
	LayoutSizingHorizontal string                   `json:"layoutSizingHorizontal,omitempty"`
	LayoutSizingVertical   string                   `json:"layoutSizingVertical,omitempty"`
	LayoutGrids            []LayoutGrid             `json:"layoutGrids,omitempty"`
	Characters             string                   `json:"characters,omitempty"`
	Style                  *TypeStyle               `json:"style,omitempty"`
	ComponentID            string                   `json:"componentId,omitempty"`
	ComponentProperties    map[string]ComponentProp `json:"componentProperties,omitempty"`
	Effects                []Effect                 `json:"effects,omitempty"`
	BoundVariables         map[string][]BoundVar    `json:"boundVariables,omitempty"`
	Constraints            *Constraints             `json:"constraints,omitempty"`
}

// LayoutGrid describes a grid layout on a frame.
type LayoutGrid struct {
	Pattern    string  `json:"pattern"`
	Section    bool    `json:"section,omitempty"`
	Alignment  string  `json:"alignment,omitempty"`
	GutterSize float64 `json:"gutterSize,omitempty"`
	Offset     float64 `json:"offset,omitempty"`
	Count      float64 `json:"count,omitempty"`
}

// Constraints describes how a node is sized/positioned relative to its parent.
type Constraints struct {
	Horizontal string `json:"horizontal"`
	Vertical   string `json:"vertical"`
}

// ComponentProp describes one component property / variant.
type ComponentProp struct {
	Type            string             `json:"type"`
	Value           any                `json:"value"`
	PreferredValues []ComponentPropVal `json:"preferredValues,omitempty"`
}

// ComponentPropVal is one option of a variant property.
type ComponentPropVal struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// File is the /v1/files/{key} response subset.
type File struct {
	Name         string `json:"name"`
	LastModified string `json:"lastModified"`
	Document     Node   `json:"document"`
}

// NodeDoc wraps one node returned by /v1/files/{key}/nodes.
type NodeDoc struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Document Node   `json:"document"`
}

// NodesResponse is the /v1/files/{key}/nodes payload.
type NodesResponse struct {
	Name  string             `json:"name"`
	Nodes map[string]NodeDoc `json:"nodes"`
}

// ImagesResponse is the /v1/images/{key} payload.
type ImagesResponse struct {
	Images map[string]string `json:"images"`
}

// User is the /v1/me payload.
type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Handle string `json:"handle"`
}

// Variable is one Figma variable (design token) definition.
type Variable struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Key                  string         `json:"key"`
	Remote               bool           `json:"remote"`
	VariableCollectionID string         `json:"variableCollectionId"`
	ResolvedType         string         `json:"resolvedType"`
	Description          string         `json:"description,omitempty"`
	Scopes               []string       `json:"scopes,omitempty"`
	ValuesByMode         map[string]any `json:"valuesByMode"`
}

// Mode is one variable collection mode (e.g. Light / Dark).
type Mode struct {
	ModeID string `json:"modeId"`
	Name   string `json:"name"`
}

// Collection groups variables and lists its modes.
type Collection struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Key           string `json:"key"`
	Remote        bool   `json:"remote"`
	DefaultModeID string `json:"defaultModeId"`
	Modes         []Mode `json:"modes"`
}

// VariablesResponse is the /v1/files/{key}/variables/* payload.
type VariablesResponse struct {
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
	Meta   struct {
		Variables           map[string]Variable   `json:"variables"`
		VariableCollections map[string]Collection `json:"variableCollections"`
	} `json:"meta"`
}
