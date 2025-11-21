package testing

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/sebdah/goldie/v2"
)

// TestHarness provides a step-based testing framework for Bubbletea models.
// It allows you to:
// - Initialize a model
// - Send a sequence of messages to Update()
// - Assert View() output using golden files or custom assertions
// - Assert model state after each step
// - Intercept and control async command execution with Expect/Finally
//
// Example usage (basic):
//
//	harness := NewTestHarness(t, myModel)
//	harness.
//		Step(TestStep[MyModel]{
//			Name: "initial_state",
//			Msg:  nil,  // Just render initial state
//			ViewGolden: "initial_state",
//		}).
//		Step(TestStep[MyModel]{
//			Name: "after_data_loaded",
//			Msg:  dataLoadedMsg{data: "test"},
//			ViewGolden: "after_data_loaded",
//			ModelAssert: func(t *testing.T, m MyModel) {
//				assert.Equal(t, "test", m.data)
//			},
//		}).
//		Run(t)
//
// Example usage (with async command interception):
//
//	harness := NewTestHarness(t, myModel)
//	harness.
//		Step(TestStep[MyModel]{
//			Name: "trigger_async",
//			Msg:  startMsg{},
//		}).
//		Expect(TestStep[MyModel]{
//			Name: "async_result",
//			// No Msg - this step intercepts the message from the previous async command
//			ViewGolden: "after_async",
//		}).
//		Finally(TestStep[MyModel]{
//			Name: "final_state",
//			// Stops command processing after this step
//			ViewGolden: "final",
//		}).
//		Run(t)
type TestHarness[T tea.Model] struct {
	model              T
	steps              []TestStep[T]
	expectedSteps      []TestStep[T] // Steps that expect messages from async commands
	finalStep          *TestStep[T]  // Final step that stops command processing
	goldie             *goldie.Goldie
	currentExpectIndex int  // Tracks which expected step we're on
	stopProcessing     bool // Set to true when Finally() step is reached
}

// TestStep represents one step in a test sequence.
// Each step:
// 1. Sends Msg to model.Update()
// 2. Asserts View() output (optional)
// 3. Asserts model state (optional)
//
// T is the concrete model type (e.g., DeployView, LoginView, SpinnerModel)
type TestStep[T tea.Model] struct {
	// Name identifies this step (used in error messages and golden file names)
	Name string

	// Msg to send to Update(). If nil, only View() is called (useful for testing initial state)
	// For Expect() steps, this should be nil - the message comes from the async command
	Msg tea.Msg

	// ExpectedMsgType is the expected message type for Expect() steps.
	// Use a zero value of your message type, e.g., filesZippedMsg{}
	// The harness will verify the received message matches this type.
	ExpectedMsgType tea.Msg

	// MessageAssert validates the actual message received (for Expect/Finally steps)
	// This runs BEFORE Update() is called, so you can inspect the raw message
	MessageAssert func(t *testing.T, msg tea.Msg)

	// ViewGolden is the golden file name for View() output.
	// If set, View() output will be compared against testdata/<ViewGolden>.golden
	// Use -update flag to regenerate golden files: go test -update
	ViewGolden string

	// ViewAssert is a custom assertion for View() output.
	// Use this for more complex assertions than golden file comparison.
	// If both ViewGolden and ViewAssert are set, both will run.
	ViewAssert func(t *testing.T, view string)

	// ModelAssert is a custom assertion for model state.
	// Use this to verify internal model state after Update().
	// The model is already the concrete type T, no type assertion needed.
	ModelAssert func(t *testing.T, m T)

	// SkipViewAssertion skips View() assertions for this step.
	// Useful when you only care about model state, not rendering.
	SkipViewAssertion bool
}

// NewTestHarness creates a new test harness for a Bubbletea model.
//
// IMPORTANT: This function sets up a consistent testing environment:
// - Forces ASCII color profile (prevents color inconsistencies across environments)
// - Uses fixed terminal size via model initialization
//
// The model is NOT initialized yet - call Run() to execute Init() and all steps.
func NewTestHarness[T tea.Model](t *testing.T, model T) *TestHarness[T] {
	t.Helper()

	// Force ASCII color profile for consistent golden files across environments
	// This prevents ANSI escape codes from varying between CI and local
	lipgloss.SetColorProfile(termenv.Ascii)

	return &TestHarness[T]{
		model:              model,
		steps:              []TestStep[T]{},
		expectedSteps:      []TestStep[T]{},
		finalStep:          nil,
		currentExpectIndex: 0,
		stopProcessing:     false,
		goldie: goldie.New(t,
			goldie.WithFixtureDir("testdata"),
			goldie.WithNameSuffix(".golden"),
		),
	}
}

// Step adds a test step to the harness.
// Steps are executed in the order they are added.
// Returns the harness for method chaining.
func (h *TestHarness[T]) Step(step TestStep[T]) *TestHarness[T] {
	h.steps = append(h.steps, step)
	return h
}

