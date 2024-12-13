package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func findModuleNode(basePath string, moduleID string) (string, Node) {
	fmt.Printf("Looking for module: %s in %s\n", moduleID, basePath)

	files := findTerraformFiles(basePath)
	parser := hclparse.NewParser()
	moduleName := strings.TrimPrefix(moduleID, "module.")

	// Split for nested module attributes (e.g., module.name.email)
	if parts := strings.SplitN(moduleName, ".", 2); len(parts) > 0 {
		moduleName = parts[0]
	}

	fmt.Printf("Looking for module with name: %s\n", moduleName)

	for _, file := range files {
		f, diags := parser.ParseHCLFile(file)
		if diags.HasErrors() {
			continue
		}

		content, diags := f.Body.Content(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "module", LabelNames: []string{"name"}},
			},
		})
		if diags.HasErrors() {
			continue
		}

		for _, block := range content.Blocks {
			if block.Type == "module" && block.Labels[0] == moduleName {
				attrs, _ := block.Body.JustAttributes()
				if sourceAttr, exists := attrs["source"]; exists {
					sourceVal, diags := sourceAttr.Expr.Value(nil)
					if !diags.HasErrors() {
						sourcePath := sourceVal.AsString()
						if strings.HasPrefix(sourcePath, ".") {
							// -path で指定されたディレクトリを基準に相対パスを解決
							absolutePath := filepath.Join(basePath, sourcePath)
							fmt.Printf("Module source path: %s (absolute: %s)\n", sourcePath, absolutePath)

							// Verify that module directory exists
							if _, err := os.Stat(absolutePath); err == nil {
								refs := extractReferences(block, file)
								return absolutePath, Node{
									ID:         moduleID,
									Type:       "module",
									Name:       moduleName,
									Path:       file,
									References: refs,
								}
							} else {
								fmt.Printf("Module directory does not exist: %s (error: %v)\n", absolutePath, err)
							}
						} else {
							fmt.Printf("Non-relative module path: %s\n", sourcePath)
						}
					}
				}
			}
		}
	}

	fmt.Printf("Module not found: %s\n", moduleName)
	return "", Node{}
}

func traverseModuleDirectory(graph *Graph, moduleBasePath string, moduleID string, depth int, processedNodes map[string]bool) {
	fmt.Printf("Traversing module directory: %s\n", moduleBasePath)
	files := findTerraformFiles(moduleBasePath)
	if len(files) == 0 {
		fmt.Printf("No Terraform files found in module directory: %s\n", moduleBasePath)
		return
	}

	parser := hclparse.NewParser()
	for _, file := range files {
		fmt.Printf("Parsing module file: %s\n", file)
		f, diags := parser.ParseHCLFile(file)
		if diags.HasErrors() {
			fmt.Printf("Error parsing module file: %s - %s\n", file, diags.Error())
			continue
		}

		content, diags := f.Body.Content(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "resource", LabelNames: []string{"type", "name"}},
				{Type: "data", LabelNames: []string{"type", "name"}},
				{Type: "module", LabelNames: []string{"name"}},
				{Type: "output", LabelNames: []string{"name"}},
			},
		})
		if diags.HasErrors() {
			fmt.Printf("Error getting content from module file: %s - %s\n", file, diags.Error())
			continue
		}

		processModuleBlocks(graph, content.Blocks, file, moduleID, depth, processedNodes, moduleBasePath)
	}
}

func processModuleBlocks(graph *Graph, blocks hcl.Blocks, file, moduleID string, depth int, processedNodes map[string]bool, moduleBasePath string) {
	for _, block := range blocks {
		currentID := getBlockID(block)
		if currentID == "" {
			continue
		}

		refs := extractReferences(block, file)
		moduleResourceNode := Node{
			ID:         fmt.Sprintf("%s/%s", moduleID, currentID),
			Type:       block.Type,
			Name:       block.Labels[len(block.Labels)-1],
			Path:       file,
			References: refs,
			Depth:      depth + 1,
		}
		graph.Nodes[moduleResourceNode.ID] = moduleResourceNode

		processModuleReferences(graph, refs, moduleID, depth, processedNodes, moduleBasePath)
	}
}

func processModuleReferences(graph *Graph, refs map[string][]Reference, moduleID string, depth int, processedNodes map[string]bool, moduleBasePath string) {
	for _, refList := range refs {
		for _, ref := range refList {
			if !strings.HasPrefix(ref.ID, "var.") && !strings.HasPrefix(ref.ID, "local.") {
				moduleRef := fmt.Sprintf("%s/%s", moduleID, ref.ID)
				if !processedNodes[moduleRef] {
					traverseNode(graph, moduleBasePath, moduleRef, depth+1, processedNodes)
				}
			}
		}
	}
}
