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

// agentsTemplate is written to AGENTS.md. Coding agents read this file at the
// start of a session without being asked, which is the only reliable way they
// learn the CLI has commands worth running after a deploy — they do not go
// looking through --help for capabilities they have no reason to expect.
const agentsTemplate = `# %[1]s

A Cerebrium app. ` + "`main.py`" + ` holds the functions that get served; ` + "`cerebrium.toml`" + ` declares
hardware, scaling and dependencies.

## Deploying

    cerebrium deploy

A deploy builds a new image and rolls it out. It takes minutes, not seconds, and
the app is not serving the new code until it finishes.

## Checking a deploy worked

Do not assume a deploy is live because the command exited 0. Verify it:

    cerebrium containers list %[1]s     # what is running right now
    cerebrium logs %[1]s                # runtime logs
    cerebrium runs list %[1]s           # recent invocations and their status
    cerebrium apps get %[1]s            # configured hardware, scaling, build status

Every command above accepts ` + "`--output json`" + ` for machine-readable output. Prefer it
over parsing the table format.

## Right-sizing the hardware

    cerebrium metrics resources %[1]s --since 24h

Reports peak CPU (cores), memory (GB) and GPU memory (GB) over the window, so you
can compare against ` + "`[cerebrium.hardware]`" + ` in cerebrium.toml and adjust. A metric
that reports ` + "`-`" + ` had no samples in the window, which is not the same as zero.

## Things that surprise people

- ` + "`min_replicas = 0`" + ` scales the app to zero when idle, so the first request after a
  quiet period pays a cold start. ` + "`cerebrium containers list`" + ` returning nothing is
  normal for an idle app, not a failed deploy.
- Secrets belong in ` + "`cerebrium secrets`" + `, never in cerebrium.toml — that file is
  committed and uploaded with the build.
- Changing ` + "`[cerebrium.hardware]`" + ` or dependencies requires a redeploy to take effect.
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
	agentsPath := filepath.Join(projectPath, "AGENTS.md")

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

	// Create cerebrium.toml with sensible defaults
	if err := createDefaultConfig(tomlPath, name); err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to create cerebrium.toml: %w", err))
	}

	// Create AGENTS.md so coding agents working in this project know how to
	// verify a deploy and size the hardware
	agentsContent := fmt.Sprintf(agentsTemplate, name)
	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil { //nolint:gosec // Project files need to be readable
		return ui.NewFileSystemError(fmt.Errorf("failed to create AGENTS.md: %w", err))
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
region = "us-east-1"

[cerebrium.scaling]
min_replicas = 0
max_replicas = 2
cooldown = 30
replica_concurrency = 1
scaling_metric = "concurrency_utilization"

[cerebrium.dependencies.pip]
numpy = "latest"
`, name)

	// Write to file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil { //nolint:gosec // Config file needs to be readable by tools
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
