# 🍟 Chip

[![Build Status](https://github.com/nyaruka/chip/workflows/CI/badge.svg)](https://github.com/nyaruka/chip/actions?query=workflow%3ACI)

Webchat service for [RapidPro](https://rapidpro.io) and [TextIt](https://textit.com).

Chip is what the chat widget embedded on a website talks to. It gives a new visitor their chat, takes the messages
they send and the files they attach, answers with the conversation so far when the widget reconnects, and delivers
what the platform sends back to them over the realtime server. Behind it, [courier](https://github.com/nyaruka/courier)
treats chip like any other messaging provider: chip posts visitors' messages and read reports to courier's webchat
channel endpoints, and courier posts the platform's outgoing messages and typing to chip.

Chip reads the platform's database - channels, the visitors chatting on them, their conversations - but never writes
to it. Everything a visitor does that changes state goes through courier.

## Running

```
go build ./cmd/chip
./chip -help
```

Configuration is by environment variables prefixed `CHIP_` (or a `chip.toml`); `-help` describes the settings. Two
listeners:

- **public** (8070) — `/wc/<channel-uuid>/*`, the endpoints the widget calls: `start`, `receive`, `status`,
  `history` and `upload`, all with CORS preflights since the widget runs on third-party sites
- **internal** (8071) — `/chi/*`, for our own services: courier's `send` and `event`, and the realtime server's
  `socket/subscribe` and `socket/sub_refresh` proxies which authorize a widget's chat socket subscription

The internal listener is gated on a shared token, on top of whatever network isolation keeps it off the internet.

## Testing

The suite is integration-level: it needs a Postgres it can create databases in, a Valkey, an S3 it can create buckets
in, and `pg_restore` on the path to load its `testsuite/testdata/postgres.dump`. Courier and the realtime server are
mocked.

```
go test ./...
```
