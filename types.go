package main

// Node represents a Terraform resource, module, or data source
type Node struct {
	ID         string
	Type       string
	Name       string
	Path       string
	References map[string][]Reference // Changed back to array of references
	Depth      int
}

// Reference stores both the reference ID and its file path
type Reference struct {
	ID   string
	Path string
}

// Graph represents the dependency graph of Terraform resources
type Graph struct {
	Nodes    map[string]Node
	MaxDepth int
}
