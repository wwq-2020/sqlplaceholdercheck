package testdata

import (
	"context"
	"database/sql"
)

type a struct {
	db *sql.DB
}

func (t a) Query(ctx context.Context) {
	t.db.Query("select id from t where id = ?", 1, 2, 3)
}
