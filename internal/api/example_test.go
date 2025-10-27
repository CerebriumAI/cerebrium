package api_test

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
)

// ExampleRun_GetDisplayStatus demonstrates how run status is formatted for display
func ExampleRun_GetDisplayStatus() {
	// Status code -1 (closed)
	run1 := api.Run{StatusCode: intPtr(-1), Status: "anything"}
	fmt.Println(run1.GetDisplayStatus())

	// Status code 0 (cancelled)
	run2 := api.Run{StatusCode: intPtr(0), Status: "anything"}
	fmt.Println(run2.GetDisplayStatus())

	// Status code 200 (success)
	run3 := api.Run{StatusCode: intPtr(200), Status: "anything"}
	fmt.Println(run3.GetDisplayStatus())

	// Status code 500 (failure)
	run4 := api.Run{StatusCode: intPtr(500), Status: "anything"}
	fmt.Println(run4.GetDisplayStatus())

	// Status code 404 (shows code)
	run5 := api.Run{StatusCode: intPtr(404), Status: "anything"}
	fmt.Println(run5.GetDisplayStatus())

	// No status code, containerQueued status
	run6 := api.Run{Status: "containerQueued"}
	fmt.Println(run6.GetDisplayStatus())

	// No status code, pending status
	run7 := api.Run{Status: "pending"}
	fmt.Println(run7.GetDisplayStatus())

	// No status code, regular status
	run8 := api.Run{Status: "Success"}
	fmt.Println(run8.GetDisplayStatus())

	// Output:
	// closed
	// cancelled
	// success
	// failure
	// 404
	// queued
	// pending
	// success
}

func intPtr(i int) *int {
	return &i
}
