package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
)

// maxStripTodos is the maximum count of todo rows for one day in the
// all-day strip of the week and day views. More todos collapse into one
// "+N todos" row, so a long todo list does not push out the time grid.
const maxStripTodos = 3

// todoLinePrefixWidth is the display width of " ☐ " in front of a todo title.
const todoLinePrefixWidth = 3

// todoGlyphColor returns the checkbox color of a todo item. The calendar
// color identifies the calendar. An overdue todo uses the error color.
func todoGlyphColor(ev CalendarEvent) color.Color {
	switch {
	case ev.Overdue:
		return ActiveTheme().Error
	case ev.Color != "":
		return lipgloss.Color(ev.Color)
	default:
		return ActiveTheme().Muted
	}
}

// todoGlyph returns the checkbox glyph of a todo item.
func todoGlyph(done bool) string {
	if done {
		return Glyphs["todo.done"]
	}
	return Glyphs["todo.open"]
}

// renderTodoLine renders a todo item as one line of width w: a checkbox in
// the calendar color and the title in the text color. A todo line has no
// background, so it stays different from an event pill. A done todo has a
// struck-through title. faint dims the line for a day outside the month.
func renderTodoLine(ev CalendarEvent, w int, faint bool) string {
	if w <= 0 {
		return ""
	}
	theme := ActiveTheme()
	glyphStyle := lipgloss.NewStyle().Foreground(todoGlyphColor(ev)).Faint(faint)
	titleStyle := lipgloss.NewStyle().Foreground(theme.Text).Faint(faint)
	if ev.Done {
		titleStyle = titleStyle.Foreground(theme.TextDim).Strikethrough(true)
	}
	if w < todoLinePrefixWidth {
		return glyphStyle.Render(truncateTo(" "+todoGlyph(ev.Done), w))
	}
	title := truncateTo(ev.Title, w-todoLinePrefixWidth)
	line := " " + glyphStyle.Render(todoGlyph(ev.Done)) + " " + titleStyle.Render(title)
	return line + strings.Repeat(" ", max(w-lipgloss.Width(line), 0))
}

// renderCalendarItem renders one calendar item as a single line of width w.
// An event renders as a pill and a todo renders as a todo line.
func renderCalendarItem(ev CalendarEvent, w int, faint bool) string {
	if ev.Kind == calendarItemTodo {
		return renderTodoLine(ev, w, faint)
	}
	return renderEventPill(ev, w, faint)
}

// stripRow is one row of the all-day strip of a day. hiddenTodos is
// non-zero for the overflow row, which counts the todos the strip does not
// show.
type stripRow struct {
	item        CalendarEvent
	hiddenTodos int
}

// allDayStripRows returns the all-day strip rows of day: all-day events
// first, then at most maxStripTodos untimed todos, then one overflow row
// for the other todos.
func allDayStripRows(items []CalendarEvent, day time.Time) []stripRow {
	dayKey := day.Format("2006-01-02")
	var rows, todos []stripRow
	for _, ev := range items {
		if !ev.AllDay || ev.Day.Format("2006-01-02") != dayKey {
			continue
		}
		if ev.Kind == calendarItemTodo {
			todos = append(todos, stripRow{item: ev})
			continue
		}
		rows = append(rows, stripRow{item: ev})
	}
	if len(todos) <= maxStripTodos {
		return append(rows, todos...)
	}
	rows = append(rows, todos[:maxStripTodos]...)
	return append(rows, stripRow{hiddenTodos: len(todos) - maxStripTodos})
}

// renderStripRow renders one all-day strip row with width w.
func renderStripRow(r stripRow, w int) string {
	if r.hiddenTodos > 0 {
		return lipgloss.NewStyle().Faint(true).Width(w).
			Render(truncateTo(fmt.Sprintf(" +%d todos", r.hiddenTodos), w))
	}
	return renderCalendarItem(r.item, w, false)
}
