package main

import (
	"fmt"
	"strings"
)

func traverseNode(graph *Graph, basePath string, nodeID string, depth int, processedNodes map[string]bool) {
	fmt.Printf("Traversing node: %s at depth %d\n", nodeID, depth)

	if depth > graph.MaxDepth || processedNodes[nodeID] {
		return
	}

	processedNodes[nodeID] = true

	// Check if this is a module reference
	if strings.HasPrefix(nodeID, "module.") {
		moduleName := strings.Split(nodeID, ".")[1]
		// Split for nested module attributes (e.g., module.name.email)
		if parts := strings.SplitN(moduleName, ".", 2); len(parts) > 1 {
			moduleName = parts[0]
		}

		modulePath, moduleNode := findModuleNode(basePath, fmt.Sprintf("module.%s", moduleName))
		if modulePath != "" {
			fmt.Printf("Found module %s at path: %s\n", moduleName, modulePath)

			// Store the module node with its full ID (including attributes)
			moduleNode.Depth = depth
			graph.Nodes[nodeID] = moduleNode

			moduleSourcePath := resolvePath(basePath, modulePath)
			fmt.Printf("Traversing module source directory: %s\n", moduleSourcePath)
			traverseModuleDirectory(graph, moduleSourcePath, fmt.Sprintf("module.%s", moduleName), depth, processedNodes)
		} else {
			fmt.Printf("Module not found: %s\n", moduleName)
		}
	}

	node, found := findResourceNode(basePath, nodeID)
	if !found {
		return
	}

	node.Depth = depth
	graph.Nodes[nodeID] = node

	for category, refs := range node.References {
		for _, ref := range refs {
			if !strings.HasPrefix(ref.ID, "var.") && !strings.HasPrefix(ref.ID, "local.") {
				traverseNode(graph, basePath, ref.ID, depth+1, processedNodes)
			}
			fmt.Printf("Found %s references for %s: %v\n", category, nodeID, ref)
		}
	}
}

func generateDOT(graph Graph) string {
	var builder strings.Builder
	builder.WriteString("digraph terraform {\n")
	builder.WriteString("  rankdir = LR;\n")
	builder.WriteString("  compound = true;\n\n")

	writeNodeStyles(&builder)
	writeNodes(&builder, graph)
	writeDependencies(&builder, graph)

	builder.WriteString("}\n")
	return builder.String()
}

func writeNodeStyles(builder *strings.Builder) {
	builder.WriteString("  // Node styles\n")
	builder.WriteString("  node [shape=box, style=rounded];\n\n")
}

func writeNodes(builder *strings.Builder, graph Graph) {
	colors := map[string]string{
		"variable": "#FFB6C1", // Light pink
		"module":   "#98FB98", // Pale green
		"data":     "#87CEEB", // Sky blue
		"resource": "#DDA0DD", // Plum
		"output":   "#FFA07A", // Light salmon
	}

	// Main resource node
	for id, node := range graph.Nodes {
		if node.Depth == 0 {
			builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", color=red];\n",
				id, id, node.Path))
		}
	}

	builder.WriteString("\n  // Referenced nodes\n")
	// Referenced nodes
	for _, node := range graph.Nodes {
		if node.Depth == 0 {
			for category, refs := range node.References {
				color := colors[category]
				if color == "" {
					color = "#DCDCDC" // Default color for unknown types
				}
				for _, ref := range refs {
					builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
						ref.ID, ref.ID, ref.Path, color))
				}
			}
		}
	}
}

func writeDependencies(builder *strings.Builder, graph Graph) {
	builder.WriteString("\n  // Dependencies\n")
	for _, node := range graph.Nodes {
		if node.Depth == 0 {
			for _, refs := range node.References {
				for _, ref := range refs {
					builder.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [style=solid];\n",
						ref.ID, node.ID))
				}
			}
		}
	}
}
