package models

import (
	"context"
	"strings"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/null/v2"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type UserID null.Int

const NilUserID = UserID(0)

type User struct {
	ID        UserID `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (u *User) Name() string { return strings.TrimSpace(u.FirstName + " " + u.LastName) }

const sqlSelectUser = `
SELECT row_to_json(r) FROM (
	SELECT id, email, first_name, last_name FROM auth_user WHERE id = $1 AND is_active
) r`

func LoadUser(ctx context.Context, rt *runtime.Runtime, id UserID) (*User, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectUser, id)
	if err != nil {
		return nil, errors.Wrap(err, "error querying user")
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("user query returned no rows")
	}
	u := &User{}
	if err := dbutil.ScanJSON(rows, u); err != nil {
		return nil, errors.Wrap(err, "error scanning user")
	}
	return u, nil
}
