# CLAUDE.md - Cerebrium Go CLI

## Quick Start
```bash
make build && export CEREBRIUM_ENV=dev && ./bin/cerebrium deploy
```

## Architecture
```
cerebrium/
├── cmd/cerebrium/          # Entry point (thin layer)
├── internal/
│   ├── commands/          # Cobra commands (validation only)
│   ├── ui/commands/       # Bubbletea models (ALL heavy work)
│   ├── api/              # API client
│   └── auth/             # OAuth, tokens
└── pkg/
    ├── config/           # CLI config (~/.cerebrium/config.yaml)
    ├── projectconfig/    # Project config (cerebrium.toml)
    └── logrium/          # Cerebrium-specific logging utility/setup (slog)
```

## Critical Patterns

### 1. Heavy Work in Bubbletea
**Cobra:** Config, validation, create model → **Bubbletea:** ALL API calls, file ops, state management

### 2. State Machine (Elm Architecture)
- State changes ONLY in `Update()`
- `View()` is pure - reads state only
- Async commands trigger state transitions

```go
func (m *MyView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case dataLoadedMsg:
        m.state = StateProcessing  // State change
        return m, m.processData     // Next command
    }
}

func (m *MyView) View() string {
    // Pure - no state changes, only reads
    if m.state == StateLoading {
        return "Loading..."
    }
    return "✓ Loaded"
}
```

### 3. Non-TTY Support (REQUIRED)
ALL commands must support both TTY and non-TTY modes.

```go
// In Update() - direct output for non-TTY
if m.opts.SimpleOutput() {
    fmt.Printf("✓ Done\n")  // Direct print
}

// In View() - return empty for non-TTY
func (m *MyView) View() string {
    if m.opts.SimpleOutput() {
        return ""  // Already printed in Update()
    }
    // ... render UI for TTY mode
}

// Setup in command
displayOpts, err := ui.NewDisplayOptions(cmd)
model := uiCommands.NewMyView(uiCommands.MyOptions{
    DisplayOptions: displayOpts,
})

var programOpts []tea.ProgramOption
if !displayOpts.IsInteractive {
    programOpts = append(programOpts,
        tea.WithoutRenderer(),
        tea.WithInput(nil),
    )
}

p := tea.NewProgram(&model, programOpts...)
ui.SetupSignalHandling(p)  // REQUIRED for signal handling
```

### 4. Config & Context
```go
// ✅ ALWAYS get config from context (loaded once in root.go)
cfg, err := config.GetConfigFromContext(cmd)

// ✅ ALWAYS use cmd.Context()
model := NewMyView(cmd.Context(), opts)

// ❌ NEVER
cfg, err := config.Load()  // Reloads file
ctx := context.Background()  // Loses context
```

### 5. Logging
**User output:** `fmt.Printf()` → stdout
**Debug logs:** `slog.Info/Debug/Error()` → stderr/file

```go
fmt.Printf("✓ Deployment successful\n")  // User sees this
slog.Info("Deployment", "id", id, "files", 42)  // Debug info
```

Interactive mode logs to `/tmp/cerebrium-debug-*.log` to avoid TUI corruption.

### 6. Testing
```go
// Bubbletea models - use test harness
harness := uitesting.NewTestHarness(t, model)
harness.
    Step(uitesting.TestStep[*MyModel]{
        Name:       "initial",
        ViewGolden: "my_model_initial",
        ModelAssert: func(t *testing.T, m *MyModel) {
            assert.Equal(t, StateLoading, m.state)
        },
    }).
    Run(t)

// Context in tests
ctx := t.Context()  // ✅ Auto-cancelled on test completion

// Logging in tests
var buf bytes.Buffer
logrium.SetupForTesting(t, &buf, slog.LevelDebug)
// ... run code ...
assert.Contains(t, buf.String(), "expected log")
```

**Golden files:** Never run `go test -update` - only humans should update golden files.

