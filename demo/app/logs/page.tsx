"use client";

import { useState, useEffect, useRef } from "react";

// This page generates verbose console output at all levels.
// Demonstrates observe/logs and configure/noise_rule tools.

export default function LogsPage() {
  const [logEntries, setLogEntries] = useState<Array<{id: number; level: string; message: string; time: string}>>([]);
  const [isStreaming, setIsStreaming] = useState(true);
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    // Initial burst of logs at different levels
    console.log("[App] LogsPage mounted — streaming mode active");
    console.info("[System] Runtime version: 2.4.1, environment: development");
    console.warn("[Deprecation] `legacyMode` will be removed in v3.0");
    console.debug("[Debug] Component render cycle #1, props: {}");

    // BUG: Noisy polling loop that floods the console
    let count = 0;
    intervalRef.current = setInterval(() => {
      count++;
      const level = getLogLevel(count);
      const message = generateLogMessage(count, level);

      // Actually log to console so Gasoline captures it
      switch (level) {
        case "error":
          console.error(message);
          break;
        case "warn":
          console.warn(message);
          break;
        case "info":
          console.info(message);
          break;
        case "debug":
          console.debug(message);
          break;
        default:
          console.log(message);
      }

      setLogEntries(prev => [{
        id: count,
        level,
        message,
        time: new Date().toLocaleTimeString(),
      }, ...prev].slice(0, 50));
    }, 2000);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  function toggleStreaming() {
    if (isStreaming) {
      if (intervalRef.current) clearInterval(intervalRef.current);
      console.log("[App] Log streaming paused by user");
    } else {
      console.log("[App] Log streaming resumed");
      // Restart with new interval
      let count = logEntries.length;
      intervalRef.current = setInterval(() => {
        count++;
        const level = getLogLevel(count);
        const message = generateLogMessage(count, level);
        switch (level) {
          case "error": console.error(message); break;
          case "warn": console.warn(message); break;
          case "info": console.info(message); break;
          default: console.log(message);
        }
        setLogEntries(prev => [{
          id: count, level, message, time: new Date().toLocaleTimeString(),
        }, ...prev].slice(0, 50));
      }, 2000);
    }
    setIsStreaming(!isStreaming);
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">System Logs</h2>

      {/* Controls */}
      <div className="flex gap-4 mb-6">
        <button
          onClick={toggleStreaming}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            isStreaming
              ? "bg-red-600 hover:bg-red-700 text-white"
              : "bg-green-600 hover:bg-green-700 text-white"
          }`}
        >
          {isStreaming ? "Pause Streaming" : "Resume Streaming"}
        </button>
        <button
          onClick={() => {
            // Trigger burst of errors
            console.error("[CRITICAL] Database connection pool exhausted");
            console.error("[CRITICAL] Request timeout after 30000ms");
            console.error("[CRITICAL] Memory usage exceeded 90% threshold");
            console.warn("[Recovery] Attempting reconnection...");
          }}
          className="px-4 py-2 bg-orange-600 hover:bg-orange-700 rounded-lg text-sm font-medium text-white transition-colors"
        >
          Trigger Error Burst
        </button>
        <button
          onClick={() => {
            // Trigger noisy repetitive logs (good for noise filter demo)
            for (let i = 0; i < 10; i++) {
              console.log(`[Heartbeat] Connection alive — ping ${Date.now()}`);
            }
          }}
          className="px-4 py-2 bg-slate-600 hover:bg-slate-700 rounded-lg text-sm font-medium text-white transition-colors"
        >
          Flood Heartbeats
        </button>
      </div>

      {/* Log level legend */}
      <div className="flex gap-4 mb-4 text-xs">
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-400" /> error</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-yellow-400" /> warn</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-blue-400" /> info</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-slate-400" /> log</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-purple-400" /> debug</span>
      </div>

      {/* Live log viewer */}
      <div className="bg-slate-900 rounded-xl border border-slate-700 overflow-hidden font-mono text-xs">
        <div className="px-4 py-2 bg-slate-800 border-b border-slate-700 flex justify-between items-center">
          <span className="text-slate-400">Live Console Output</span>
          <span className={`flex items-center gap-1 ${isStreaming ? "text-green-400" : "text-slate-500"}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${isStreaming ? "bg-green-400 animate-pulse" : "bg-slate-500"}`} />
            {isStreaming ? "Streaming" : "Paused"}
          </span>
        </div>
        <div className="p-4 max-h-96 overflow-y-auto space-y-1">
          {logEntries.length === 0 ? (
            <p className="text-slate-500">Waiting for logs...</p>
          ) : (
            logEntries.map((entry) => (
              <div key={entry.id} className="flex gap-2">
                <span className="text-slate-600 w-20 flex-shrink-0">{entry.time}</span>
                <span className={`w-12 flex-shrink-0 ${levelColor(entry.level)}`}>[{entry.level.toUpperCase().padEnd(5)}]</span>
                <span className="text-slate-300">{entry.message}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function getLogLevel(n: number): string {
  if (n % 17 === 0) return "error";
  if (n % 7 === 0) return "warn";
  if (n % 3 === 0) return "info";
  if (n % 11 === 0) return "debug";
  return "log";
}

function generateLogMessage(n: number, level: string): string {
  const messages: Record<string, string[]> = {
    error: [
      "[API] POST /api/users failed: connection refused",
      "[Worker] Job processing timeout exceeded",
      "[Auth] Token validation failed for session abc123",
      "[DB] Query execution exceeded 5000ms limit",
    ],
    warn: [
      "[Cache] Miss rate above 30% — consider warming",
      "[Memory] Heap usage at 78% — approaching threshold",
      "[API] Rate limit approaching for client_id=demo",
      "[Deprecation] `ctx.legacy` is deprecated, use `ctx.v2`",
    ],
    info: [
      "[Server] Request processed in 42ms",
      "[Queue] 3 jobs pending, 12 completed",
      "[Auth] User demo@whatsaas.io authenticated",
      "[Metrics] CPU: 34%, Memory: 2.1GB, Uptime: 4h22m",
    ],
    debug: [
      "[React] Component re-render triggered by props change",
      "[Router] Navigation to /logs completed",
      "[Cache] Hit for key: user:demo:profile",
    ],
    log: [
      `[Heartbeat] Connection alive — ping ${Date.now()}`,
      "[System] Background sync completed",
      "[Polling] Checking for updates...",
      "[App] State snapshot saved",
    ],
  };
  const pool = messages[level] || messages.log;
  return pool[n % pool.length];
}

function levelColor(level: string): string {
  switch (level) {
    case "error": return "text-red-400";
    case "warn": return "text-yellow-400";
    case "info": return "text-blue-400";
    case "debug": return "text-purple-400";
    default: return "text-slate-400";
  }
}
