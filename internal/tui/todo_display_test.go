package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/douglasdemoura/chroncal/internal/todo"
)

// todoTestZone is five hours behind UTC. A UTC due time late in the evening
// then falls on the next UTC date, and the tests can catch a day computed in
// the wrong zone.
var todoTestZone = time.FixedZone("UTC-5", -5*60*60)

// todoTestNow is 14:00 local time on Friday, 16 October 2026.
var todoTestNow = time.Date(2026, 10, 16, 14, 0, 0, 0, todoTestZone)

func localDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, todoTestZone)
}

func TestPlaceTodo(t *testing.T) {
	today := localDay(2026, 10, 16)
	tests := []struct {
		name   string
		td     todo.Todo
		want   todoPlacement
		placed bool
	}{
		{
			name:   "undated open todo goes on today",
			td:     todo.Todo{Status: "NEEDS-ACTION"},
			want:   todoPlacement{Day: today},
			placed: true,
		},
		{
			name:   "date-only DUE in the future stays on its date",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-20"},
			want:   todoPlacement{Day: localDay(2026, 10, 20)},
			placed: true,
		},
		{
			name:   "date-only DUE today is not overdue",
			td:     todo.Todo{Status: "IN-PROCESS", DueDate: "2026-10-16"},
			want:   todoPlacement{Day: today},
			placed: true,
		},
		{
			name:   "date-only DUE in the past moves to today as overdue",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-12"},
			want:   todoPlacement{Day: today, Overdue: true},
			placed: true,
		},
		{
			name:   "timed DUE later today keeps its time",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-16T20:00:00Z"},
			want:   todoPlacement{Day: today, At: time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)},
			placed: true,
		},
		{
			name:   "timed DUE earlier today keeps its time and is overdue",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-16T15:00:00Z"},
			want:   todoPlacement{Day: today, At: time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC), Overdue: true},
			placed: true,
		},
		{
			name:   "timed DUE uses the local date, not the UTC date",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-17T02:00:00Z"},
			want:   todoPlacement{Day: today, At: time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)},
			placed: true,
		},
		{
			name:   "timed DUE on a later day stays on that day",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-19T14:00:00Z"},
			want:   todoPlacement{Day: localDay(2026, 10, 19), At: time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)},
			placed: true,
		},
		{
			name:   "timed DUE on an earlier day moves to today without its time",
			td:     todo.Todo{Status: "NEEDS-ACTION", DueDate: "2026-10-15T14:00:00Z"},
			want:   todoPlacement{Day: today, Overdue: true},
			placed: true,
		},
		{
			name:   "future DTSTART without DUE waits on its start date",
			td:     todo.Todo{Status: "NEEDS-ACTION", StartDate: "2026-10-22"},
			want:   todoPlacement{Day: localDay(2026, 10, 22)},
			placed: true,
		},
		{
			name:   "past DTSTART without DUE goes on today",
			td:     todo.Todo{Status: "NEEDS-ACTION", StartDate: "2026-10-01"},
			want:   todoPlacement{Day: today},
			placed: true,
		},
		{
			name:   "completed todo stays on its DUE date",
			td:     todo.Todo{Status: "COMPLETED", DueDate: "2026-10-12", CompletedAt: "2026-10-14T12:00:00Z"},
			want:   todoPlacement{Day: localDay(2026, 10, 12)},
			placed: true,
		},
		{
			name:   "completed timed todo keeps its time",
			td:     todo.Todo{Status: "COMPLETED", DueDate: "2026-10-12T15:00:00Z"},
			want:   todoPlacement{Day: localDay(2026, 10, 12), At: time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)},
			placed: true,
		},
		{
			name:   "completed undated todo goes on its local completion date",
			td:     todo.Todo{Status: "COMPLETED", CompletedAt: "2026-10-14T03:00:00Z"},
			want:   todoPlacement{Day: localDay(2026, 10, 13)},
			placed: true,
		},
		{
			name: "completed undated todo without a completion time is not placed",
			td:   todo.Todo{Status: "COMPLETED"},
		},
		{
			name: "cancelled todo is not placed",
			td:   todo.Todo{Status: "CANCELLED"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := placeTodo(tt.td, todoTestNow)
			require.Equal(t, tt.placed, ok)
			if !tt.placed {
				return
			}
			require.True(t, got.Day.Equal(tt.want.Day), "Day = %v, want %v", got.Day, tt.want.Day)
			require.True(t, got.At.Equal(tt.want.At), "At = %v, want %v", got.At, tt.want.At)
			require.Equal(t, tt.want.Overdue, got.Overdue)
		})
	}
}

