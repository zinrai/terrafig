package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type Node struct {
	ID         string
	Type       string
	Name       string
	Path       string
	References map[string][]Reference // Changed from map[string][]string to store file paths
	Depth      int
}

// New struct to store both reference ID and its file path
type Reference struct {
	ID   string
	Path string
}

type Graph struct {
	Nodes    map[string]Node
	MaxDepth int
}

func main() {
	path := flag.String("path", "", "Path to Terraform file")
	resourceType := flag.String("type", "", "Resource type (e.g., aws_instance)")
	resourceName := flag.String("name", "", "Resource name")
	output := flag.String("output", "graph.dot", "Output file path")
	format := flag.String("format", "dot", "Output format (dot)")
	maxDepth := flag.Int("depth", 3, "Maximum depth of dependency tracking")
	flag.Parse()

	if *path == "" || *resourceType == "" || *resourceName == "" {
		fmt.Println("Error: path, type, and name are required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if _, err := os.Stat(*path); os.IsNotExist(err) {
		fmt.Printf("Error: File %s does not exist\n", *path)
		os.Exit(1)
	}

	graph := Graph{
		Nodes:    make(map[string]Node),
		MaxDepth: *maxDepth,
	}

	targetID := fmt.Sprintf("%s.%s", *resourceType, *resourceName)
	basePath := filepath.Dir(*path)
	buildDependencyGraph(&graph, basePath, targetID)

	var content string
	switch *format {
	case "dot":
		content = generateDOT(graph)
	default:
		fmt.Printf("Error: Unsupported format %s\n", *format)
		os.Exit(1)
	}

	err := os.WriteFile(*output, []byte(content), 0644)
	if err != nil {
		fmt.Printf("Error writing output: %v\n", err)
		os.Exit(1)
	}
}

func buildDependencyGraph(graph *Graph, basePath string, targetID string) {
	processedNodes := make(map[string]bool)
	traverseNode(graph, basePath, targetID, 0, processedNodes)
}

func traverseNode(graph *Graph, basePath string, nodeID string, depth int, processedNodes map[string]bool) {
	if depth > graph.MaxDepth || processedNodes[nodeID] {
		return
	}

	processedNodes[nodeID] = true

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
		}
		fmt.Printf("Found %s references for %s: %v\n", category, nodeID, refs)
	}
}

func findResourceNode(basePath string, nodeID string) (Node, bool) {
	fmt.Printf("Searching for nodeID: %s in %s\n", nodeID, basePath)

	var files []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(info.Name(), ".tf") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		return Node{}, false
	}

	fmt.Printf("Found Terraform files: %v\n", files)
	parser := hclparse.NewParser()

	for _, file := range files {
		fmt.Printf("Parsing file: %s\n", file)
		f, diags := parser.ParseHCLFile(file)
		if diags.HasErrors() {
			fmt.Printf("Diagnostic errors in %s: %s\n", file, diags.Error())
			continue
		}

		content, diags := f.Body.Content(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "resource", LabelNames: []string{"type", "name"}},
				{Type: "module", LabelNames: []string{"name"}},
				{Type: "data", LabelNames: []string{"type", "name"}},
				{Type: "variable", LabelNames: []string{"name"}},
				{Type: "output", LabelNames: []string{"name"}},
				{Type: "provider", LabelNames: []string{"name"}},
				{Type: "terraform", LabelNames: []string{}},
				{Type: "locals", LabelNames: []string{}},
			},
		})
		if diags.HasErrors() {
			fmt.Printf("Content errors in %s: %s\n", file, diags.Error())
			continue
		}

		for _, block := range content.Blocks {
			var currentID string
			switch block.Type {
			case "resource":
				currentID = fmt.Sprintf("%s.%s", block.Labels[0], block.Labels[1])
			case "module":
				currentID = fmt.Sprintf("module.%s", block.Labels[0])
			case "data":
				currentID = fmt.Sprintf("data.%s.%s", block.Labels[0], block.Labels[1])
			}

			if currentID == nodeID {
				fmt.Printf("Found matching block: %s\n", currentID)
				refs := extractReferences(block, file)
				return Node{
					ID:         currentID,
					Type:       block.Type,
					Name:       block.Labels[len(block.Labels)-1],
					Path:       file,
					References: refs,
				}, true
			}
		}
	}

	fmt.Printf("Resource not found: %s\n", nodeID)
	return Node{}, false
}

