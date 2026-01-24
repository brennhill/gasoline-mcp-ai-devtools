"use client";

import { useState, useEffect, useRef } from "react";

// BUG: WebSocket disconnects after 10 seconds with code 1006 (abnormal closure)
// No visible error — notifications just stop arriving silently

interface Notification {
  id: number;
  type: string;
  message: string;
  time: string;
}

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    connectWebSocket();
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  function connectWebSocket() {
    // Connect to standalone WS server on port 3001
    const ws = new WebSocket("ws://localhost:3001");
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        setNotifications((prev) => [data, ...prev]);
      } catch {
        // ignore parse errors
      }
    };

    ws.onclose = () => {
      // BUG: Silent failure — no error shown to user, notifications just stop
      setConnected(false);
    };

    ws.onerror = () => {
      setConnected(false);
    };
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-white">Notifications</h2>
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connected ? "bg-green-400" : "bg-red-400"}`} />
          <span className="text-sm text-slate-400">
            {connected ? "Live" : "Disconnected"}
          </span>
        </div>
      </div>

      <p className="text-slate-400 text-sm mb-6">
        Real-time notifications via WebSocket. Watch for new events below.
      </p>

      {notifications.length === 0 ? (
        <div className="bg-slate-800 rounded-xl p-12 border border-slate-700 text-center">
          <p className="text-slate-400">Waiting for notifications...</p>
          <p className="text-slate-500 text-sm mt-2">Events will appear here in real-time</p>
        </div>
      ) : (
        <div className="space-y-3">
          {notifications.map((notif) => (
            <div key={notif.id} className="bg-slate-800 rounded-lg p-4 border border-slate-700 flex items-start gap-3">
              <div className={`w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 ${
                notif.type === "error" ? "bg-red-900/50 text-red-300" :
                notif.type === "success" ? "bg-green-900/50 text-green-300" :
                "bg-blue-900/50 text-blue-300"
              }`}>
                {notif.type === "error" ? "!" : notif.type === "success" ? "✓" : "i"}
              </div>
              <div className="flex-1">
                <p className="text-white text-sm">{notif.message}</p>
                <p className="text-slate-400 text-xs mt-1">{notif.time}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