func TestPlaceTodo_RollsOverAtLocalMidnight(t *testing.T) {
	td := todo.Todo{Status: "NEEDS-ACTION"}
	before, ok := placeTodo(td, time.Date(2026, 10, 16, 23, 59, 0, 0, todoTestZone))
	require.True(t, ok)
	after, ok := placeTodo(td, time.Date(2026, 10, 17, 0, 1, 0, 0, todoTestZone))
	require.True(t, ok)
	require.True(t, before.Day.Equal(localDay(2026, 10, 16)))
	require.True(t, after.Day.Equal(localDay(2026, 10, 17)))
}

func TestSortTodoEntries(t *testing.T) {
	entry := func(summary string, priority int64, due string, overdue bool, status string) todoEntry {
		return todoEntry{
			todo:      todo.Todo{Summary: summary, Priority: priority, DueDate: due, Status: status},
			placement: todoPlacement{Overdue: overdue},
		}
	}
	entries := []todoEntry{
		entry("done", 1, "", false, "COMPLETED"),
		entry("no priority", 0, "", false, "NEEDS-ACTION"),
		entry("low", 9, "", false, "NEEDS-ACTION"),
		entry("high b", 1, "", false, "NEEDS-ACTION"),
		entry("high a", 1, "", false, "NEEDS-ACTION"),
		entry("overdue newer", 9, "2026-10-14", true, "NEEDS-ACTION"),
		entry("overdue older", 0, "2026-10-02", true, "NEEDS-ACTION"),
	}
	sortTodoEntries(entries)

	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.todo.Summary
	}
	require.Equal(t, []string{
		"overdue older", "overdue newer", "high a", "high b", "low", "no priority", "done",
	}, got)
}

func TestTodosToCalendar(t *testing.T) {
	at := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	entries := []todoEntry{
		{todo: todo.Todo{ID: 1, CalendarID: 1, Summary: "Open", Status: "NEEDS-ACTION"}, placement: todoPlacement{Day: localDay(2026, 10, 16), Overdue: true}},
		{todo: todo.Todo{ID: 2, CalendarID: 1, Summary: "Timed", Status: "COMPLETED"}, placement: todoPlacement{Day: localDay(2026, 10, 16), At: at}},
		{todo: todo.Todo{ID: 3, CalendarID: 2, Summary: "Hidden"}, placement: todoPlacement{Day: localDay(2026, 10, 16)}},
	}
	calendars := map[int64]CalendarInfo{1: {Color: "#112233"}, 2: {Color: "#445566"}}

	got := todosToCalendar(entries, calendars, map[int64]bool{2: true})

	require.Len(t, got, 2)
	require.Equal(t, CalendarEvent{
		ID: 1, Kind: calendarItemTodo, Title: "Open", Color: "#112233",
		AllDay: true, Day: localDay(2026, 10, 16), Overdue: true,
	}, got[0])
	require.Equal(t, CalendarEvent{
		ID: 2, Kind: calendarItemTodo, Title: "Timed", Color: "#112233",
		Day: localDay(2026, 10, 16), StartTime: at.In(todoTestZone), Done: true,
	}, got[1])
}
