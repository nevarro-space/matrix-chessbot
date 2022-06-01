//
// Stores the event IDs of the board images sent in reply to FENs.
//

package store

import (
	"database/sql"

	mid "maunium.net/go/mautrix/id"
)

type FenImageStore struct {
	DB *sql.DB
}

func (fs *FenImageStore) CreateTables() error {
	tx, err := fs.DB.Begin()
	if err != nil {
		return err
	}

	queries := []string{
		`
		CREATE TABLE IF NOT EXISTS sent_fen_board_events (
			room_id         TEXT,
			fen_event_id    TEXT,
			board_event_id  TEXT,
			PRIMARY KEY (room_id, fen_event_id)
		)
		`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(query); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (fs *FenImageStore) GetEventID(roomID mid.RoomID, fenEventID mid.EventID) mid.EventID {
	row := fs.DB.QueryRow(`
		SELECT board_event_id
		FROM sent_fen_board_events
		WHERE room_id = ?
			AND fen_event_id = ?
	`, roomID, fenEventID)

	var boardEventID string
	err := row.Scan(&boardEventID)
	if err == nil {
		return mid.EventID(boardEventID)
	}
	return mid.EventID("")
}

func (fs *FenImageStore) SetEventID(roomID mid.RoomID, fenEventID, boardEventID mid.EventID) error {
	_, err := fs.DB.Exec(`
		INSERT INTO sent_fen_board_events (room_id, fen_event_id, board_event_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, fen_event_id)
		DO UPDATE SET board_event_id=EXCLUDED.board_event_id
	`, roomID, fenEventID, boardEventID)
	return err
}
