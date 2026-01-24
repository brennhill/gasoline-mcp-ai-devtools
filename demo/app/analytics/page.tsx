"use client";

import { useState, useEffect } from "react";

// BUG: This page loads multiple third-party tracking scripts and leaks PII
// to external analytics endpoints. Demonstrates CSP generator and security audit tools.

export default function AnalyticsPage() {
  const [events, setEvents] = useState<Array<{id: number; name: string; email: string; action: string; timestamp: string}>>([]);
  const [loading, setLoading] = useState(true);
  const [trackingStatus, setTrackingStatus] = useState("initializing");

  useEffect(() => {
    // BUG: Loads inline script that sends PII to third-party analytics
    const script = document.createElement("script");
    script.textContent = `
      (function() {
        // Simulated third-party tracker — sends PII to external origin
        var userData = {
          email: document.querySelector('[data-user-email]')?.getAttribute('data-user-email') || 'demo@whatsaas.io',
          name: 'Demo User',
          sessionId: 'sess_' + Math.random().toString(36).substr(2, 9)
        };
        // BUG: PII sent to third-party analytics endpoint
        fetch('https://analytics.tracker-cdn.example.com/collect', {
          method: 'POST',
          body: JSON.stringify(userData),
          mode: 'no-cors'
        }).catch(function() {});
        // BUG: Another tracker pixel with PII in URL
        new Image().src = 'https://pixel.adnetwork.example.com/track?email=' + encodeURIComponent(userData.email) + '&session=' + userData.sessionId;
        // BUG: Loading external script from untrusted CDN
        var s = document.createElement('script');
        s.src = 'https://cdn.sketchy-analytics.example.com/v2/tracker.js';
        s.onerror = function() { console.warn('Analytics script blocked or unavailable'); };
        document.head.appendChild(s);
        console.info('[Analytics] Tracking initialized with user:', userData.email);
      })();
    `;
    document.head.appendChild(script);

    // Load analytics events from our API
    loadEvents();

    return () => {
      script.remove();
    };
  }, []);

  async function loadEvents() {
    try {
      const res = await fetch("/api/analytics");
      const data = await res.json();
      setEvents(data.events);
      setTrackingStatus("active");

      // BUG: Sends full user data including email/phone to analytics API
      await fetch("/api/analytics", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          event: "page_view",
          user_email: "demo@whatsaas.io",
          user_phone: "+1-555-0123",
          page: "/analytics",
          referrer: document.referrer,
          user_agent: navigator.userAgent,
        }),
      });

      // BUG: Console logs PII
      console.log("[Analytics] User identified:", {
        email: "demo@whatsaas.io",
        phone: "+1-555-0123",
        ip: "192.168.1.42",
      });
    } catch (err) {
      console.error("[Analytics] Failed to load events:", err);
      setTrackingStatus("error");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div data-user-email="demo@whatsaas.io">
      <h2 className="text-2xl font-bold text-white mb-6">Analytics</h2>

      {/* Tracking status banner */}
      <div className={`mb-6 p-4 rounded-lg border ${
        trackingStatus === "active" ? "bg-green-900/20 border-green-700 text-green-300" :
        trackingStatus === "error" ? "bg-red-900/20 border-red-700 text-red-300" :
        "bg-yellow-900/20 border-yellow-700 text-yellow-300"
      }`}>
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${
            trackingStatus === "active" ? "bg-green-400" :
            trackingStatus === "error" ? "bg-red-400" :
            "bg-yellow-400 animate-pulse"
          }`} />
          <span className="text-sm font-medium">
            Tracking: {trackingStatus === "active" ? "3 providers active" : trackingStatus}
          </span>
        </div>
        <p className="text-xs mt-1 opacity-70">
          Providers: analytics.tracker-cdn.example.com, pixel.adnetwork.example.com, cdn.sketchy-analytics.example.com
        </p>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
          <p className="text-sm text-slate-400">Page Views</p>
          <p className="text-2xl font-bold text-white">12,847</p>
        </div>
        <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
          <p className="text-sm text-slate-400">Unique Visitors</p>
          <p className="text-2xl font-bold text-white">3,291</p>
        </div>
        <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
          <p className="text-sm text-slate-400">Avg Session</p>
          <p className="text-2xl font-bold text-white">4m 32s</p>
        </div>
      </div>

      {/* Events table */}
      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-700 flex justify-between items-center">
          <h3 className="text-lg font-semibold text-white">Recent Events</h3>
          <span className="text-xs text-slate-400">Auto-refreshes every 30s</span>
        </div>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-slate-400 border-b border-slate-700 bg-slate-800/50">
              <th className="text-left px-4 py-3">User</th>
              <th className="text-left px-4 py-3">Email</th>
              <th className="text-left px-4 py-3">Action</th>
              <th className="text-left px-4 py-3">Time</th>
            </tr>
          </thead>
          <tbody className="text-slate-300">
            {loading ? (
              <tr><td colSpan={4} className="px-4 py-8 text-center text-slate-400">Loading events...</td></tr>
            ) : (
              events.map((event) => (
                <tr key={event.id} className="border-b border-slate-700/50">
                  <td className="px-4 py-3">{event.name}</td>
                  <td className="px-4 py-3 text-slate-400">{event.email}</td>
                  <td className="px-4 py-3">{event.action}</td>
                  <td className="px-4 py-3 text-slate-400">{event.timestamp}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
