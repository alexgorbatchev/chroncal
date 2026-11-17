package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/douglasdemoura/chroncal/internal/timeutil"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

// todoDetailLines returns the detail rows of a todo for the day dialog and
// the todo view dialog. overdue marks the DUE row (see placeTodo).
// interactive enables mouse-zone markers on link rows (see
// eventMetaDetailLinesOptions).
func todoDetailLines(td todo.Todo, cal CalendarInfo, overdue bool, w, labelWidth int, interactive bool) []string {
	faint := lipgloss.NewStyle().Faint(true)
	var lines []string
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, detailLine(faint, label, value, labelWidth, w))
		}
	}

	add("Due", formatTodoDue(td.DueDate, overdue))
	add("Starts", formatTodoDate(td.StartDate))
	if td.IsCompleted() {
		if completed := td.ParseCompletedAt(); !completed.IsZero() {
			add("Completed", completed.Local().Format("Mon, Jan 2, 2006 15:04"))
		}
	}
	add("Priority", todoPriorityLabel(td.Priority))
	if td.PercentComplete > 0 && !td.IsCompleted() {
		add("Progress", fmt.Sprintf("%d%%", td.PercentComplete))
	}

	rw := eventMetaURLRewriter(cal)
	meta := eventMetaDetailLines(eventMetaDetailLinesOptions{
		labelStyle:  faint,
		width:       w,
		labelWidth:  labelWidth,
		urlRewriter: rw,
		interactive: interactive,
		calendar:    cal,
		location:    td.Location,
		url:         td.URL,
		status:      td.Status,
		statusBadge: todoStatusBadge,
		tags:        td.Categories,
		repeat:      describeRecurrence(td.RecurrenceRule),
		visibility:  formatVisibility(td.Class),
	})
	if len(meta) > 0 {
		lines = append(lines, "")
		lines = append(lines, meta...)
	}

	if len(td.Alarms) > 0 {
		lines = append(lines, "", faint.Render("Alarms"))
		for _, a := range td.Alarms {
			lines = append(lines, truncateTo("  "+formatAlarmWithAction(a), w))
		}
	}
	if td.Description != "" {
		lines = append(lines, "", faint.Render("Notes"))
		lines = append(lines, descriptionLines(td.Description, w, rw, interactive)...)
	}
	return lines
}

// formatTodoDue returns the DUE text of a todo. An overdue todo gets an
// "(overdue)" mark in the error color.
func formatTodoDue(due string, overdue bool) string {
	text := formatTodoDate(due)
	if text == "" || !overdue {
		return text
	}
	return text + " " + lipgloss.NewStyle().Foreground(ActiveTheme().Error).Render("(overdue)")
}

// formatTodoDate formats a todo date string for display. A date-only value
// shows the date. A timed value shows the local date and time.
func formatTodoDate(value string) string {
	if value == "" {
		return ""
	}
	t := timeutil.ParseDate(value)
	if t.IsZero() {
		return value
	}
	if timeutil.IsDateOnly(value) {
		return t.Format("Mon, Jan 2, 2006")
	}
	return t.Local().Format("Mon, Jan 2, 2006 15:04")
}

// todoPriorityLabel maps an RFC 5545 PRIORITY to the three levels that
// RFC 5545 section 3.8.1.9 names: 1-4 high, 5 medium, 6-9 low. The value 0
// means no priority and returns an empty label.
func todoPriorityLabel(priority int64) string {
	switch {
	case priority <= 0:
		return ""
	case priority <= 4:
		return "High"
	case priority == 5:
		return "Medium"
	default:
		return "Low"
	}
}

// todoStatusBadge renders a VTODO status as a colored pill.
func todoStatusBadge(status string) string {
	switch strings.ToUpper(status) {
	case "":
		return ""
	case "NEEDS-ACTION":
		return badge("Needs action", badgeNeutral)
	case "IN-PROCESS":
		return badge("In progress", badgeInfo)
	case "COMPLETED":
		return badge("Completed", badgeOK)
	case "CANCELLED":
		return badge("Cancelled", badgeDanger)
	default:
		return badge(titleCase(status), badgeNeutral)
	}
}
