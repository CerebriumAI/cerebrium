# Bubbletea Testing Harness

A comprehensive test harness for testing Bubbletea components with support for golden file testing, model state assertions, and step-based test flows.

## Quick Start

```go
package mypackage

//go:generate go test -v -run TestMyModel -update

import (
    "testing"
    uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
    "github.com/stretchr/testify/assert"
)

func TestMyModel(t *testing.T) {
    model := NewMyModel()

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[MyModel]{
            Name:       "initial_state",
            ViewGolden: "my_model_initial",
        }).
        Step(uitesting.TestStep[MyModel]{
            Name: "after_message",
            Msg:  myMsg{data: "test"},
            ViewGolden: "my_model_after_message",
            ModelAssert: func(t *testing.T, m MyModel) {
                // No type assertion needed - m is already MyModel!
                assert.Equal(t, "test", m.data)
            },
        }).
        Run(t)
}
```

## Architecture

The test harness follows a **step-based testing pattern**:

1. **Initialize**: Calls `model.Init()` and processes any returned commands
2. **Execute Steps**: For each test step:
   - Sends the configured `tea.Msg` to `Update()`
   - Processes any commands returned (simulating the Bubbletea runtime loop)
   - Asserts `View()` output using golden files or custom assertions
   - Asserts model state using custom assertions

### Key Features

