sock = new WebSocket("ws://localhost:8070/start")

sock.onclose = function (event) {
    console.log("socket closed");
}

sock.onmessage = function (event) {
    console.log(event.data);
};
