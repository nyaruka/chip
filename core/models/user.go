package models

import (
	"database/sql"
	"strings"

	"github.com/nyaruka/chip/v26/runtime"
)

// User is a platform user as a message's sender is shown to the visitor - just the name and avatar the chat
// renders
type User struct {
	ID_        UserID         `db:"id"`
	UUID_      UserUUID       `db:"uuid"`
	FirstName_ string         `db:"first_name"`
	LastName_  string         `db:"last_name"`
	Avatar_    sql.NullString `db:"avatar"` // the path of the avatar in public storage, if they have one
}

func (u *User) ID() UserID     { return u.ID_ }
func (u *User) UUID() UserUUID { return u.UUID_ }

// Name returns the user's full name, as the platform shows it
func (u *User) Name() string {
	return strings.TrimSpace(u.FirstName_ + " " + u.LastName_)
}

// AvatarURL returns the URL of the user's avatar in public storage, or an empty string if they don't have one
func (u *User) AvatarURL(rt *runtime.Runtime) string {
	if !u.Avatar_.Valid || u.Avatar_.String == "" {
		return ""
	}
	return rt.S3.ObjectURL(rt.Config.S3PublicBucket, u.Avatar_.String)
}
