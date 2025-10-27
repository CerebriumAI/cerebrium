package ui

import (
	"github.com/charmbracelet/bubbles/table"
)

const (
	MAX_TABLE_HEIGHT = 15 // Not including header. Tables longer than this should scroll.
)

func TableBiggerThanView(t table.Model) bool {
	return len(t.Rows()) > MAX_TABLE_HEIGHT
}
