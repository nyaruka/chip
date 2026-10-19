// Package models reads what chip needs from the platform's database: webchat channels, the visitors chatting on
// them, and their conversations. It only ever reads - contacts, messages and statuses are written by courier.
package models

import (
	"context"
	"database/sql"
	"fmt"
)

type OrgID int
type ChannelID int
type ChannelUUID string
type ContactID int
type URNID int
type UserID int
type UserUUID string
type MsgUUID string

// DBorTx is the interface for a database or transaction
type DBorTx interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
}

// getOne loads a single row into the given struct, returning nil if there's no such row
func getOne[T any](ctx context.Context, db DBorTx, query string, args ...any) (*T, error) {
	v := new(T)
	if err := db.GetContext(ctx, v, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("error loading %T: %w", v, err)
	}
	return v, nil
}
