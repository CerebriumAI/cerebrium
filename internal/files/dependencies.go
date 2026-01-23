package files

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
)

// GenerateDependencyFiles creates the content for pip, conda, and apt requirement files
func GenerateDependencyFiles(config *projectconfig.ProjectConfig) (map[string]string, error) {
	files := make(map[string]string)

	// Get effective dependencies (merged from top-level and runtime-specific)
	effectiveDeps := config.GetEffectiveDependencies()

	// Generate pip requirements
	if err := generateDependencyFile(files, "requirements.txt", effectiveDeps.Pip, effectiveDeps.Paths.Pip); err != nil {
		return nil, fmt.Errorf("failed to generate pip requirements: %w", err)
	}

	// Generate conda requirements
	if err := generateDependencyFile(files, "conda_pkglist.txt", effectiveDeps.Conda, effectiveDeps.Paths.Conda); err != nil {
		return nil, fmt.Errorf("failed to generate conda requirements: %w", err)
	}

	// Generate apt requirements
	if err := generateDependencyFile(files, "pkglist.txt", effectiveDeps.Apt, effectiveDeps.Paths.Apt); err != nil {
		return nil, fmt.Errorf("failed to generate apt requirements: %w", err)
	}

	// Generate shell commands file (from runtime or deprecated deployment section)
	if shellCmds := config.GetEffectiveShellCommands(); len(shellCmds) > 0 {
		files["shell_commands.sh"] = generateShellCommandsContent(shellCmds)
	}

	// Generate pre-build commands file (from runtime or deprecated deployment section)
	if preBuildCmds := config.GetEffectivePreBuildCommands(); len(preBuildCmds) > 0 {
		files["pre_build_commands.sh"] = generateShellCommandsContent(preBuildCmds)
	}

	return files, nil
}

// generateDependencyFile handles both inline dependencies and file paths
func generateDependencyFile(files map[string]string, fileName string, deps map[string]string, filePath string) error {
	// Check if both are specified
	if len(deps) > 0 && filePath != "" {
		return fmt.Errorf("both %s and dependencies specified in config - please specify only one", fileName)
	}

	// If a file path is specified, read and use that file
	if filePath != "" {
		// Check if the file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("the specified file '%s' was not found", filePath)
		}

		// Read the file content
		content, err := os.ReadFile(filePath) //nolint:gosec // File path from user's project configuration
		if err != nil {
			return fmt.Errorf("failed to read file '%s': %w", filePath, err)
		}

		files[fileName] = string(content)
		return nil
	}

	// Otherwise, generate from inline dependencies
	if len(deps) > 0 {
		files[fileName] = generateRequirementsContent(deps)
	}

	return nil
}

// generateRequirementsContent creates content for a requirements file
func generateRequirementsContent(deps map[string]string) string {
	// Sort package names for deterministic output
	packages := make([]string, 0, len(deps))
	for pkg := range deps {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	var lines []string
	for _, pkg := range packages {
		version := deps[pkg]
		// Treat empty, "*", and "latest" as package without version
		if version == "" || version == "*" || version == "latest" {
			lines = append(lines, pkg)
		} else {
			// Handle version specifiers properly
			if strings.HasPrefix(version, "=") || strings.HasPrefix(version, ">") ||
				strings.HasPrefix(version, "<") || strings.HasPrefix(version, "!") {
				lines = append(lines, fmt.Sprintf("%s%s", pkg, version))
			} else {
				lines = append(lines, fmt.Sprintf("%s==%s", pkg, version))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// generateShellCommandsContent creates content for shell command files
func generateShellCommandsContent(commands []string) string {
	var lines []string
	lines = append(lines, "#!/bin/bash")
	lines = append(lines, "set -e")
	lines = append(lines, "")
	lines = append(lines, commands...)
	return strings.Join(lines, "\n") + "\n"
}