### 7. Error Types
```go
ui.NewUserCancelledError()     // Ctrl+C (silent)
ui.NewValidationError(err)     // Pre-flight
ui.NewAPIError(err)            // API failures
ui.NewFileSystemError(err)     // File ops
ui.NewInternalError(err)       // Unexpected
```

## Command Template

### Simple Command
```go
func runMyCommand(cmd *cobra.Command, args []string) error {
    cmd.SilenceUsage = true

    cfg, err := config.GetConfigFromContext(cmd)
    if err != nil {
        return ui.NewValidationError(err)
    }

    client, err := api.NewClient(cfg)
    // ... do work ...

    fmt.Printf("Result: %v\n", result)
    return nil
}
```

### Interactive Command
```go
// Cobra command - validation only
func runMyCommand(cmd *cobra.Command, args []string) error {
    cmd.SilenceUsage = true

    displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
    cfg, err := config.GetConfigFromContext(cmd)
    client, err := api.NewClient(cfg)

    model := uiCommands.NewMyView(uiCommands.MyConfig{
        DisplayOptions: displayOpts,
        Client: client,
    })

    var programOpts []tea.ProgramOption
    if !displayOpts.IsInteractive {
        programOpts = append(programOpts,
            tea.WithoutRenderer(),
            tea.WithInput(nil),
        )
    }

    p := tea.NewProgram(&model, programOpts...)
    ui.SetupSignalHandling(p)  // REQUIRED

    finalModel, err := p.Run()
    // ... handle errors ...
}

// Bubbletea model - all heavy work
type myState int
const (
	myStateOne = iota
	myStateTwo
)
type MyView struct {
	ctx     context.Context
    conf    MyConfig
    state   myState
    spinner ui.SpinnerModel
    err     *ui.UIError
}

func (m *MyView) Init() tea.Cmd {
    return tea.Batch(m.spinner.Init(), m.loadData)
}

func (m *MyView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ui.SignalCancelMsg:
        return m.onCancel(msg)
    case dataLoadedMsg:
        return m.onDataLoaded(msg)
    default:
		return m.onDefault(msg)
    }

    if !m.opts.SimpleOutput() {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    return m, nil
}

func (m *MyView) View() string {
    if m.opts.SimpleOutput() {
        return ""
    }
    // Render UI for TTY mode - use state to determine what to render
	if m.state == myStateOne {}
	if m.state > myStateOne {}

    if m.state < myStateTwo {}
    if m.state == myStateTwo {}
    if m.state > myStateTwo {}
}

// Async command
func (m *MyView) onDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.opts.SimpleOutput() {
        return m, nil
    }
    var cmd tea.Cmd
    var spinnerModel tea.Model
    spinnerModel, cmd = m.spinner.Update(msg)
    m.spinner = spinnerModel.(*ui.SpinnerModel)
    return m, cmd
}

func (m *MyView) loadData() tea.Msg {
    result, err := m.opts.Client.FetchData()  // API call
    if err != nil {
        return ui.NewAPIError(err)
    }
    return dataLoadedMsg{data: result}
}

func (m *MyView) Error() error {
	return m.err
}
```

## Quick Reference
- **Config:** `cfg, err := config.GetConfigFromContext(cmd)`
- **Client:** `client, err := api.NewClient(cfg)`
- **Display:** `displayOpts, err := ui.GetDisplayOptionsFromContext(cmd)`
- **Project:** `projectConfig, err := projectconfig.Load("./cerebrium.toml")`
- **Spinner:** `spinner := ui.NewSpinner()`
- **Styles:** `ui.SuccessStyle`, `ui.ErrorStyle`, `ui.ActiveStyle`

## Environment
- `CEREBRIUM_ENV` → prod/dev/local
- Config keys prefixed: `dev-project`, `dev-accesstoken`

## Rules
1. Bubbletea does ALL heavy work, Cobra only validates
2. State changes ONLY in Update(), View() is pure
3. Always support SimpleOutput() for non-TTY
4. Always call ui.SetupSignalHandling(p)
5. Use cmd.Context() and GetConfigFromContext()
6. Never update golden files (humans only)
7. Print SimpleOutput user output with fmt.Printf, debug with slog