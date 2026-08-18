package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobclaw/internal/database"
)

type EventRepository interface {
	Create(ctx context.Context, event Event) error
	ListByApplicationID(ctx context.Context, applicationID int64) ([]Event, error)
}

type SQLiteEventRepository struct {
	db *database.DB
}

func NewSQLiteEventRepository(db *database.DB) *SQLiteEventRepository {
	return &SQLiteEventRepository{db: db}
}

func (r *SQLiteEventRepository) Create(
	ctx context.Context,
	event Event,
) error {
	if event.ApplicationID <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO application_events (
			application_id,
			event_type,
			metadata
		)
		VALUES (?, ?, ?)
	`,
		event.ApplicationID,
		event.Type,
		event.Metadata,
	)
	if err != nil {
		return fmt.Errorf("create application event: %w", err)
	}

	return nil
}

func (r *SQLiteEventRepository) ListByApplicationID(
	ctx context.Context,
	applicationID int64,
) ([]Event, error) {
	if applicationID <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			application_id,
			event_type,
			COALESCE(metadata, ''),
			created_at
		FROM application_events
		WHERE application_id = ?
		ORDER BY id ASC
	`, applicationID)
	if err != nil {
		return nil, fmt.Errorf("list application events: %w", err)
	}
	defer rows.Close()

	var events []Event

	for rows.Next() {
		var (
			event     Event
			eventType string
			createdAt string
		)

		if err := rows.Scan(
			&event.ID,
			&event.ApplicationID,
			&eventType,
			&event.Metadata,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan application event: %w", err)
		}

		event.Type = EventType(eventType)

		parsed, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse application event created_at: %w",
				err,
			)
		}

		event.CreatedAt = parsed
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application events: %w", err)
	}

	if events == nil {
		events = []Event{}
	}

	return events, nil
}

// Keep database/sql referenced explicitly so this repository follows
// the same ErrNoRows conventions as the application repository.
var _ = sql.ErrNoRows

var _ = time.Time{}
