# Echo Example

This directory contains a echo server example using nhooyr.io/websocket.

```bash
$ make run-local-instance
...
listening on http://[::]:8000
```

You can use a WebSocket client like wscat to connect. All messages written will
be echoed back.

```bash
$ npm install -g wscat
```

```bash
$ wscat -c ws://127.0.01:8000 -s echo
```

To talk to the version in production you can use:

```
$ wscat -c wss://websockets.skia.org -s echo
```

## Structure

The server is in `server.go` and is implemented as a `http.HandlerFunc` that
accepts the WebSocket and then reads all messages and writes them exactly as is
back to the connection.

`server_test.go` contains a small unit test to verify it works correctly.

`main.go` brings it all together so that you can run it and play around with it.
