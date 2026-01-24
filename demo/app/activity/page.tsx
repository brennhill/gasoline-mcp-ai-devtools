"use client";

import { useState, useEffect } from "react";

// BUG: POST payload sends user_id as string instead of number
// Server expects { user_id: 123 } but client sends { user_id: "123" }
// API returns 422 with validation error

interface ActivityEvent {
  id: number;
  user: string;
  action: string;
  timestamp: string;
  status: "success" | "failed";
}

export default function ActivityPage() {
  const [events, setEvents] = useState<ActivityEvent[]>([
    { id: 1, user: "Alice", action: "Logged in", timestamp: "2 min ago", status: "success" },
    { id: 2, user: "Bob", action: "Updated profile", timestamp: "5 min ago", status: "success" },
    { id: 3, user: "Carol", action: "Created project", timestamp: "12 min ago", status: "success" },
  ]);
  const [posting, setPosting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function trackEvent() {
    setPosting(true);
    setError(null);

    try {
      const res = await fetch("/api/events", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          // BUG: user_id should be a number but is sent as string
          user_id: "42",
          action: "page_view",
          page: "/activity",
          timestamp: new Date().toISOString(),
        }),
      });

      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || `HTTP ${res.status}`);
      }

      // Add to local list
      setEvents((prev) => [
        {
          id: Date.now(),
          user: "Demo User",
          action: "Tracked page view",
          timestamp: "Just now",
          status: "success",
        },
        ...prev,
      ]);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
      setEvents((prev) => [
        {
          id: Date.now(),
          user: "Demo User",
          action: "Track event failed",
          timestamp: "Just now",
          status: "failed",
        },
        ...prev,
      ]);
    } finally {
      setPosting(false);
    }
  }

  // Auto-track on mount to trigger the bug immediately
  useEffect(() => {
    trackEvent();
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-white">Activity Feed</h2>
        <button
          data-testid="track-event"
          onClick={trackEvent}
          disabled={posting}
          className="px-4 py-2 bg-orange-500 hover:bg-orange-600 disabled:bg-slate-600 text-white rounded-lg text-sm font-medium transition-colors"
        >
          {posting ? "Tracking..." : "Track Event"}
        </button>
      </div>

      {error && (
        <div data-testid="error-message" className="mb-4 p-4 bg-red-900/30 border border-red-700 rounded-lg text-red-300 text-sm">
          Failed to track event: {error}
        </div>
      )}

      <div className="space-y-3">
        {events.map((event) => (
          <div key={event.id} className="bg-slate-800 rounded-lg p-4 border border-slate-700 flex items-center gap-4">
            <div className={`w-2 h-2 rounded-full ${
              event.status === "success" ? "bg-green-400" : "bg-red-400"
            }`} />
            <div className="flex-1">
              <p className="text-sm text-white">
                <span className="font-medium">{event.user}</span>
                {" — "}
                {event.action}
              </p>
            </div>
            <span className="text-xs text-slate-400">{event.timestamp}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
