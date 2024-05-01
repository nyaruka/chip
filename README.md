# temba-chat

[![Build Status](https://github.com/nyaruka/temba-chat/workflows/CI/badge.svg)](https://github.com/nyaruka/temba-chat/actions?query=workflow%3ACI) 

Webchat server that talks to [Courier](https://github.com/nyaruka/courier/).

To start chat session as new user:

```javascript
sock = new WebSocket("ws://localhost:8070/connect/7d62d551-3030-4100-a260-2d7c4e9693e7/");

sock.onclose = function (event) {
    console.log("bye!");
};
sock.onmessage = function (event) {
    console.log(event.data);
};
```

## Client Commands

### `start_chat`

Can be used to start a new chat session as a new contact:

```json
{
    "type": "start_chat"
}
```

Or resume a chat session as an existing contact:

```json
{
    "type": "start_chat",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

Server will respond with a `chat_started` or `chat_resumed` event depending on whether the provided chat ID matches an
existing contact.

### `send_msg`

Creates a new incoming message from the client:

```json
{
    "type": "send_msg",
    "text": "I need help!"
}
```

### `set_email`

Updates the email address for the current contact:

```json
{
    "type": "set_email",
    "email": "bob@nyaruka.com"
}
```

## Client Events

### `chat_started`

A chat session for a new contact has been successfully started:

```json
{
    "type": "chat_started",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

### `chat_resumed`

A chat session for an existing contact has been successfully resumed:

```json
{
    "type": "chat_resumed",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO",
    "email": "bob@nyaruka.com"
}
```

### `msg_created`

A new outgoing message has been created and should be displayed in the client:

```json
{
    "type": "msg_created",
    "text": "Thanks for contacting us!",
    "origin": "flow"
}
```

```json
{
    "type": "msg_created",
    "text": "How can we help?",
    "origin": "chat",
    "origin": "bob@nyaruka.com"
}
```