package testdata

import (
	"context"
)

func (t a) Query(ctx context.Context) {
	var id int64
	rows, _ := t.db.Query("select id from t where id > ?", 1)
	rows.Scan(&id, &id)
}
