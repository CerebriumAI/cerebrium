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
	// Pass deprecated Paths.Pip as fallback for backwards compatibility
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
// Supports both:
// 1. _file_relative_path key in the deps map (recommended)
// 2. deprecatedFilePath from [dependencies.paths] (backwards compatible)
// If both are specified, _file_relative_path takes precedence
// The file is read as a base and inline packages are merged on top
func generateDependencyFile(files map[string]string, fileName string, deps map[string]string, deprecatedFilePath string) error {
	// Get file path - new _file_relative_path key takes precedence over deprecated Paths
	filePath := projectconfig.GetFilePath(deps)
	if filePath == "" {
		filePath = deprecatedFilePath // Fall back to deprecated [dependencies.paths] if new key not set
	}
	packages := projectconfig.GetPackages(deps)

	// Start with base dependencies from file (if specified)
	baseDeps := make(map[string]string)
	if filePath != "" {
		// Check if the file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("the specified file '%s' was not found", filePath)
		}

		// Read and parse the file content
		content, err := os.ReadFile(filePath) //nolint:gosec // File path from user's project configuration
		if err != nil {
			return fmt.Errorf("failed to read file '%s': %w", filePath, err)
		}

		// Parse the file into a dependency map
		baseDeps = parseRequirementsFile(string(content))
	}

	// Merge inline packages on top (inline wins per-package)
	for pkg, ver := range packages {
		baseDeps[pkg] = ver
	}

	// Generate the output if we have any dependencies
	if len(baseDeps) > 0 {
		files[fileName] = generateRequirementsContent(baseDeps)
	}

	return nil
}

// parseRequirementsFile parses a requirements.txt style file into a dependency map
func parseRequirementsFile(content string) map[string]string {
	deps := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle various formats: pkg==1.0, pkg>=1.0, pkg, git+https://...
		if strings.HasPrefix(line, "git+") || strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			// URL-based dependency - use the whole line as the key
			deps[line] = ""
		} else if idx := strings.IndexAny(line, "=<>!~"); idx != -1 {
			// Package with version specifier
			pkg := line[:idx]
			ver := line[idx:]
			deps[pkg] = ver
		} else {
			// Package without version
			deps[line] = ""
		}
	}

	return deps
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

// ParseRequirementsContent parses requirements.txt content into a map of package -> version.
// This is the inverse of generateRequirementsContent.
func ParseRequirementsContent(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue // Skip empty lines, comments, and pip flags (-r, -e, etc.)
		}
		pkg, ver := parseDepLine(line)
		if pkg != "" {
			// Backend requires non-empty version string; use "latest" for packages without version
			if ver == "" {
				ver = "latest"
			}
			result[pkg] = ver
		}
	}
	return result
}

// parseDepLine parses a single dependency line from a requirements file.
// Returns (package_name, version_specifier) where version_specifier may be empty.
// Handles: package, package==1.0, package>=1.0, package[extra], package[extra]>=1.0
func parseDepLine(line string) (string, string) {
	// Handle extras: package[extra] -> extract package name
	pkgName := line
	extraSuffix := ""
	if bracketIdx := strings.Index(line, "["); bracketIdx > 0 {
		if closeBracket := strings.Index(line, "]"); closeBracket > bracketIdx {
			pkgName = line[:bracketIdx]
			extraSuffix = line[bracketIdx : closeBracket+1]
			line = pkgName + line[closeBracket+1:] // Remove extras for version parsing
		}
	}

	// Handle version specifiers: ==, >=, <=, ~=, !=, >, <
	for _, sep := range []string{"==", ">=", "<=", "~=", "!=", ">", "<"} {
		if idx := strings.Index(line, sep); idx > 0 {
			pkg := strings.TrimSpace(line[:idx])
			ver := sep + strings.TrimSpace(line[idx+len(sep):])
			return pkg + extraSuffix, ver
		}
	}
	// No version specifier, just package name (possibly with extras)
	return strings.TrimSpace(pkgName) + extraSuffix, ""
}
