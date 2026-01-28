package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiCommands "github.com/cerebriumai/cerebrium/internal/ui/commands"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

const (
	// runPollingTimeout is the maximum time to wait for run completion
	runPollingTimeout = 15 * time.Minute
)

// NewRunCmd creates a run command
func NewRunCmd() *cobra.Command {
	var (
		data   string
		region string
	)

	cmd := &cobra.Command{
		Use:   "run <filename> [flags]",
		Short: "Run a file in the current project context",
		Long: `Run a given file in the current project context.

This command packages the current directory into a tar file and uploads it to Cerebrium.
Cerebrium will then execute the specified entry file.

If a cerebrium.toml file is present, it will be used to configure the app.
If no app name is provided, the current directory name will be used as the app name.
If dependencies are specified in the cerebrium.toml file, a base image will be created and used for the run.

Examples:
  cerebrium run main.py
  cerebrium run main.py --data '{"input": "test"}'
  cerebrium run main.py::run --prompt "Hello World"
  cerebrium run script.py::process_data --region us-east-1
  cerebrium run app.py --key1 value1 --key2 value2

Note: You can pass custom arguments to your function using --key value format.`,
		// Allow unknown flags to be passed through as function arguments
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "Error: No filename provided\n\n")
				fmt.Fprintf(os.Stderr, "Usage: cerebrium run <filename> [flags]\n\n")
				fmt.Fprintf(os.Stderr, "Run 'cerebrium run --help' for more information.\n\n")
				err := ui.NewValidationError(fmt.Errorf("filename is required"))
				err.SilentExit = true
				return err
			}

			dataMap, err := parseDataArguments(data, args[1:])
			if err != nil {
				return err
			}

			return runRun(cmd, args[0], dataMap, region)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "JSON data to pass to the app")
	cmd.Flags().StringVarP(&region, "region", "r", "", "Region for the app execution")

	// Stop flag parsing after the first positional argument (filename)
	// This allows passing through --key value arguments for the function
	cmd.Flags().SetInterspersed(false)

	return cmd
}

func runRun(cmd *cobra.Command, filename string, dataMap map[string]any, region string) error {
	cmd.SilenceUsage = true

	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get display options: %w", err))
	}

	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	if cfg.ProjectID == "" {
		return ui.NewValidationError(fmt.Errorf("no project configured. Please run 'cerebrium login' first"))
	}

	parsedFilename, functionName, err := parseFilenameAndFunctionName(filename)
	if err != nil {
		return err
	}
	filename = parsedFilename

	exists, err := fileExists(filename)
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to check file: %w", err))
	}
	if !exists {
		return ui.NewValidationError(fmt.Errorf("file not found: %s", filename))
	}

	projectConfig, err := loadProjectConfigOrDefault()
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to load project config: %w", err))
	}

	if len(projectConfig.Deployment.Name) > 30 {
		return ui.NewValidationError(fmt.Errorf("app name '%s' is too long. App names must be 30 characters or less", projectConfig.Deployment.Name))
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Create context with timeout for run polling
	ctx, cancel := context.WithTimeout(cmd.Context(), runPollingTimeout)
	defer cancel()

	model := uiCommands.NewRunView(ctx, uiCommands.RunConfig{
		DisplayConfig: displayOpts,
		Config:        projectConfig,
		ProjectID:     cfg.ProjectID,
		Client:        client,
		Filename:      filename,
		FunctionName:  functionName,
		Region:        region,
		DataMap:       dataMap,
	})

	var programOpts []tea.ProgramOption

	if !displayOpts.IsInteractive {
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	}

	p := tea.NewProgram(model, programOpts...)

	ui.SetupSignalHandling(p, 5*time.Second)

	finalModel, err := p.Run()
	if err != nil {
		return ui.NewInternalError(fmt.Errorf("internal error: %w", err))
	}

	m, ok := finalModel.(*uiCommands.RunView)
	if !ok {
		return ui.NewInternalError(fmt.Errorf("unexpected model type"))
	}

	if uiErr := m.GetError(); uiErr != nil {
		if uiErr.SilentExit {
			return nil
		}
		return uiErr
	}

	return nil
}

func parseDataArguments(dataFlag string, extraArgs []string) (map[string]any, error) {
	dataMap := make(map[string]any)

	// Case 1: --data JSON string
	if dataFlag != "" {
		if err := json.Unmarshal([]byte(dataFlag), &dataMap); err != nil {
			return nil, ui.NewValidationError(fmt.Errorf("invalid JSON in --data flag: %w", err))
		}
		return dataMap, nil
	}

	// Case 2: extra args like --key value or --key=value
	for i := 0; i < len(extraArgs); i++ {
		arg := extraArgs[i]
		if !strings.HasPrefix(arg, "--") {
			return nil, ui.NewValidationError(fmt.Errorf("invalid argument '%s': use '--key value' or '--key=value'", arg))
		}

		arg = strings.TrimPrefix(arg, "--")

		if key, val, ok := strings.Cut(arg, "="); ok {
			dataMap[key] = val
			continue
		}

		if i+1 >= len(extraArgs) {
			return nil, ui.NewValidationError(fmt.Errorf("missing value for '--%s'", arg))
		}
		i++
		dataMap[arg] = extraArgs[i]
	}

	return dataMap, nil
}

func parseFilenameAndFunctionName(input string) (filename string, functionName *string, err error) {
	filename = input

	if strings.Contains(input, "::") {
		parts := strings.Split(input, "::")
		if len(parts) != 2 {
			return "", nil, ui.NewValidationError(fmt.Errorf("invalid filename format. Use 'filename::function_name'"))
		}
		filename = parts[0]
		funcName := parts[1]
		functionName = &funcName
	}

	if !strings.HasSuffix(filename, ".py") {
		return "", nil, ui.NewValidationError(fmt.Errorf("invalid file type. Expected a Python file (.py), got: %s", filename))
	}

	return filename, functionName, nil
}

func loadProjectConfigOrDefault() (*projectconfig.ProjectConfig, error) {
	projectConfig, err := projectconfig.Load("./cerebrium.toml")
	if err != nil {
		// Check if the error is because the file doesn't exist
		if _, statErr := os.Stat("./cerebrium.toml"); os.IsNotExist(statErr) {
			// File doesn't exist, create default config
			cwd, err := os.Getwd()
			if err != nil {
				cwd = "."
			}
			appName := filepath.Base(cwd)
			if appName == "." || appName == "/" || appName == "" {
				appName = "app"
			}

			projectConfig = &projectconfig.ProjectConfig{
				Deployment: projectconfig.DeploymentConfig{
					Name:    appName,
					Include: []string{"*"},
					Exclude: []string{".git/*", "*.pyc", "__pycache__/*", ".DS_Store", "*.swp", "*.swo", "venv", "node_modules/*"},
				},
			}
		} else {
			// File exists but has parsing errors
			return nil, fmt.Errorf("failed to parse cerebrium.toml: %w", err)
		}
	}

	// Print deprecation warnings if any
	for _, warning := range projectConfig.DeprecationWarnings {
		fmt.Printf("⚠️  Deprecation warning: %s\n", warning)
	}

	return projectConfig, nil
}

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}
