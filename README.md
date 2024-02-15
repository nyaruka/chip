# temba-chat

[![Build Status](https://github.com/nyaruka/temba-chat/workflows/CI/badge.svg)](https://github.com/nyaruka/temba-chat/actions?query=workflow%3ACI) 

Webchat server that talks to [Courier](https://github.com/nyaruka/courier/).

To start chat session as new user:

```javascript
sock = new WebSocket("ws://localhost:8070/start/a204047b-5224-4b8b-a328-08a538f1b3cb/");

sock.onclose = function (event) {
    console.log("bye!");
};
sock.onmessage = function (event) {
    console.log(event.data);
};
```

## Server Events

The first message the client will receive will contain the new chat ID: 

```json
{
    "type": "chat_started",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

The client can store that identifier to reconnect as the same contact in future. Pass it as a param to the start endpoint:

```javascript
sock = new WebSocket("ws://localhost:8070/start/a204047b-5224-4b8b-a328-08a538f1b3cb/?chat_id=65vbbDAQCdPdEWlEhDGy4utO")
```

And in this case the first message the client will receive will be:

```json
{
    "type": "chat_resumed",
    "chat_id": "65vbbDAQCdPdEWlEhDGy4utO",
    "email": ""
}
```

Messages from courier are sent to the client as events that look like:

```json
{
    "type": "msg_out",
    "text": "Hello there!",
    "origin": "flow"
}
```

## Client Events

To send a message from the client, send a `msg_in` event to the socket, e.g.

```json
{
    "type": "msg_in", 
    "text": "Thanks!"
}
```

The client can collect an email address by sending a `email_added` event, e.g.

```json
{
    "type": "email_added", 
    "email": "bob@nyaruka.com"
}
```

