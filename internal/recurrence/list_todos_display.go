package recurrence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/douglasdemoura/chroncal/internal/storage"
	"github.com/douglasdemoura/chroncal/internal/timeutil"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

const dateLayout = "2006-01-02"

// ListOpenTodos returns every todo that is not COMPLETED and not CANCELLED.
// A non-recurring todo appears as itself, with or without a date. A recurring
// series appears once, as its current occurrence. The current occurrence is
// the first occurrence that no COMPLETED or CANCELLED override closes. An open
// override of that occurrence replaces it. A series with no open occurrence
// does not appear.
func (s *Service) ListOpenTodos(ctx context.Context) ([]ExpandedTodo, error) {
	open := storage.TodoFilterParams{HideCompleted: true}
	rows, err := s.q.ListTodosFiltered(ctx, open)
	if err != nil {
		return nil, fmt.Errorf("list open todos: %w", err)
	}
	result := make([]ExpandedTodo, 0, len(rows))
	for _, row := range rows {
		td := todoFromRow(row)
		result = append(result, ExpandedTodo{Todo: td, InstanceTime: todoAnchor(td)})
	}

	masters, err := s.q.ListRecurringTodosFiltered(ctx, open)
	if err != nil {
		return nil, fmt.Errorf("list open recurring todos: %w", err)
	}
	overridesByUID, err := s.todoOverridesByUID(ctx, masters)
	if err != nil {
		return nil, fmt.Errorf("list todo overrides: %w", err)
	}
	for _, row := range masters {
		if cur, ok := currentTodoOccurrence(todoFromRow(row), overridesByUID[seriesKey(row.CalendarID, row.Uid)]); ok {
			result = append(result, cur)
		}
	}

	s.populateExpandedTodoCategories(ctx, result)
	return result, nil
}

// currentTodoOccurrence returns the first occurrence of master that no closed
// override (COMPLETED or CANCELLED) removes. The walk stops after one more
// occurrence than the count of closed overrides. Each skipped occurrence uses
// up one closed override, so an infinite RRULE cannot loop forever.
func currentTodoOccurrence(master todo.Todo, overrides []storage.Todo) (ExpandedTodo, bool) {
	rs, ok := newTodoRRuleSet(master, true)
	if !ok {
		// A master without an anchor or with an unparseable RRULE is its
		// own sole occurrence (see singleTodoInstance).
		return ExpandedTodo{Todo: master, InstanceTime: todoAnchor(master)}, true
	}

	closed := make(map[string]struct{}, len(overrides))
	openOverrides := make(map[string]storage.Todo, len(overrides))
	for _, o := range overrides {
		key := canonicalRecurrenceID(o.RecurrenceID)
		if isClosedTodoStatus(o.Status) {
			closed[key] = struct{}{}
			continue
		}
		openOverrides[key] = o
	}

	next := rs.set.Iterator()
	for range len(closed) + 1 {
		occ, ok := next()
		if !ok {
			return ExpandedTodo{}, false
		}
		instance := occ.UTC()
		key := instance.Format(time.RFC3339)
		if _, done := closed[key]; done {
			continue
		}
		if o, ok := openOverrides[key]; ok {
			return ExpandedTodo{Todo: todoFromRow(o), InstanceTime: instance, IsOverride: true}, true
		}
		_, isRDate := rs.rdateSet[rdateKey(occ)]
		return ExpandedTodo{
			Todo:         todoAtOccurrence(ExpandedTodo{Todo: master, InstanceTime: instance}),
			InstanceTime: instance,
			IsOverride:   isRDate,
		}, true
	}
	return ExpandedTodo{}, false
}

// isClosedTodoStatus reports whether a todo status ends the todo.
func isClosedTodoStatus(status string) bool {
	return strings.EqualFold(status, "COMPLETED") || strings.EqualFold(status, "CANCELLED")
}

// ListCompletedTodos returns the COMPLETED non-recurring todos and COMPLETED
// overrides whose date falls in [from, to). The date is DUE, or the completion
// time for a todo without DUE. A date-only DUE is a whole UTC day, the same as
// an all-day event, so it matches when that day overlaps the window.
func (s *Service) ListCompletedTodos(ctx context.Context, from, to time.Time) ([]todo.Todo, error) {
	// The SQL bounds are whole UTC dates that contain the window. The exact
	// window check below then removes the rows outside it.
	fromDate := from.UTC().Format(dateLayout)
	toDate := to.UTC().AddDate(0, 0, 1).Format(dateLayout)
	rows, err := s.q.ListCompletedTodosByDate(ctx, storage.ListCompletedTodosByDateParams{
		FromDate: &fromDate,
		ToDate:   &toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("list completed todos: %w", err)
	}
	var result []todo.Todo
	for _, row := range rows {
		td := todoFromRow(row)
		if completedTodoInWindow(td, from, to) {
			result = append(result, td)
		}
	}
	s.populateTodoCategories(ctx, result)
	return result, nil
}

// completedTodoInWindow reports whether the date of a completed todo falls in
// [from, to). See ListCompletedTodos for the date rule.
func completedTodoInWindow(td todo.Todo, from, to time.Time) bool {
	if td.DueDate == "" {
		return inWindow(td.ParseCompletedAt(), from, to)
	}
	due := td.ParseDueDate()
	if timeutil.IsDateOnly(td.DueDate) {
		return overlapsWindow(due, due.AddDate(0, 0, 1), from, to)
	}
	return inWindow(due, from, to)
}

// populateExpandedTodoCategories loads the categories of each expanded todo.
func (s *Service) populateExpandedTodoCategories(ctx context.Context, expanded []ExpandedTodo) {
	todos := make([]todo.Todo, len(expanded))
	for i, e := range expanded {
		todos[i] = e.Todo
	}
	s.populateTodoCategories(ctx, todos)
	for i := range expanded {
		expanded[i].Categories = todos[i].Categories
	}
}