func extractReferences(block *hcl.Block, filePath string) map[string][]Reference {
	refs := make(map[string][]Reference)
	attrs, _ := block.Body.JustAttributes()

	for name, attr := range attrs {
		fmt.Printf("Analyzing attribute: %s\n", name)
		vars := attr.Expr.Variables()

		for _, v := range vars {
			ref := traversalToReference(v)
			if ref == "" {
				continue
			}

			parts := strings.SplitN(ref, ".", 2)
			if len(parts) < 2 {
				continue
			}

			prefix := parts[0]
			reference := Reference{
				ID:   ref,
				Path: filePath,
			}

			switch prefix {
			case "var":
				refs["variable"] = appendUniqueReference(refs["variable"], reference)
			case "module":
				refs["module"] = appendUniqueReference(refs["module"], reference)
			case "data":
				refs["data"] = appendUniqueReference(refs["data"], reference)
			default:
				refs["resource"] = appendUniqueReference(refs["resource"], reference)
			}
		}
	}

	return refs
}

func traversalToReference(traversal hcl.Traversal) string {
	var parts []string
	for _, traverser := range traversal {
		switch t := traverser.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, t.Name)
		case hcl.TraverseAttr:
			parts = append(parts, t.Name)
		case hcl.TraverseIndex:
			if t.Key.Type().FriendlyName() == "string" {
				parts = append(parts, t.Key.AsString())
			}
		}
	}
	return strings.Join(parts, ".")
}

func appendUniqueReference(slice []Reference, element Reference) []Reference {
	for _, existing := range slice {
		if existing.ID == element.ID {
			return slice
		}
	}
	return append(slice, element)
}

func generateDOT(graph Graph) string {
	var builder strings.Builder
	builder.WriteString("digraph terraform {\n")
	builder.WriteString("  rankdir = LR;\n")
	builder.WriteString("  compound = true;\n\n")

	// Node style
	builder.WriteString("  // Node styles\n")
	builder.WriteString("  node [shape=box, style=rounded];\n\n")

	// Color schema
	colors := map[string]string{
		"variable": "#FFB6C1", // Light pink
		"module":   "#98FB98", // Pale green
		"data":     "#87CEEB", // Sky blue
		"resource": "#DDA0DD", // Plum
	}

	// Main resource node
	for id, node := range graph.Nodes {
		if node.Depth == 0 {
			builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", color=red];\n", id, id, node.Path))
		}
	}

	builder.WriteString("\n  // Referenced nodes\n")
	// Append referenced nodes
	for _, node := range graph.Nodes {
		if node.Depth == 0 {
			// variable ref
			for _, ref := range node.References["variable"] {
				builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
					ref.ID, ref.ID, ref.Path, colors["variable"]))
			}
			// module ref
			for _, ref := range node.References["module"] {
				builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
					ref.ID, ref.ID, ref.Path, colors["module"]))
			}
			// data ref
			for _, ref := range node.References["data"] {
				builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
					ref.ID, ref.ID, ref.Path, colors["data"]))
			}
			// resource ref
			for _, ref := range node.References["resource"] {
				builder.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
					ref.ID, ref.ID, ref.Path, colors["resource"]))
			}
		}
	}

	builder.WriteString("\n  // Dependencies\n")
	// Append edges with solid lines only
	for _, node := range graph.Nodes {
		if node.Depth == 0 {
			// All references now use solid lines to avoid the dashdot warning
			for _, refs := range node.References {
				for _, ref := range refs {
					builder.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [style=solid];\n", ref.ID, node.ID))
				}
			}
		}
	}

	builder.WriteString("}\n")
	return builder.String()
}
