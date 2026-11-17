package tui

import (
	"cmp"
	"slices"
	"time"

	"github.com/douglasdemoura/chroncal/internal/timeutil"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

// todoPlacement tells where a todo appears in the calendar views.
type todoPlacement struct {
	// Day is the midnight of the display day in the display zone.
	Day time.Time
	// At is the DUE instant of a timed todo on its own DUE day. A todo with
	// a zero At goes in the all-day area of Day.
	At time.Time
	// Overdue is true for an open todo after its DUE.
	Overdue bool
}

// todoEntry is one todo with its placement.
type todoEntry struct {
	todo      todo.Todo
	placement todoPlacement
}

// placeTodo returns the display day of td. The day and the overdue state come
// from now and from the zone of now. These are the rules:
//
//   - A CANCELLED todo does not appear.
//   - A COMPLETED todo appears on its DUE date. Without DUE, it appears on
//     its completion date.
//   - An open todo with DUE appears on its DUE date. When that date is in the
//     past, the todo moves to today without its time and is overdue.
//   - An open todo without DUE appears on today. A future DTSTART holds it on
//     its DTSTART date until that date comes.
func placeTodo(td todo.Todo, now time.Time) (todoPlacement, bool) {
	loc := now.Location()
	today := dayIn(now, loc)
	switch td.Status {
	case "CANCELLED":
		return todoPlacement{}, false
	case "COMPLETED":
		if td.DueDate != "" {
			return todoPlacement{Day: todoDateDay(td.DueDate, loc), At: todoDateAt(td.DueDate)}, true
		}
		completed := td.ParseCompletedAt()
		if completed.IsZero() {
			return todoPlacement{}, false
		}
		return todoPlacement{Day: dayIn(completed, loc)}, true
	}

	if td.DueDate != "" {
		dueDay := todoDateDay(td.DueDate, loc)
		if dueDay.Before(today) {
			return todoPlacement{Day: today, Overdue: true}, true
		}
		at := todoDateAt(td.DueDate)
		return todoPlacement{Day: dueDay, At: at, Overdue: !at.IsZero() && now.After(at)}, true
	}
	if td.StartDate != "" {
		if startDay := todoDateDay(td.StartDate, loc); startDay.After(today) {
			return todoPlacement{Day: startDay}, true
		}
	}
	return todoPlacement{Day: today}, true
}

// todoDateDay returns the display day of a todo date string. A date-only
// value names a calendar date, so the day keeps that date in loc. A timed
// value is an instant, so the day is its local date in loc.
func todoDateDay(value string, loc *time.Location) time.Time {
	t := timeutil.ParseDate(value)
	if timeutil.IsDateOnly(value) {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	return dayIn(t, loc)
}

// todoDateAt returns the instant of a timed todo date string. It returns
// the zero time for a date-only value.
func todoDateAt(value string) time.Time {
	if timeutil.IsDateOnly(value) {
		return time.Time{}
	}
	return timeutil.ParseDate(value)
}

// dayIn returns the midnight of the date of t in loc.
func dayIn(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// placeTodos places each todo for now and sorts the result for display.
// It drops the todos that do not appear.
func placeTodos(todos []todo.Todo, now time.Time) []todoEntry {
	entries := make([]todoEntry, 0, len(todos))
	for _, td := range todos {
		if p, ok := placeTodo(td, now); ok {
			entries = append(entries, todoEntry{todo: td, placement: p})
		}
	}
	sortTodoEntries(entries)
	return entries
}

// sortTodoEntries puts the todos of one day in display order:
//
//  1. Open todos before completed todos.
//  2. Overdue todos first, the oldest DUE first.
//  3. Priority 1 (high) to 9 (low). Priority 0 (none) goes last.
//  4. Summary.
func sortTodoEntries(entries []todoEntry) {
	slices.SortStableFunc(entries, func(a, b todoEntry) int {
		if c := compareBool(a.todo.IsCompleted(), b.todo.IsCompleted()); c != 0 {
			return c
		}
		if c := compareBool(!a.placement.Overdue, !b.placement.Overdue); c != 0 {
			return c
		}
		if a.placement.Overdue {
			if c := a.todo.ParseDueDate().Compare(b.todo.ParseDueDate()); c != 0 {
				return c
			}
		}
		if c := cmp.Compare(todoPriorityRank(a.todo.Priority), todoPriorityRank(b.todo.Priority)); c != 0 {
			return c
		}
		return cmp.Compare(a.todo.Summary, b.todo.Summary)
	})
}

// compareBool orders false before true.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// todoPriorityRank maps an RFC 5545 PRIORITY to a sort rank. The value 0
// means "no priority", so it sorts after the lowest priority 9.
func todoPriorityRank(priority int64) int64 {
	if priority <= 0 {
		return 10
	}
	return priority
}

// todosToCalendar converts placed todos to calendar items. It skips the
// todos of hidden calendars.
func todosToCalendar(entries []todoEntry, calendars map[int64]CalendarInfo, hidden map[int64]bool) []CalendarEvent {
	out := make([]CalendarEvent, 0, len(entries))
	for _, e := range entries {
		if hidden[e.todo.CalendarID] {
			continue
		}
		item := CalendarEvent{
			ID:      e.todo.ID,
			Kind:    calendarItemTodo,
			Title:   e.todo.Summary,
			Color:   calendars[e.todo.CalendarID].Color,
			AllDay:  e.placement.At.IsZero(),
			Day:     e.placement.Day,
			Done:    e.todo.IsCompleted(),
			Overdue: e.placement.Overdue,
		}
		if !item.AllDay {
			item.StartTime = e.placement.At.In(e.placement.Day.Location())
		}
		out = append(out, item)
	}
	return out
}

// filterVisibleTodos drops the todos of hidden calendars.
func filterVisibleTodos(entries []todoEntry, hidden map[int64]bool) []todoEntry {
	if len(hidden) == 0 {
		return entries
	}
	out := make([]todoEntry, 0, len(entries))
	for _, e := range entries {
		if !hidden[e.todo.CalendarID] {
			out = append(out, e)
		}
	}
	return out
}

// todosOn returns the entries whose display day is day.
func todosOn(entries []todoEntry, day time.Time) []todoEntry {
	var out []todoEntry
	for _, e := range entries {
		if sameDay(e.placement.Day, day) {
			out = append(out, e)
		}
	}
	return out
}
