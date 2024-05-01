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

```json
{
    "type": "chat_started",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

Or resume a chat session as an existing contact:

```json
{
    "type": "start_chat",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

```json
{
    "type": "chat_resumed",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO",
    "email": ""
}
```

### `create_msg`

Creates a new message from the client:

```json
{
    "type": "create_msg",
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
