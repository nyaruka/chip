# temba-chat

Webchat server that talks to [Courier](https://github.com/nyaruka/courier/).

To start chat session as new user:

```javascript
sock = new WebSocket("ws://localhost:8070/start?channel=a204047b-5224-4b8b-a328-08a538f1b3cb");

sock.onclose = function (event) {
    console.log("socket closed");
};
sock.onmessage = function (event) {
    console.log(event.data);
};
```

The first message the client will receive will be: 

```json
{
    "type": "chat_started",
    "identifier": "65vbbDAQCdPdEWlEhDGy4utO"
}
```

Messages from courier will be sent to the client as:

```json
{
    "type": "msg_out",
    "text": "Hello there!"
}
```

To send a message from the client use:

```javascript
sock.send('{"type": "msg_in", "text": "Thanks!"}');
```

To start chat session as existing user, pass the identifier to the start endpoint:

```javascript
sock = new WebSocket("ws://localhost:8070/start?channel=a204047b-5224-4b8b-a328-08a538f1b3cb&identifier=65vbbDAQCdPdEWlEhDGy4utO")
```