// Expect adds an expected step that will intercept a message from async command execution.
// Expected steps should NOT have Msg set - they receive messages from commands.
// The harness will process commands until it receives a message, then match it against
// the next expected step. If the message type doesn't match, the test fails.
//
// Use this to test async command chains while maintaining control:
//
//	harness.
//		Step(TestStep{Name: "trigger", Msg: startMsg{}}).  // Triggers async command
//		Expect(TestStep{Name: "result"}).                   // Receives command's message
//		Run(t)
//
// Returns the harness for method chaining.
func (h *TestHarness[T]) Expect(step TestStep[T]) *TestHarness[T] {
	h.expectedSteps = append(h.expectedSteps, step)
	return h
}

// Finally adds a final step that stops command processing.
// After this step's assertions run, no more commands will be processed.
// This prevents infinite command loops and gives you control over when to stop.
//
// Use this as the last step in a chain of Expect() calls:
//
//	harness.
//		Step(TestStep{Msg: startMsg{}}).
//		Expect(TestStep{Name: "first_result"}).
//		Expect(TestStep{Name: "second_result"}).
//		Finally(TestStep{Name: "done"}).  // Stops here
//		Run(t)
//
// Returns the harness for method chaining.
func (h *TestHarness[T]) Finally(step TestStep[T]) *TestHarness[T] {
	h.finalStep = &step
	return h
}

// Run executes the test harness:
//  1. Calls model.Init() and processes any returned commands
//  2. For each Step:
//     a. Sends the message to Update()
//     b. Processes commands, checking against Expect() steps
//     c. Stops at Finally() if configured
//     d. Asserts View() and model state
//
// With Expect/Finally, command processing intercepts async messages:
// - Each Expect() step matches one async message by type
// - Finally() stops command processing completely
// - This gives you full control over async command chains
func (h *TestHarness[T]) Run(t *testing.T) {
	t.Helper()

	// Reset interception state
	h.currentExpectIndex = 0
	h.stopProcessing = false

	// Step 1: Initialize model
	initCmd := h.model.Init()
	h.processCommands(t, initCmd, 0)

	// Step 2: Execute each regular step
	for _, step := range h.steps {
		if h.stopProcessing {
			break
		}

		t.Run(step.Name, func(t *testing.T) {
			// Send message and get updated model + command
			var cmd tea.Cmd
			if step.Msg != nil {
				updatedModel, updateCmd := h.model.Update(step.Msg)
				var ok bool
				h.model, ok = updatedModel.(T)
				if !ok {
					t.Fatalf("model %T is not %T", updatedModel, new(T))
				}
				cmd = updateCmd

				// Process commands with interception support
				h.processCommands(t, cmd, 0)
			}

			// Assert View() output
			if !step.SkipViewAssertion {
				view := h.model.View()
				view = normalizeView(view)

				if step.ViewGolden != "" {
					h.goldie.Assert(t, step.ViewGolden, []byte(view))
				}

				if step.ViewAssert != nil {
					step.ViewAssert(t, view)
				}
			}

			// Assert model state
			if step.ModelAssert != nil {
				step.ModelAssert(t, h.model)
			}
		})
	}
}

const maxCommandDepth = 10

// processCommands executes a command and feeds any returned messages back to Update().
// This simulates the Bubbletea runtime loop where commands produce messages.
//
// With Expect/Finally configured:
// - When a command produces a message, check if it matches the next expected step
// - If it matches, run assertions and optionally stop (if Finally)
// - If it doesn't match and we have expectations, fail the test
//
// Commands are processed recursively until:
// - No more commands are returned
// - maxCommandDepth is reached (prevents infinite loops)
// - An expected Finally step is reached (stops processing)
//
// To prevent infinite loops from tick-based commands (like spinner.Tick),
// we limit recursion depth to maxCommandDepth.
func (h *TestHarness[T]) processCommands(t *testing.T, cmd tea.Cmd, depth int) {
	t.Helper()

	if cmd == nil || h.stopProcessing {
		return
	}

	// Prevent infinite loops from tick-based commands
	if depth >= maxCommandDepth {
		t.Log("max command depth exceeded")
		return
	}

	// Execute command to get message
	msg := cmd()
	if msg == nil {
		return
	}

	// Check if this message should be intercepted by an Expect or Finally step
	if h.shouldIntercept(t, msg) {
		return // Interception handled the message and assertions
	}

	// Feed message back to Update() (normal flow)
	updatedModel, nextCmd := h.model.Update(msg)
	h.model = updatedModel.(T) //nolint:errcheck // Type assertion guaranteed by test harness generic type

	// Recursively process any commands returned by Update()
	h.processCommands(t, nextCmd, depth+1)
}

