#!/usr/bin/env node

import http from "node:http";
import { createHash } from "node:crypto";

const PORT = Number(process.env.PORT || 8787);
const WS_PATH = "/third-party-feed";
const WS_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

function createAcceptValue(key) {
  return createHash("sha1").update(key + WS_GUID).digest("base64");
}

function encodeTextFrame(message) {
  const payload = Buffer.from(message);
  const length = payload.length;

  if (length < 126) {
    return Buffer.concat([Buffer.from([0x81, length]), payload]);
  }

  if (length < 65536) {
    const header = Buffer.alloc(4);
    header[0] = 0x81;
    header[1] = 126;
    header.writeUInt16BE(length, 2);
    return Buffer.concat([header, payload]);
  }

  throw new Error("Payload too large for demo websocket frame");
}

function send(socket, data) {
  socket.write(encodeTextFrame(data));
}

const server = http.createServer((req, res) => {
  if (req.url === "/") {
    res.writeHead(200, { "content-type": "text/plain; charset=utf-8" });
    res.end("Third-party websocket demo server is running.\n");
    return;
  }

  res.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
  res.end("Not found\n");
});

server.on("upgrade", (req, socket) => {
  if (req.url !== WS_PATH) {
    socket.write("HTTP/1.1 404 Not Found\r\n\r\n");
    socket.destroy();
    return;
  }

  const key = req.headers["sec-websocket-key"];
  if (!key || Array.isArray(key)) {
    socket.write("HTTP/1.1 400 Bad Request\r\n\r\n");
    socket.destroy();
    return;
  }

  const acceptValue = createAcceptValue(key);
  socket.write(
    [
      "HTTP/1.1 101 Switching Protocols",
      "Upgrade: websocket",
      "Connection: Upgrade",
      `Sec-WebSocket-Accept: ${acceptValue}`,
      "",
      "",
    ].join("\r\n")
  );

  const sequence = [
    JSON.stringify({
      event_name: "status_update",
      source: "vendor_stream_v2",
      data: ["checkout", "degraded", 1739649001],
    }),
    JSON.stringify({
      event_name: "status_update",
      source: "vendor_stream_v2",
      data: ["checkout", "critical", 1739649002],
    }),
    JSON.stringify({
      event_name: "operator_note",
      source: "vendor_stream_v2",
      data: ["bridge ok", "payload schema changed recently"],
    }),
    "MALFORMED_TEXT_MESSAGE_FROM_VENDOR",
  ];

  let index = 0;
  const timer = setInterval(() => {
    send(socket, sequence[index % sequence.length]);
    index += 1;
  }, 1300);

  socket.on("error", () => {
    clearInterval(timer);
  });

  socket.on("close", () => {
    clearInterval(timer);
  });

  socket.on("end", () => {
    clearInterval(timer);
  });
});

server.listen(PORT, () => {
  console.log(`third-party websocket demo server listening on ws://localhost:${PORT}${WS_PATH}`);
});
