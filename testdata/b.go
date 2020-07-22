package testdata
import (
	"context"
)

func (t a) Query(ctx context.Context) {
	t.db.QueryRow("select id from t where id = ?", 1, 2, 3)
}