package mock

// Generate mock for the Client interface using Mockery v3
// Configuration is defined in .mockery.yaml at the repository root
//
// This creates a mock that implements github.com/cerebriumai/cerebrium/internal/api.Client
//
// Usage in tests:
//   mockClient := mock.NewClient(t)
//   mockClient.EXPECT().GetApp(ctx, "project-id", "app-id").Return(&api.AppDetails{...}, nil)
//
// To regenerate:
//   cd <repo-root> && go generate ./internal/api/mock
//   OR from repo root: mockery
//
// For more information, see: https://vektra.github.io/mockery/v3.5/

//go:generate mockery
