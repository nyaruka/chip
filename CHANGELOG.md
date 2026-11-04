## v26.3.4 (2026-11-04)
 * Add channel email config, Mailgun client and unread message email rendering
 * Refuse unknown Mailgun regions and anything but a single recipient, and name attachments from their decoded path
 * Email webchat visitors the messages they didn't read once their conversation goes quiet
 * Email visitors who don't have the chat open almost straight away
 * Only ever move a conversation's due time later, so holding back a message doesn't cut a retry's backoff short
 * Check presence inside the hold script, so a failed check can't lose the hold

## v26.3.3 (2026-10-28)
 * Default to Valkey DB 7 since chip's keys are its own
 * Fix the Licensed Work name in LICENSE

## v26.3.2 (2026-10-20)
 * Update vkutil to v0.30.0 for default Valkey connection timeouts

## v26.3.1 (2026-10-19)
 * Add webchat channel type that posts to courier's CHP channel endpoints

## v26.3.0 (2026-10-19)
 * Add initial webchat service with widget endpoints, courier client, and send/socket proxies
 * Serve the internal endpoints under /wi/

