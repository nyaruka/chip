package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dbutil"
	"github.com/nyaruka/null/v2"
)

type UserID null.Int

const NilUserID = UserID(0)

type User struct {
	ID        UserID `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Avatar    string `json:"avatar"`
}

func (u *User) Name() string { return strings.TrimSpace(u.FirstName + " " + u.LastName) }

func (u *User) AvatarURL(cfg *runtime.Config) string {
	if u.Avatar != "" {
		return cfg.StorageURL + u.Avatar
	}
	return ""
}

const sqlSelectUser = `
SELECT row_to_json(r) FROM (
	SELECT u.id, u.email, u.first_name, u.last_name, s.avatar
	FROM auth_user u
	INNER JOIN orgs_usersettings s ON s.user_id = u.id
	WHERE u.id = $1 AND u.is_active
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
	return u, nil
}
