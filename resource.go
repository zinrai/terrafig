package main

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func findResourceNode(basePath string, nodeID string) (Node, bool) {
	fmt.Printf("Searching for nodeID: %s in %s\n", nodeID, basePath)

	files := findTerraformFiles(basePath)
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

		if node, found := findNodeInBlocks(content.Blocks, nodeID, file); found {
			return node, true
		}
	}

	fmt.Printf("Resource not found: %s\n", nodeID)
	return Node{}, false
}

func findNodeInBlocks(blocks hcl.Blocks, nodeID string, file string) (Node, bool) {
	for _, block := range blocks {
		currentID := getBlockID(block)
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
	return Node{}, false
}

func getBlockID(block *hcl.Block) string {
	switch block.Type {
	case "resource":
		return fmt.Sprintf("%s.%s", block.Labels[0], block.Labels[1])
	case "module":
		return fmt.Sprintf("module.%s", block.Labels[0])
	case "data":
		return fmt.Sprintf("data.%s.%s", block.Labels[0], block.Labels[1])
	case "output":
		return fmt.Sprintf("output.%s", block.Labels[0])
	default:
		return ""
	}
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

func appendUniqueReference(slice []Reference, element Reference) []Reference {
	for _, existing := range slice {
		if existing.ID == element.ID {
			return slice
		}
	}
	return append(slice, element)
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
