package tui

import (
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestRenderTodoLine(t *testing.T) {
	tests := []struct {
		name string
		item CalendarEvent
		want string
	}{
		{
			name: "open todo",
			item: CalendarEvent{Kind: calendarItemTodo, Title: "Pay rent"},
			want: " ☐ Pay rent ",
		},
		{
			name: "done todo",
			item: CalendarEvent{Kind: calendarItemTodo, Title: "Pay rent", Done: true},
			want: " ☑ Pay rent ",
		},
		{
			name: "long title is cut to the cell",
			item: CalendarEvent{Kind: calendarItemTodo, Title: "Renew the passport"},
			want: " ☐ Renew th…",
		},
		{
			name: "wide glyphs are cut by display width",
			item: CalendarEvent{Kind: calendarItemTodo, Title: "会議会議会議"},
			want: " ☐ 会議会議…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderTodoLine(tt.item, 12, false)
			require.Equal(t, 1, lipgloss.Height(out))
			require.Equal(t, 12, lipgloss.Width(out))
			require.Equal(t, tt.want, ansi.Strip(out))
		})
	}
}

func TestTodoGlyphColor(t *testing.T) {
	theme := ActiveTheme()
	require.Equal(t, lipgloss.Color("#112233"), todoGlyphColor(CalendarEvent{Color: "#112233"}))
	require.Equal(t, theme.Muted, todoGlyphColor(CalendarEvent{}))
	require.Equal(t, theme.Error, todoGlyphColor(CalendarEvent{Color: "#112233", Overdue: true}))
}

func TestRenderCalendarItem_TodoUsesTodoLine(t *testing.T) {
	todoOut := ansi.Strip(renderCalendarItem(CalendarEvent{Kind: calendarItemTodo, Title: "Task"}, 10, false))
	eventOut := ansi.Strip(renderCalendarItem(CalendarEvent{Title: "Meeting"}, 10, false))
	require.Equal(t, " ☐ Task   ", todoOut)
	require.Equal(t, " Meeting  ", eventOut)
}

func TestRenderTimeCellContent_Todo(t *testing.T) {
	p := placedEvent{event: CalendarEvent{
		Kind:      calendarItemTodo,
		Title:     "Call back",
		StartTime: time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC),
	}, startRow: 4, endRow: 6}

	require.Equal(t, " ☐ Call back  ", ansi.Strip(renderTimeCellContent(p, 4, 14)))
	require.Equal(t, strings.Repeat(" ", 14), ansi.Strip(renderTimeCellContent(p, 5, 14)))
}

func TestAllDayStripRows(t *testing.T) {
	day := time.Date(2026, 10, 16, 0, 0, 0, 0, time.Local)
	other := day.AddDate(0, 0, 1)
	items := []CalendarEvent{
		{ID: 1, Title: "Holiday", AllDay: true, Day: day},
		{ID: 2, Title: "Timed", Day: day, StartTime: day.Add(9 * time.Hour)},
		{ID: 3, Title: "Other day", AllDay: true, Day: other},
		{ID: 1, Kind: calendarItemTodo, Title: "a", AllDay: true, Day: day},
		{ID: 2, Kind: calendarItemTodo, Title: "b", AllDay: true, Day: day},
		{ID: 3, Kind: calendarItemTodo, Title: "c", AllDay: true, Day: day},
		{ID: 4, Kind: calendarItemTodo, Title: "d", AllDay: true, Day: day},
		{ID: 5, Kind: calendarItemTodo, Title: "e", AllDay: true, Day: day},
		{ID: 6, Kind: calendarItemTodo, Title: "timed todo", Day: day, StartTime: day.Add(10 * time.Hour)},
	}

	rows := allDayStripRows(items, day)

	var got []string
	for _, r := range rows {
		if r.hiddenTodos > 0 {
			got = append(got, "+more")
			require.Equal(t, 2, r.hiddenTodos)
			continue
		}
		got = append(got, r.item.Title)
	}
	require.Equal(t, []string{"Holiday", "a", "b", "c", "+more"}, got)
	require.Equal(t, " +2 todos ", ansi.Strip(renderStripRow(rows[4], 10)))
}

func TestAllDayStripRows_NoOverflowRowAtTheCap(t *testing.T) {
	day := time.Date(2026, 10, 16, 0, 0, 0, 0, time.Local)
	items := make([]CalendarEvent, 0, maxStripTodos)
	for i := range maxStripTodos {
		items = append(items, CalendarEvent{ID: int64(i + 1), Kind: calendarItemTodo, AllDay: true, Day: day})
	}
	rows := allDayStripRows(items, day)
	require.Len(t, rows, maxStripTodos)
	for _, r := range rows {
		require.Zero(t, r.hiddenTodos)
	}
}

func TestBuildCalendarCell_ShowsTodosAfterEvents(t *testing.T) {
	day := time.Date(2026, 10, 16, 0, 0, 0, 0, time.Local)
	items := []CalendarEvent{
		{ID: 1, Title: "Meeting", Day: day},
		{ID: 1, Kind: calendarItemTodo, Title: "Task", AllDay: true, Day: day},
	}
	lines := strings.Split(ansi.Strip(buildCalendarCell(day, false, false, true, items, 12, 4, "")), "\n")
	require.Equal(t, " Meeting    ", lines[1])
	require.Equal(t, " ☐ Task     ", lines[2])
}
