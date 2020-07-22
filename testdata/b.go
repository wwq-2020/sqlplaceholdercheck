package testdata

import (
	"context"
)

func (t a) Query(ctx context.Context) {
	t.db.Query("select id from t where id > ? limit ?", 1, 2)
}
