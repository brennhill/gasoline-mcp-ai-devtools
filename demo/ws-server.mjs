// Standalone WebSocket server for the notifications demo
// BUG: Disconnects clients after 10 seconds with code 1006 (abnormal closure)

import { WebSocketServer } from "ws";

const PORT = 3001;
const wss = new WebSocketServer({ port: PORT });

const notifications = [
  { type: "info", message: "New user signed up: frank@example.com" },
  { type: "success", message: "Deployment completed successfully" },
  { type: "info", message: "Report generated: Q4 Revenue Analysis" },
  { type: "error", message: "Failed to send email to bob@example.com" },
  { type: "success", message: "Database backup completed" },
];

let notifId = 1;

wss.on("connection", (ws) => {
  console.log("[WS] Client connected");
  let messageIndex = 0;

  // Send notifications every 2-3 seconds
  const interval = setInterval(() => {
    if (messageIndex >= notifications.length) {
      messageIndex = 0;
    }

    const notif = {
      id: notifId++,
      ...notifications[messageIndex],
      time: new Date().toLocaleTimeString(),
    };

    ws.send(JSON.stringify(notif));
    messageIndex++;
  }, 2500);

  // BUG: Force disconnect after 10 seconds with abnormal closure
  const disconnectTimer = setTimeout(() => {
    console.log("[WS] Forcing abnormal disconnect (code 1006 simulation)");
    clearInterval(interval);
    // Destroy the socket without sending a close frame — triggers code 1006 on client
    ws.terminate();
  }, 10000);

  ws.on("close", () => {
    console.log("[WS] Client disconnected");
    clearInterval(interval);
    clearTimeout(disconnectTimer);
  });
});

console.log(`[WS] Notification server running on ws://localhost:${PORT}`);
