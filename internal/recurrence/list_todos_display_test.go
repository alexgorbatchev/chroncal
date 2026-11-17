package recurrence

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/douglasdemoura/chroncal/internal/testutil"
	"github.com/douglasdemoura/chroncal/internal/todo"
)

// createTodo creates a todo through the todo service and fails the test on error.
func createTodo(t *testing.T, svc *todo.Service, p todo.CreateParams) todo.Todo {
	t.Helper()
	if p.CalendarID == 0 {
		p.CalendarID = 1
	}
	td, err := svc.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("create %q: %v", p.Summary, err)
	}
	return td
}

// upsertTodo writes a todo row with an explicit UID through the todo service.
// The tests use it for override rows and for rows with a fixed completion time.
func upsertTodo(t *testing.T, svc *todo.Service, p todo.UpsertParams) todo.Todo {
	t.Helper()
	if p.CalendarID == 0 {
		p.CalendarID = 1
	}
	td, err := svc.UpsertByUID(context.Background(), p)
	if err != nil {
		t.Fatalf("upsert %q: %v", p.Summary, err)
	}
	return td
}

// todoKey is the comparable part of a listed todo: the summary and the
// occurrence DUE date.
type todoKey struct {
	Summary string
	DueDate string
}

func sortedTodoKeys(todos []todo.Todo) []todoKey {
	keys := make([]todoKey, len(todos))
	for i, td := range todos {
		keys[i] = todoKey{Summary: td.Summary, DueDate: td.DueDate}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Summary != keys[j].Summary {
			return keys[i].Summary < keys[j].Summary
		}
		return keys[i].DueDate < keys[j].DueDate
	})
	return keys
}

func assertTodoKeys(t *testing.T, got []todo.Todo, want []todoKey) {
	t.Helper()
	keys := sortedTodoKeys(got)
	if len(keys) != len(want) {
		t.Fatalf("got %d todos %v, want %d %v", len(keys), keys, len(want), want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("todos = %v, want %v", keys, want)
		}
	}
}

func TestListOpenTodos(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, svc *todo.Service)
		want  []todoKey
	}{
		{
			name: "non-recurring open todos with and without a date",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Undated"})
				createTodo(t, svc, todo.CreateParams{Summary: "Dated", DueDate: "2026-10-20"})
				createTodo(t, svc, todo.CreateParams{Summary: "Started", StartDate: "2026-10-01"})
				createTodo(t, svc, todo.CreateParams{Summary: "In process", Status: "IN-PROCESS"})
			},
			want: []todoKey{{"Dated", "2026-10-20"}, {"In process", ""}, {"Started", ""}, {"Undated", ""}},
		},
		{
			name: "completed and cancelled non-recurring todos are not open",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Done", Status: "COMPLETED"})
				createTodo(t, svc, todo.CreateParams{Summary: "Dropped", Status: "CANCELLED"})
				createTodo(t, svc, todo.CreateParams{Summary: "Open"})
			},
			want: []todoKey{{"Open", ""}},
		},
		{
			name: "recurring series shows only its first occurrence",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Daily", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY"})
			},
			want: []todoKey{{"Daily", "2026-10-10"}},
		},
		{
			name: "completed occurrences advance the series to the next open one",
			setup: func(t *testing.T, svc *todo.Service) {
				m := createTodo(t, svc, todo.CreateParams{Summary: "Daily", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY"})
				for _, day := range []string{"2026-10-10", "2026-10-11"} {
					upsertTodo(t, svc, todo.UpsertParams{UID: m.UID, Summary: "Daily", DueDate: day, RecurrenceID: day, Status: "COMPLETED"})
				}
			},
			want: []todoKey{{"Daily", "2026-10-12"}},
		},
		{
			name: "a cancelled occurrence is skipped",
			setup: func(t *testing.T, svc *todo.Service) {
				m := createTodo(t, svc, todo.CreateParams{Summary: "Daily", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY"})
				upsertTodo(t, svc, todo.UpsertParams{UID: m.UID, Summary: "Daily", DueDate: "2026-10-10", RecurrenceID: "2026-10-10", Status: "CANCELLED"})
			},
			want: []todoKey{{"Daily", "2026-10-11"}},
		},
		{
			name: "an EXDATE occurrence is skipped",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Daily", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY", ExDates: "2026-10-10T00:00:00Z"})
			},
			want: []todoKey{{"Daily", "2026-10-11"}},
		},
		{
			name: "an open override replaces its occurrence",
			setup: func(t *testing.T, svc *todo.Service) {
				m := createTodo(t, svc, todo.CreateParams{Summary: "Daily", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY"})
				upsertTodo(t, svc, todo.UpsertParams{UID: m.UID, Summary: "Daily moved", DueDate: "2026-10-15", RecurrenceID: "2026-10-10", Status: "IN-PROCESS"})
			},
			want: []todoKey{{"Daily moved", "2026-10-15"}},
		},
		{
			name: "a finished series has no open occurrence",
			setup: func(t *testing.T, svc *todo.Service) {
				m := createTodo(t, svc, todo.CreateParams{Summary: "Twice", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY;COUNT=2"})
				for _, day := range []string{"2026-10-10", "2026-10-11"} {
					upsertTodo(t, svc, todo.UpsertParams{UID: m.UID, Summary: "Twice", DueDate: day, RecurrenceID: day, Status: "COMPLETED"})
				}
			},
			want: nil,
		},
		{
			name: "completed and cancelled recurring masters have no open occurrence",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Done series", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY", Status: "COMPLETED"})
				createTodo(t, svc, todo.CreateParams{Summary: "Dropped series", DueDate: "2026-10-10", RecurrenceRule: "FREQ=DAILY", Status: "CANCELLED"})
			},
			want: nil,
		},
		{
			name: "a recurring series without DUE uses its DTSTART",
			setup: func(t *testing.T, svc *todo.Service) {
				createTodo(t, svc, todo.CreateParams{Summary: "Weekly", StartDate: "2026-10-05", RecurrenceRule: "FREQ=WEEKLY"})
			},
			want: []todoKey{{"Weekly", ""}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, q := testutil.NewTestDB(t)
			tt.setup(t, todo.NewService(db, q))

			open, err := NewService(db, q).ListOpenTodos(context.Background())
			if err != nil {
				t.Fatalf("ListOpenTodos: %v", err)
			}
			todos := make([]todo.Todo, len(open))
			for i, o := range open {
				todos[i] = o.Todo
			}
			assertTodoKeys(t, todos, tt.want)
		})
	}
}