// shouldIntercept checks if a message should be intercepted by an Expect/Finally step.
// Returns true if the message was intercepted and handled.
func (h *TestHarness[T]) shouldIntercept(t *testing.T, msg tea.Msg) bool {
	t.Helper()

	// No interception configured
	if len(h.expectedSteps) == 0 && h.finalStep == nil {
		return false
	}

	// Check if we're expecting more steps
	if h.currentExpectIndex < len(h.expectedSteps) {
		step := h.expectedSteps[h.currentExpectIndex]

		// Match by message type
		if matchesMessageType(msg, step) {
			h.currentExpectIndex++

			// Run message assertions BEFORE Update() (inspect raw message)
			if step.MessageAssert != nil {
				step.MessageAssert(t, msg)
			}

			// Feed message to Update()
			updatedModel, _ := h.model.Update(msg)
			h.model = updatedModel.(T) //nolint:errcheck // Type assertion guaranteed by test harness generic type

			// Then run View/Model assertions on new state
			h.runStepAssertions(t, step)
			return true
		}

		// Message doesn't match - fail the test
		expectedTypeName := "any async message"
		if step.ExpectedMsgType != nil {
			expectedTypeName = reflect.TypeOf(step.ExpectedMsgType).String()
		}
		t.Fatalf("Unexpected message type during command processing.\nExpected step: %s (type: %s)\nGot message type: %T\nMessage: %+v",
			step.Name, expectedTypeName, msg, msg)
		return true
	}

	// Check if this is the Finally step
	if h.finalStep != nil {
		if matchesMessageType(msg, *h.finalStep) {
			// Run message assertions BEFORE Update() (inspect raw message)
			if h.finalStep.MessageAssert != nil {
				h.finalStep.MessageAssert(t, msg)
			}

			// Feed message to Update()
			updatedModel, _ := h.model.Update(msg)
			h.model = updatedModel.(T) //nolint:errcheck // Type assertion guaranteed by test harness generic type

			// Then run View/Model assertions on new state
			h.runStepAssertions(t, *h.finalStep)

			h.stopProcessing = true // Stop all further processing
			return true
		}

		// Message doesn't match - if it's a framework message (Batch, KeyMsg, etc.),
		// let it pass through normal processing. Otherwise fail.
		if !isFrameworkMessage(msg) {
			expectedTypeName := "any async message"
			if h.finalStep.ExpectedMsgType != nil {
				expectedTypeName = reflect.TypeOf(h.finalStep.ExpectedMsgType).String()
			}
			t.Fatalf("Unexpected message before Finally step.\nExpected Finally step: %s (type: %s)\nGot message type: %T\nMessage: %+v",
				h.finalStep.Name, expectedTypeName, msg, msg)
		}
		return false // Let framework messages pass through
	}

	return false
}

// isFrameworkMessage returns true for messages that are framework-level
// (BatchMsg, spinner ticks, etc.) and should not be intercepted
func isFrameworkMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.BatchMsg, tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg:
		return true
	default:
		return false
	}
}

// matchesMessageType checks if a message matches a test step's expected type.
// If ExpectedMsgType is set, checks for exact type match.
// Otherwise, accepts any non-framework message (backward compatible).
func matchesMessageType[T tea.Model](msg tea.Msg, step TestStep[T]) bool {
	// If ExpectedMsgType is specified, check for exact type match
	if step.ExpectedMsgType != nil {
		msgType := reflect.TypeOf(msg)
		expectedType := reflect.TypeOf(step.ExpectedMsgType)
		return msgType == expectedType
	}

	// Backward compatible: accept any message except framework messages
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg:
		return false // User input
	case tea.BatchMsg:
		return false // Batch is a wrapper, not a real async result
	default:
		return true // Assume it's an async result message
	}
}

// runStepAssertions runs the View and Model assertions for an intercepted step
// The message has already been processed by Update() in shouldIntercept,
// so we just run the assertions on the current model state.
func (h *TestHarness[T]) runStepAssertions(t *testing.T, step TestStep[T]) {
	t.Helper()

	t.Run(step.Name, func(t *testing.T) {
		// Assert View() output
		if !step.SkipViewAssertion {
			view := h.model.View()
			view = normalizeView(view)

			if step.ViewGolden != "" {
				h.goldie.Assert(t, step.ViewGolden, []byte(view))
			}

			if step.ViewAssert != nil {
				step.ViewAssert(t, view)
			}
		}

		// Assert model state
		if step.ModelAssert != nil {
			step.ModelAssert(t, h.model)
		}
	})
}

// normalizeView normalizes View() output for consistent golden file comparison.
// - Trims leading/trailing whitespace
// - Normalizes line endings to \n
func normalizeView(view string) string {
	view = strings.TrimSpace(view)
	view = strings.ReplaceAll(view, "\r\n", "\n")
	return view
}

// AssertContains is a helper assertion that checks if view contains a substring.
// Useful for ViewAssert when you don't want to use golden files.
func AssertContains(t *testing.T, view, substring string) {
	t.Helper()
	if !strings.Contains(view, substring) {
		t.Errorf("View does not contain expected substring.\nExpected substring: %q\nActual view:\n%s", substring, view)
	}
}

// AssertNotContains is a helper assertion that checks if view does NOT contain a substring.
func AssertNotContains(t *testing.T, view, substring string) {
	t.Helper()
	if strings.Contains(view, substring) {
		t.Errorf("View contains unexpected substring.\nUnexpected substring: %q\nActual view:\n%s", substring, view)
	}
}
