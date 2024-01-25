package models

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/null/v2"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/pkg/errors"
)

type UserID null.Int64

const NilUserID = UserID(0)

type User interface {
	ID() UserID
	Email() string
	Name() string
}

type user struct {
	ID_        UserID `json:"id"`
	Email_     string `json:"email"`
	FirstName_ string `json:"first_name"`
	LastName_  string `json:"last_name"`
}

func (u *user) ID() UserID    { return u.ID_ }
func (u *user) Email() string { return u.Email_ }
func (u *user) Name() string  { return strings.TrimSpace(u.FirstName_ + " " + u.LastName_) }

func (u *user) MarshalJSON() ([]byte, error) {
	type e struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	return json.Marshal(e{Email: u.Email(), Name: u.Name()})
}

const sqlSelectUser = `
SELECT row_to_json(r) FROM (
	SELECT id, email, first_name, last_name FROM auth_user WHERE id = $1 AND is_active
) r`

func LoadUser(ctx context.Context, rt *runtime.Runtime, id UserID) (User, error) {
	rows, err := rt.DB.QueryContext(ctx, sqlSelectUser, id)
	if err != nil {
		return nil, errors.Wrap(err, "error querying user")
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("user query returned no rows")
	}
	u := &user{}
	if err := dbutil.ScanJSON(rows, u); err != nil {
		return nil, errors.Wrap(err, "error scanning user")
	}
	return u, nil
}
