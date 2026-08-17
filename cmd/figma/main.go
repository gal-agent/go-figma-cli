// Command figma wraps the official Figma MCP server for agent-friendly,
// context-efficient CLI access.
package main

import "github.com/piratecoder/go-figma-cli/internal/cli"

func main() {
	cli.Execute()
}
