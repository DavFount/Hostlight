package main

import "fmt"

type ActiveOutage struct {
	ID              string `json:"id"`
	DurationSeconds int64  `json:"durationSeconds"`
}

// GetActiveOutages measures only through the last recorded observation, never
// extrapolating wall-clock time during pauses or monitoring gaps.
func (a *App) GetActiveOutages() ([]ActiveOutage, error) {
	if a.history == nil {
		return nil, fmt.Errorf("History storage is unavailable")
	}
	rows, err := a.history.db.Query("SELECT target_id, MAX(0,(last_seen-opened-excluded_ms)/1000) FROM incidents WHERE status='open'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ActiveOutage{}
	for rows.Next() {
		var item ActiveOutage
		if err := rows.Scan(&item.ID, &item.DurationSeconds); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
