package repository

import (
	"context"
	"database/sql"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// HeatmapRepository handles heatmap-related database queries.
type HeatmapRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewHeatmapRepository creates a new HeatmapRepository.
func NewHeatmapRepository(db *sql.DB, nrApp *newrelic.Application) *HeatmapRepository {
	return &HeatmapRepository{db: db, nrApp: nrApp}
}

// GetGroupHeatmap returns the activity heatmap for a chat in the specified timezone.
func (r *HeatmapRepository) GetGroupHeatmap(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-group-heatmap")()

	tzName := "UTC"
	if tz != nil {
		tzName = tz.String()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time query with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date AT TIME ZONE $2)::int as day_of_week,
				EXTRACT(HOUR FROM m.date AT TIME ZONE $2)::int as hour,
				COUNT(*) as message_count,
				COUNT(DISTINCT m.user_id) as unique_users
			FROM messages m
			WHERE m.chat_id = $1
			GROUP BY
				EXTRACT(DOW FROM m.date AT TIME ZONE $2),
				EXTRACT(HOUR FROM m.date AT TIME ZONE $2)
			ORDER BY day_of_week, hour
		`, chatID, tzName)
	} else {
		// Date-filtered query with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date AT TIME ZONE $4)::int as day_of_week,
				EXTRACT(HOUR FROM m.date AT TIME ZONE $4)::int as hour,
				COUNT(*) as message_count,
				COUNT(DISTINCT m.user_id) as unique_users
			FROM messages m
			WHERE m.chat_id = $1
				AND m.date >= $2
				AND m.date < $3
			GROUP BY
				EXTRACT(DOW FROM m.date AT TIME ZONE $4),
				EXTRACT(HOUR FROM m.date AT TIME ZONE $4)
			ORDER BY day_of_week, hour
		`, chatID, startDate, endDate, tzName)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	heatmap := &HeatmapData{
		Data: []HeatmapCell{},
	}

	for rows.Next() {
		var cell HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.MessageCount, &cell.UniqueUsers); err != nil {
			return nil, err
		}
		heatmap.Data = append(heatmap.Data, cell)
		heatmap.TotalMessages += cell.MessageCount
		if cell.MessageCount > heatmap.MaxCount {
			heatmap.MaxCount = cell.MessageCount
		}
	}

	return heatmap, rows.Err()
}

// GetUserHeatmap returns the activity heatmap for a specific user in the specified timezone.
func (r *HeatmapRepository) GetUserHeatmap(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-heatmap")()

	tzName := "UTC"
	if tz != nil {
		tzName = tz.String()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date AT TIME ZONE $3)::int as day_of_week,
				EXTRACT(HOUR FROM m.date AT TIME ZONE $3)::int as hour,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1 AND m.user_id = $2
			GROUP BY
				EXTRACT(DOW FROM m.date AT TIME ZONE $3),
				EXTRACT(HOUR FROM m.date AT TIME ZONE $3)
			ORDER BY day_of_week, hour
		`, chatID, userID, tzName)
	} else {
		// Date-filtered with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date AT TIME ZONE $5)::int as day_of_week,
				EXTRACT(HOUR FROM m.date AT TIME ZONE $5)::int as hour,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1
				AND m.user_id = $2
				AND m.date >= $3
				AND m.date < $4
			GROUP BY
				EXTRACT(DOW FROM m.date AT TIME ZONE $5),
				EXTRACT(HOUR FROM m.date AT TIME ZONE $5)
			ORDER BY day_of_week, hour
		`, chatID, userID, startDate, endDate, tzName)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	heatmap := &HeatmapData{
		Data: []HeatmapCell{},
	}

	for rows.Next() {
		var cell HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.MessageCount); err != nil {
			return nil, err
		}
		heatmap.Data = append(heatmap.Data, cell)
		heatmap.TotalMessages += cell.MessageCount
		if cell.MessageCount > heatmap.MaxCount {
			heatmap.MaxCount = cell.MessageCount
		}
	}

	return heatmap, rows.Err()
}
