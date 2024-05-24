# chip chat

[![Build Status](https://github.com/nyaruka/chip/workflows/CI/badge.svg)](https://github.com/nyaruka/chip/actions?query=workflow%3ACI) 

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

### `get_history`

Requests message history for the current contact:

```json
{
    "type": "get_history",
    "before": "2024-05-01T17:15:30.123456Z"
}
```

Server will repond with a `history` event.

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
    "time": "2024-05-01T17:15:30.123456Z",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

### `chat_resumed`

A chat session for an existing contact has been successfully resumed:

```json
{
    "type": "chat_resumed",
    "time": "2024-05-01T17:15:30.123456Z",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO",
    "email": "bob@nyaruka.com"
}
```

### `msg_out`

A new outgoing message has been created and should be displayed in the client:

```json
{
    "type": "msg_out",
    "time": "2024-05-01T17:15:30.123456Z",
    "msg_id": 34634,
    "text": "Thanks for contacting us!",
    "origin": "flow"
}
```

```json
{
    "type": "msg_out",
    "time": "2024-05-01T17:15:30.123456Z",
    "msg_id": 34634,
    "text": "How can we help?",
    "origin": "chat",
    "user": {"name": "Bob McTickets", "email": "bob@nyaruka.com"}
}
```

### `history`

The client previously requested history with a `get_history` command:

```json
{
    "type": "history",
    "time": "2024-05-01T17:15:30.123456Z",
    "history": [
        {
            "type": "msg_in",
            "time": "2024-04-01T13:15:30.123456Z",
            "msg_id": 34632,
            "text": "I need help!"
        },
        {
            "type": "msg_out",
            "time": "2024-04-01T13:15:30.123456Z",
            "msg_id": 34634,
            "text": "Thanks for contacting us!",
            "origin": "flow"
        }
    ]
}
```
