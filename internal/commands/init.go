package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/spf13/cobra"
)

const exampleMain = `def run(prompt: str):
    print(f"Running on Cerebrium: {prompt}")

    return {"my_result": prompt}

# To run your app, run:
# cerebrium run main.py::run --prompt "Hello World"
#
# To deploy your app, run:
# cerebrium deploy
`

const exampleRequirements = `# Add your Python dependencies here
# Example:
# numpy==1.24.0
# requests>=2.28.0
`

// NewInitCmd creates a new init command
func NewInitCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize an empty Cerebrium Cortex project",
		Long: `Initialize an empty Cerebrium Cortex project with default configuration.

This command will:
1. Create a new directory with the specified name
2. Generate a main.py file with example code
3. Create a cerebrium.toml configuration file with sensible defaults

Example:
  cerebrium init my-app
  cerebrium init my-app --dir /path/to/parent
  cerebrium init my-app --dir ./projects`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, args[0], dir)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "./", "Directory to create the Cortex deployment")

	return cmd
}

// validateProjectName validates the project name to prevent path traversal and other security issues
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Check for relative path components first (more specific)
	if name == "." || name == ".." {
		return fmt.Errorf("project name cannot be '.' or '..'")
	}

	// Check for absolute paths (before checking separators)
	if filepath.IsAbs(name) {
		return fmt.Errorf("project name cannot be an absolute path - use --dir to initialise in a specific directory")
	}

	// Check for path separators
	if strings.Contains(name, string(filepath.Separator)) || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("project name cannot contain path separators")
	}

	// Check for reserved names on Windows
	reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	upperName := strings.ToUpper(name)
	for _, r := range reserved {
		if upperName == r {
			return fmt.Errorf("project name '%s' is a reserved name on Windows", name)
		}
	}

	// Check for null bytes
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("project name cannot contain null bytes")
	}

	return nil
}

func runInit(cmd *cobra.Command, name string, dir string) error {
	cmd.SilenceUsage = true

	// Validate project name to prevent path traversal attacks
	if err := validateProjectName(name); err != nil {
		return ui.NewValidationError(err)
	}

	// Determine paths
	projectPath := filepath.Join(dir, name)
	tomlPath := filepath.Join(projectPath, "cerebrium.toml")
	mainPath := filepath.Join(projectPath, "main.py")
	requirementsPath := filepath.Join(projectPath, "requirements.txt")

	// Verify the resulting path is safe (no path traversal)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("invalid directory path: %w", err))
	}
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("invalid project path: %w", err))
	}
	// Ensure the project path is a subdirectory of the target directory
	if !strings.HasPrefix(absProjectPath, absDir+string(filepath.Separator)) {
		return ui.NewValidationError(fmt.Errorf("invalid project name: path traversal detected"))
	}

	// Print initialization message
	if dir != "./" {
		fmt.Printf("Initializing Cerebrium Cortex project in new directory %s\n", name)
	} else {
		fmt.Printf("Initializing Cerebrium Cortex project in directory %s\n", projectPath)
	}

	// Check if directory already exists
	if _, err := os.Stat(projectPath); err == nil {
		return ui.NewValidationError(fmt.Errorf("directory already exists. Please choose a different name"))
	} else if !os.IsNotExist(err) {
		return ui.NewFileSystemError(fmt.Errorf("failed to check directory: %w", err))
	}

	// Create directory
	if err := os.MkdirAll(projectPath, 0755); err != nil { //nolint:gosec // Project directory needs standard permissions
		return ui.NewFileSystemError(fmt.Errorf("failed to create directory: %w", err))
	}

	// Create main.py file
	if err := os.WriteFile(mainPath, []byte(exampleMain), 0644); err != nil { //nolint:gosec // Project files need to be readable
		return ui.NewFileSystemError(fmt.Errorf("failed to create main.py: %w", err))
	}

	// Create requirements.txt file
	if err := os.WriteFile(requirementsPath, []byte(exampleRequirements), 0644); err != nil { //nolint:gosec // Project files need to be readable
		return ui.NewFileSystemError(fmt.Errorf("failed to create requirements.txt: %w", err))
	}

	// Create cerebrium.toml with sensible defaults
	if err := createDefaultConfig(tomlPath, name); err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to create cerebrium.toml: %w", err))
	}

	fmt.Println("Cerebrium Cortex project initialized successfully!")
	fmt.Printf("cd %s && cerebrium deploy to get started\n", projectPath)

	return nil
}

// createDefaultConfig creates a cerebrium.toml file with sensible defaults
func createDefaultConfig(path string, name string) error {
	// Manually construct TOML to match Python output exactly
	// Using double quotes for strings and avoiding empty [cerebrium.dependencies] section
	content := fmt.Sprintf(`[cerebrium.deployment]
name = "%s"
python_version = "3.11"
docker_base_image_url = "debian:bookworm-slim"
disable_auth = true
include = ['./*', 'main.py', 'cerebrium.toml']
exclude = ['.*']

[cerebrium.hardware]
cpu = 2.0
memory = 2.0
compute = "CPU"
provider = "aws"
region = "us-east-1"

[cerebrium.scaling]
min_replicas = 0
max_replicas = 2
cooldown = 30
replica_concurrency = 1
scaling_metric = "concurrency_utilization"

[cerebrium.dependencies.paths]
pip = "requirements.txt"
`, name)

	// Write to file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil { //nolint:gosec // Config file needs to be readable by tools
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
