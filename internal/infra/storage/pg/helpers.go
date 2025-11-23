package pg

import (
	"database/sql"
	"log"
)

func CloseRows(rows *sql.Rows) {
	if rows != nil {
		if err := rows.Close(); err != nil {
			log.Printf("ERROR closing rows: %v", err)
		}
	}
}