// TestListOpenTodos_OccurrenceIdentity checks that the current occurrence
// carries the master ID, the shifted dates, and its instance time. The later
// complete action needs all three to write the override for that occurrence.
func TestListOpenTodos_OccurrenceIdentity(t *testing.T) {
	db, q := testutil.NewTestDB(t)
	svc := todo.NewService(db, q)
	m := createTodo(t, svc, todo.CreateParams{
		Summary:        "Weekly",
		StartDate:      "2026-10-05T09:00:00Z",
		DueDate:        "2026-10-05T17:00:00Z",
		RecurrenceRule: "FREQ=WEEKLY",
	})
	upsertTodo(t, svc, todo.UpsertParams{UID: m.UID, Summary: "Weekly", StartDate: "2026-10-05T09:00:00Z", DueDate: "2026-10-05T17:00:00Z", RecurrenceID: "2026-10-05T09:00:00Z", Status: "COMPLETED"})

	open, err := NewService(db, q).ListOpenTodos(context.Background())
	if err != nil {
		t.Fatalf("ListOpenTodos: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("got %d open todos, want 1", len(open))
	}
	got := open[0]
	wantInstance := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	if got.ID != m.ID || got.IsOverride || !got.InstanceTime.Equal(wantInstance) {
		t.Fatalf("occurrence = ID %d override %v instance %v, want ID %d override false instance %v",
			got.ID, got.IsOverride, got.InstanceTime, m.ID, wantInstance)
	}
	if got.StartDate != "2026-10-12T09:00:00Z" || got.DueDate != "2026-10-12T17:00:00Z" {
		t.Fatalf("dates = %s..%s, want 2026-10-12T09:00:00Z..2026-10-12T17:00:00Z", got.StartDate, got.DueDate)
	}
}

func TestListCompletedTodos(t *testing.T) {
	db, q := testutil.NewTestDB(t)
	svc := todo.NewService(db, q)
	completedAt := "2026-10-14T12:00:00Z"
	upsertTodo(t, svc, todo.UpsertParams{UID: "due-in", Summary: "Due in range", DueDate: "2026-10-12", Status: "COMPLETED", CompletedAt: completedAt})
	upsertTodo(t, svc, todo.UpsertParams{UID: "due-out", Summary: "Due before range", DueDate: "2026-09-30", Status: "COMPLETED", CompletedAt: completedAt})
	upsertTodo(t, svc, todo.UpsertParams{UID: "timed-late", Summary: "Timed after range end", DueDate: "2026-10-31T23:30:00Z", Status: "COMPLETED", CompletedAt: completedAt})
	upsertTodo(t, svc, todo.UpsertParams{UID: "undated-in", Summary: "Undated done in range", Status: "COMPLETED", CompletedAt: completedAt})
	upsertTodo(t, svc, todo.UpsertParams{UID: "undated-out", Summary: "Undated done before range", Status: "COMPLETED", CompletedAt: "2026-09-01T12:00:00Z"})
	upsertTodo(t, svc, todo.UpsertParams{UID: "open", Summary: "Open in range", DueDate: "2026-10-12"})
	upsertTodo(t, svc, todo.UpsertParams{UID: "cancelled", Summary: "Cancelled in range", DueDate: "2026-10-12", Status: "CANCELLED"})
	master := upsertTodo(t, svc, todo.UpsertParams{UID: "series", Summary: "Series", DueDate: "2026-10-01", RecurrenceRule: "FREQ=DAILY", Status: "COMPLETED", CompletedAt: completedAt})
	upsertTodo(t, svc, todo.UpsertParams{UID: master.UID, Summary: "Series occurrence", DueDate: "2026-10-13", RecurrenceID: "2026-10-13", Status: "COMPLETED", CompletedAt: completedAt})

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)
	got, err := NewService(db, q).ListCompletedTodos(context.Background(), from, to)
	if err != nil {
		t.Fatalf("ListCompletedTodos: %v", err)
	}
	assertTodoKeys(t, got, []todoKey{
		{"Due in range", "2026-10-12"},
		{"Series occurrence", "2026-10-13"},
		{"Undated done in range", ""},
	})
}
