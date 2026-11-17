package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/douglasdemoura/chroncal/internal/model"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

func strippedLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return out
}

func TestTodoDetailLines(t *testing.T) {
	withLocalUTC(t)
	td := todo.Todo{
		Summary:         "File taxes",
		DueDate:         "2026-10-12T15:00:00Z",
		StartDate:       "2026-10-01",
		Status:          "IN-PROCESS",
		Priority:        1,
		PercentComplete: 40,
		Location:        "Home office",
		Categories:      "admin",
		RecurrenceRule:  "FREQ=WEEKLY",
		Alarms:          []model.Alarm{{Action: "DISPLAY", TriggerValue: "-PT15M"}},
		Description:     "Bring receipts",
	}
	cal := CalendarInfo{Name: "Personal"}

	got := strippedLines(todoDetailLines(td, cal, true, 60, eventViewLabelWidth, false))

	require.Equal(t, []string{
		"       Due  Mon, Oct 12, 2026 15:00 (overdue)",
		"    Starts  Thu, Oct 1, 2026",
		"  Priority  High",
		"  Progress  40%",
		"",
		"  Calendar  ● Personal",
		"     Where  Home office",
		"    Status   In progress",
		"      Tags  admin",
		"    Repeat  Every week",
		"",
		"Alarms",
		"  15 min. before",
		"",
		"Notes",
		"Bring receipts",
	}, got)
}

func TestTodoDetailLines_Completed(t *testing.T) {
	withLocalUTC(t)
	td := todo.Todo{
		Summary:     "Pay rent",
		DueDate:     "2026-10-01",
		Status:      "COMPLETED",
		CompletedAt: "2026-10-02T09:30:00Z",
		Priority:    7,
	}

	got := strippedLines(todoDetailLines(td, CalendarInfo{}, false, 60, eventViewLabelWidth, false))

	require.Equal(t, []string{
		"       Due  Thu, Oct 1, 2026",
		" Completed  Fri, Oct 2, 2026 09:30",
		"  Priority  Low",
		"",
		"    Status   Completed",
	}, got)
}

func TestTodoPriorityLabel(t *testing.T) {
	for priority, want := range map[int64]string{0: "", 1: "High", 4: "High", 5: "Medium", 6: "Low", 9: "Low"} {
		require.Equal(t, want, todoPriorityLabel(priority), "priority %d", priority)
	}
}
