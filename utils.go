package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findTerraformFiles returns a list of .tf files in the given directory
func findTerraformFiles(basePath string) []string {
	var files []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return err
		}
		if strings.HasSuffix(info.Name(), ".tf") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %q: %v\n", basePath, err)
		return []string{}
	}

	return files
}

// resolvePath resolves a relative path against a base path
func resolvePath(basePath, relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Clean(filepath.Join(basePath, relativePath))
}
