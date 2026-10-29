package models

import (
	"context"
	"time"
)

// Org is a workspace as chip shows it to visitors: by its name, and in its timezone
type Org struct {
	ID_       OrgID  `db:"id"`
	Name_     string `db:"name"`
	Timezone_ string `db:"timezone"`
}

func (o *Org) ID() OrgID    { return o.ID_ }
func (o *Org) Name() string { return o.Name_ }

// Timezone returns the workspace's timezone, or UTC if it isn't one we know
func (o *Org) Timezone() *time.Location {
	tz, err := time.LoadLocation(o.Timezone_)
	if err != nil {
		return time.UTC
	}
	return tz
}

const sqlSelectOrg = `
SELECT id, name, timezone
  FROM orgs_org
 WHERE id = $1 AND is_active`

// LoadOrg loads the active workspace with the given ID, or nil if there isn't one
func LoadOrg(ctx context.Context, db DBorTx, id OrgID) (*Org, error) {
	return getOne[Org](ctx, db, sqlSelectOrg, id)
}
