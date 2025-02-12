package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/null/v2"
)

type UserID null.Int

const NilUserID = UserID(0)

type User struct {
	ID     UserID `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

const sqlSelectUser = `
SELECT row_to_json(r) FROM (
    SELECT id, email, TRIM(CONCAT(first_name, ' ', last_name)) AS name, avatar
    FROM users_user 
    WHERE id = $1 AND is_active
) r`

func LoadUser(ctx context.Context, rt *runtime.Runtime, id UserID) (*User, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectUser, id)
	if err != nil {
		return nil, fmt.Errorf("error querying user: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	u := &User{}
	if err := dbutil.ScanJSON(rows, u); err != nil {
		return nil, fmt.Errorf("error scanning user: %w", err)
	}

	if u.Avatar != "" {
		u.Avatar = rt.Config.StorageURL + u.Avatar
	}

	return u, nil
}
