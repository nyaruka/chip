package models

import (
	"context"
	"encoding/json"
	"fmt"
)

// the type code of webchat channels on the platform
const ChannelTypeWebChat = "WCH"

// the channel config keys chip reads
const (
	// the host[:port] values (no scheme) of the sites the widget may be embedded on - empty or absent means
	// unrestricted
	ConfigAllowedDomains = "allowed_domains"
)

// ChannelConfig is a channel's JSON config as the platform stores it
type ChannelConfig map[string]any

func (c *ChannelConfig) Scan(value any) error {
	if value == nil {
		*c = ChannelConfig{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan channel config: expected []byte, got %T", value)
	}
	return json.Unmarshal(b, c)
}

// Channel is a webchat channel as chip sees it: an address for the widget's requests, a workspace to scope
// visitors and their uploads to, and the config the workspace has given it
type Channel struct {
	ID_     ChannelID     `db:"id"`
	UUID_   ChannelUUID   `db:"uuid"`
	OrgID_  OrgID         `db:"org_id"`
	Config_ ChannelConfig `db:"config"`
}

func (c *Channel) ID() ChannelID     { return c.ID_ }
func (c *Channel) UUID() ChannelUUID { return c.UUID_ }
func (c *Channel) OrgID() OrgID      { return c.OrgID_ }

// AllowedDomains returns the channel's allowed_domains config as a list of host[:port] strings
func (c *Channel) AllowedDomains() []string {
	vals, _ := c.Config_[ConfigAllowedDomains].([]any)
	domains := make([]string, 0, len(vals))
	for _, val := range vals {
		if s, ok := val.(string); ok {
			domains = append(domains, s)
		}
	}
	return domains
}

const sqlSelectChannel = `
SELECT id, uuid, org_id, config
  FROM channels_channel
 WHERE uuid = $1 AND channel_type = $2 AND is_active`

// LoadChannel loads the active webchat channel with the given UUID, or nil if there isn't one
func LoadChannel(ctx context.Context, db DBorTx, uuid ChannelUUID) (*Channel, error) {
	return getOne[Channel](ctx, db, sqlSelectChannel, uuid, ChannelTypeWebChat)
}

// ChatSocket returns the realtime pub/sub socket of a conversation ("chat:<channel-uuid>:<chat-id>") - what the
// visitor's browser subscribes to, and what everything the visitor is shown live is published to. ("Socket" is the
// platform's name for a realtime pub/sub address, so as not to overload channel.)
func ChatSocket(channelUUID ChannelUUID, chatID string) string {
	return fmt.Sprintf("chat:%s:%s", channelUUID, chatID)
}