- **Golden File Testing**: Uses [Goldie](https://github.com/sebdah/goldie) for snapshot testing of View() output
- **Command Processing**: Automatically executes commands and feeds messages back to Update()
- **Consistent Environment**: Forces ASCII color profile to prevent flaky tests across environments
- **Method Chaining**: Fluent API for building test sequences
- **Flexible Assertions**: Supports both golden files and custom assertion functions

## TestStep Configuration

```go
type TestStep[T tea.Model] struct {
    // Name identifies this step (used in error messages and golden file names)
    Name string

    // Msg to send to Update(). If nil, only View() is called (useful for testing initial state)
    Msg tea.Msg

    // ViewGolden is the golden file name for View() output.
    // File will be stored at: testdata/<ViewGolden>.golden
    ViewGolden string

    // ViewAssert is a custom assertion for View() output.
    ViewAssert func(t *testing.T, view string)

    // ModelAssert is a custom assertion for model state.
    // T is your concrete model type - no type assertion needed!
    ModelAssert func(t *testing.T, m T)

    // SkipViewAssertion skips View() assertions for this step.
    SkipViewAssertion bool
}
```

## Usage Examples

### Example 1: Testing Initial State

```go
func TestInitialState(t *testing.T) {
    model := NewDeployView(DeployConfig{})

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[*DeployView]{
            Name:       "initial",
            Msg:        nil,  // No message, just test initial render
            ViewGolden: "deploy_initial",
            ModelAssert: func(t *testing.T, m *DeployView) {
                // m is already *DeployView - no type assertion!
                assert.Equal(t, StateLoadingFiles, m.state)
            },
        }).
        Run(t)
}
```

### Example 2: Testing State Transitions

```go
func TestStateTransitions(t *testing.T) {
    model := NewDeployView(opts)

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[*DeployView]{
            Name:       "loading_files",
            Msg:        filesLoadedMsg{fileList: []string{"main.go"}},
            ViewGolden: "deploy_loading_files",
            ModelAssert: func(t *testing.T, m *DeployView) {
                assert.Equal(t, StateZippingFiles, m.state)
                assert.Len(t, m.fileList, 1)
            },
        }).
        Step(uitesting.TestStep[*DeployView]{
            Name:       "zipping_files",
            Msg:        filesZippedMsg{zipPath: "/tmp/test.zip", zipSize: 1024},
            ViewGolden: "deploy_zipping_files",
            ModelAssert: func(t *testing.T, m *DeployView) {
                assert.Equal(t, StateCreatingApp, m.state)
                assert.Equal(t, int64(1024), m.zipSize)
            },
        }).
        Run(t)
}
```

### Example 3: Testing Error Handling

```go
func TestErrorHandling(t *testing.T) {
    model := NewLoginView(cfg, opts)

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[LoginView]{
            Name: "api_error",
            Msg:  ui.NewAPIError(fmt.Errorf("connection failed")),
            ViewAssert: func(t *testing.T, view string) {
                uitesting.AssertContains(t, view, "connection failed")
                uitesting.AssertContains(t, view, "Error")
            },
            ModelAssert: func(t *testing.T, m LoginView) {
                assert.Equal(t, StateError, m.state)
                assert.NotNil(t, m.err)
            },
        }).
        Run(t)
}
```

### Example 4: Testing Keyboard Input

```go
func TestKeyboardInput(t *testing.T) {
    model := NewDeployView(opts)

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[*DeployView]{
            Name: "ctrl_c_cancellation",
            Msg: tea.KeyMsg{
                Type:  tea.KeyCtrlC,
                Runes: []rune("c"),
            },
            ModelAssert: func(t *testing.T, m *DeployView) {
                assert.NotNil(t, m.err)
                // Check if it's a user cancelled error
                assert.True(t, m.err.SilentExit)
            },
        }).
        Run(t)
}
```

### Example 5: Custom View Assertions

```go
func TestCustomViewAssertions(t *testing.T) {
    model := NewLogViewer(config)

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[*LogViewer]{
            Name: "logs_received",
            Msg:  logsReceivedMsg{logs: []string{"line1", "line2"}},
            ViewAssert: func(t *testing.T, view string) {
                // Custom assertions without golden files
                uitesting.AssertContains(t, view, "line1")
                uitesting.AssertContains(t, view, "line2")
                uitesting.AssertNotContains(t, view, "error")

                // Or use testify for more complex assertions
                lines := strings.Split(view, "\n")
                assert.GreaterOrEqual(t, len(lines), 2)
            },
        }).
        Run(t)
}
```

## Golden File Management

### Directory Structure

```
internal/ui/commands/
├── deploy.go
├── deploy_test.go
└── testdata/
    ├── deploy_initial.golden
    ├── deploy_loading_files.golden
    └── deploy_zipping_files.golden
```

### Generating Golden Files

Add a `//go:generate` directive to your test file:

```go
//go:generate go test -v -run TestDeployView -update
```

Then run:

```bash
# Generate golden files for specific test
go generate ./internal/ui/commands/deploy_test.go

# Or run the test directly with -update flag
go test -v -run TestDeployView -update ./internal/ui/commands

# Generate golden files for all tests in a package
go test -update ./internal/ui/commands
```

### Updating Golden Files

```bash
# Update specific test's golden files
go test -v -run TestDeployView -update ./internal/ui/commands

# Update all golden files in a package
go test -update ./internal/ui/commands

# Update all golden files in the project
go test -update ./...
```

## Best Practices

### 1. Test Environment Consistency

The harness automatically forces ASCII color profile to ensure consistent output across different terminals and CI environments. This prevents golden file diffs caused by ANSI escape codes.

### 2. Name Your Steps Clearly

Use descriptive step names that explain what state is being tested:

```go
// ✅ Good
Name: "after_files_loaded"
Name: "error_invalid_token"
Name: "initial_state_no_project"

// ❌ Bad
Name: "step1"
Name: "test"
Name: "a"
```

### 3. Combine Golden Files with State Assertions

Golden files test the UI output, but you should also assert internal model state:

```go
Step(uitesting.TestStep[*DeployView]{
    Name:       "files_loaded",
    Msg:        filesLoadedMsg{...},
    ViewGolden: "files_loaded",  // Tests UI
    ModelAssert: func(t *testing.T, m *DeployView) {
        // Test internal state - m is already *DeployView!
        assert.Equal(t, StateZippingFiles, m.state)
        assert.NotEmpty(t, m.fileList)
    },
})
```

### 4. Test Error Cases

Always test error handling:

```go
Step(uitesting.TestStep[*DeployView]{
    Name: "api_error",
    Msg:  ui.NewAPIError(fmt.Errorf("failed")),
    ViewAssert: func(t *testing.T, view string) {
        uitesting.AssertContains(t, view, "Error")
        uitesting.AssertContains(t, view, "failed")
    },
})
```

### 5. Test Initial State

Always include a step that tests the initial state without sending any messages:

```go
Step(uitesting.TestStep[*DeployView]{
    Name:       "initial",
    Msg:        nil,  // No message
    ViewGolden: "initial",
})
```

### 6. Skip View Assertions When Appropriate

If you only care about model state (e.g., testing a command that doesn't change the view):

```go
Step(uitesting.TestStep[*DeployView]{
    Name:              "internal_state_change",
    Msg:               internalMsg{},
    SkipViewAssertion: true,
    ModelAssert: func(t *testing.T, m *DeployView) {
        // Only test model state
        assert.Equal(t, expectedState, m.state)
    },
})
```

## Helper Assertions

The package provides helper functions for common assertions:

```go
// Assert view contains substring
uitesting.AssertContains(t, view, "expected text")

// Assert view does NOT contain substring
uitesting.AssertNotContains(t, view, "unexpected text")
```

## Testing with Non-TTY Mode

The harness tests the model logic directly and doesn't run a full Bubbletea program. However, you can test both TTY and non-TTY modes by creating models with different `DisplayConfig`:

```go
func TestSimpleOutput(t *testing.T) {
    opts := DeployConfig{
        DisplayConfig: ui.DisplayConfig{
            IsInteractive:    false,
            DisableAnimation: true,
        },
    }
    model := NewDeployView(opts)

    harness := uitesting.NewTestHarness(t, model)
    harness.
        Step(uitesting.TestStep[*DeployView]{
            Name: "simple_mode",
            ViewAssert: func(t *testing.T, view string) {
                // In simple mode, View() should return empty string
                assert.Empty(t, view)
            },
        }).
        Run(t)
}
```

## Debugging Tips

### View Golden File Diffs

When a test fails, Goldie shows a detailed diff:

```
--- Expected
+++ Actual
@@ -1,3 +1,3 @@
-✓  Loaded 5 files
+✓  Loaded 3 files
```

### Print View Output

Add debug prints in your ViewAssert:

```go
ViewAssert: func(t *testing.T, view string) {
    t.Logf("View output:\n%s", view)
    uitesting.AssertContains(t, view, "expected")
}
```

### Print Model State

Add debug prints in your ModelAssert:

```go
ModelAssert: func(t *testing.T, m *DeployView) {
    t.Logf("Model state: %+v", m)
    assert.Equal(t, expectedState, m.state)
}
```

## Advanced: Testing Async Command Chains

### The Challenge

Bubbletea models often trigger chains of async commands:
1. Message triggers state change → returns command A
2. Command A executes → produces message B
3. Message B triggers state change → returns command C
4. And so on...

The harness automatically processes these chains, which can cause problems:
- Commands may call real APIs (unmocked)
- Commands may perform file I/O
- Chains can be very long (hard to mock everything)

### Solution: Expect() and Finally()

The harness provides **interception points** to control async command execution:

#### Finally() - Stop After One Message

Use `Finally()` to send a message and stop before processing the returned command:

```go
harness.
    Finally(uitesting.TestStep[*DeployView]{
        Name:       "files_zipped",
        Msg:        filesZippedMsg{zipPath: "/tmp/test.zip", zipSize: 1024},
        ViewGolden: "after_zip",
        ModelAssert: func(t *testing.T, m *DeployView) {
            assert.Equal(t, StateCreatingApp, m.state)
        },
    }).
    Run(t)
// ← Stops here! The model returns a command `createApp`, but that command is NOT executed
```

**Use case**: Test a state transition without triggering the next async operation.

#### Expect() + Finally() - Intercept Async Results

Use `Expect()` to intercept messages from async commands, then `Finally()` to stop:

```go
harness.
    Step(uitesting.TestStep[*DeployView]{
        Name: "trigger_async",
        Msg:  filesLoadedMsg{fileList: testFiles},
        SkipViewAssertion: true,  // View checked in Finally
    }).
    Finally(uitesting.TestStep[*DeployView]{
        Name:       "async_complete",
        // No Msg! This intercepts filesZippedMsg from zipFiles() command
        ViewGolden: "after_async",
        ModelAssert: func(t *testing.T, m *DeployView) {
            assert.Equal(t, StateCreatingApp, m.state)
            assert.NotEmpty(t, m.zipPath)  // Real zip was created!
        },
    }).
    Run(t)
```

**What happens**:
1. `filesLoadedMsg` triggers state change → returns `zipFiles` command
2. Harness executes `zipFiles()` → **actually zips real files!**
3. `zipFiles()` returns `filesZippedMsg{zipPath: "...", zipSize: 628}`
4. `Finally()` intercepts `filesZippedMsg`
5. Processes message through `Update()` → state changes to `StateCreatingApp`
6. Runs assertions
7. **STOPS** - `createApp` command is never executed

**Use case**: Test real async operations (file I/O, etc.) while controlling the chain.

### Real-World Example: Deploy Flow

```go
func TestDeployWithRealZip(t *testing.T) {
    mockClient := apimock.NewClient(t)

    // Create test files in testdata/test-app/
    testFiles := []string{
        "testdata/test-app/main.py",
        "testdata/test-app/requirements.txt",
    }

    model := NewDeployView(DeployConfig{
        Config:    config,
        Client:    mockClient,
        ProjectID: "test-project",
    })

    harness := uitesting.NewTestHarness(t, &model)
    harness.
        Step(uitesting.TestStep[*DeployView]{
            Name: "files_loaded",
            Msg:  filesLoadedMsg{fileList: testFiles},
            SkipViewAssertion: true,
        }).
        Finally(uitesting.TestStep[*DeployView]{
            Name:       "files_zipped",
            ViewGolden: "deploy_real_zip",
            ModelAssert: func(t *testing.T, m *DeployView) {
                // Verify real zip was created
                assert.Equal(t, StateCreatingApp, m.state)
                assert.NotEmpty(t, m.zipPath)
                assert.Greater(t, m.zipSize, int64(0))
            },
        }).
        Run(t)
}
```

This test:
- ✅ Actually zips real files from testdata/
- ✅ Verifies the zip was created with correct size
- ✅ Tests state transitions
- ✅ Stops before calling unmocked CreateApp API
- ✅ Runs in <1ms (zipping 100 bytes is instant)

### Multiple Expect() Steps

You can chain multiple `Expect()` steps to test longer async sequences:

```go
harness.
    Step(uitesting.TestStep{
        Name: "start",
        Msg:  startMsg{},
    }).
    Expect(uitesting.TestStep{
        Name: "first_result",
        // Intercepts first async message
    }).
    Expect(uitesting.TestStep{
        Name: "second_result",
        // Intercepts second async message
    }).
    Finally(uitesting.TestStep{
        Name: "done",
        // Intercepts third async message and stops
    }).
    Run(t)
```

### Message Type Matching

The harness automatically filters framework messages:
- **Ignored**: `tea.BatchMsg`, `tea.KeyMsg`, `tea.MouseMsg`, `tea.WindowSizeMsg`
- **Intercepted**: Your custom message types (e.g., `filesZippedMsg`, `appCreatedMsg`)

If you receive an unexpected message type, the test fails with a clear error:
```
Unexpected message type during command processing.
Expected step: files_zipped
Got message type: *ui.UIError
Message: &{...}
```

### When to Use Each Approach

**Use Step()** when:
- Message doesn't trigger async commands
- You want to continue processing (testing longer chains)

**Use Finally()** with Msg when:
- Message triggers an async command you want to skip
- Testing a single state transition

**Use Step() + Finally()** without Msg when:
- Testing real async operations (file I/O, etc.)
- Want to intercept the async result and stop

**Use Step() + Expect() + ... + Finally()** when:
- Testing long async chains
- Need to assert state at multiple points
- Want full control over the command flow

## Related Resources

- [Bubbletea Documentation](https://github.com/charmbracelet/bubbletea)
- [Goldie Documentation](https://github.com/sebdah/goldie)
- [Charm's teatest (experimental)](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest)
- [Catwalk Testing Library](https://github.com/knz/catwalk)
