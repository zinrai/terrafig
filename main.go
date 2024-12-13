package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

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
