package restapi

import (
	"fmt"
	"sort"
	"strings"
)

// RenderVariables renders variable (design token) definitions grouped by
// collection, one line per variable with values per mode, mirroring the
// compact text form of the MCP get_variable_defs tool.
func RenderVariables(vr *VariablesResponse) string {
	if vr == nil || len(vr.Meta.Variables) == 0 {
		return "(no variables found)"
	}
	// Group variables by collection.
	byColl := map[string][]Variable{}
	for _, v := range vr.Meta.Variables {
		byColl[v.VariableCollectionID] = append(byColl[v.VariableCollectionID], v)
	}
	collIDs := make([]string, 0, len(byColl))
	for id := range byColl {
		collIDs = append(collIDs, id)
	}
	sort.Slice(collIDs, func(i, j int) bool {
		ci, cj := vr.Meta.VariableCollections[collIDs[i]], vr.Meta.VariableCollections[collIDs[j]]
		return collectionName(ci) < collectionName(cj)
	})

	var b strings.Builder
	for _, collID := range collIDs {
		coll := vr.Meta.VariableCollections[collID]
		vars := byColl[collID]
		sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
		remote := ""
		if coll.Remote {
			remote = " (library)"
		}
		fmt.Fprintf(&b, "## %s%s\n", collectionName(coll), remote)
		for _, v := range vars {
			desc := ""
			if v.Description != "" {
				desc = " -- " + v.Description
			}
			fmt.Fprintf(&b, "  %s: %s [%s]%s\n", v.Name, modeValues(vr, v), strings.ToLower(v.ResolvedType), desc)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func collectionName(c Collection) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

// modeValues formats one variable's values across collection modes.
func modeValues(vr *VariablesResponse, v Variable) string {
	if len(vr.Meta.VariableCollections) == 0 || len(v.ValuesByMode) <= 1 {
		return valueString(vr, firstValue(v.ValuesByMode))
	}
	coll := vr.Meta.VariableCollections[v.VariableCollectionID]
	parts := make([]string, 0, len(coll.Modes))
	for _, m := range coll.Modes {
		raw, ok := v.ValuesByMode[m.ModeID]
		if !ok {
			continue
		}
		parts = append(parts, m.Name+"="+valueString(vr, raw))
	}
	if len(parts) == 0 {
		return valueString(vr, firstValue(v.ValuesByMode))
	}
	return strings.Join(parts, " ")
}

func firstValue(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return m[keys[0]]
}

// valueString renders one raw valuesByMode entry:
// {"r":..,"g":..,"b":..,"a":..} -> #RRGGBB; {"type":"VARIABLE_ALIAS",...} -> var(name);
// primitives are printed as-is.
func valueString(vr *VariablesResponse, raw any) string {
	switch val := raw.(type) {
	case nil:
		return "?"
	case map[string]any:
		if val["type"] == "VARIABLE_ALIAS" {
			id, _ := val["id"].(string)
			return "var(" + variableName(vr, id) + ")"
		}
		r, _ := val["r"].(float64)
		g, _ := val["g"].(float64)
		b, _ := val["b"].(float64)
		a, _ := val["a"].(float64)
		s := fmt.Sprintf("#%02X%02X%02X", roundByte(r), roundByte(g), roundByte(b))
		if a < 1 {
			s += fmt.Sprintf(" alpha=%.2f", a)
		}
		return s
	case float64:
		return ftoa(val)
	case string:
		return val
	default:
		return fmt.Sprint(val)
	}
}

func variableName(vr *VariablesResponse, id string) string {
	if vr != nil {
		if v, ok := vr.Meta.Variables[id]; ok && v.Name != "" {
			return v.Name
		}
	}
	return id
}
