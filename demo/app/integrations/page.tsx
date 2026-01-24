"use client";

import { useState, useEffect } from "react";

// This page makes multiple API calls to different endpoints.
// Demonstrates analyze/api (schema detection) and observe/network tools.

interface Integration {
  id: string;
  name: string;
  status: "connected" | "disconnected" | "error";
  lastSync: string;
  events: number;
}

export default function IntegrationsPage() {
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState<string | null>(null);

  useEffect(() => {
    loadIntegrations();
  }, []);

  async function loadIntegrations() {
    try {
      // Multiple API calls to show analyze/api discovering the schema
      const [integrationsRes, statsRes] = await Promise.all([
        fetch("/api/integrations"),
        fetch("/api/integrations/stats"),
      ]);

      if (!integrationsRes.ok) throw new Error(`Failed: ${integrationsRes.status}`);
      const data = await integrationsRes.json();
      setIntegrations(data.integrations);

      if (statsRes.ok) {
        const stats = await statsRes.json();
        console.info("[Integrations] Stats loaded:", stats.total, "connections");
      }
    } catch (err) {
      console.error("[Integrations] Load failed:", err);
    } finally {
      setLoading(false);
    }
  }

  async function syncIntegration(id: string) {
    setSyncing(id);
    console.log(`[Integrations] Syncing ${id}...`);

    try {
      // POST to trigger sync — shows a different request pattern for API schema
      const res = await fetch(`/api/integrations/${id}/sync`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ force: true, since: new Date(Date.now() - 86400000).toISOString() }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `Sync failed: ${res.status}`);
      }

      const result = await res.json();
      console.info(`[Integrations] Sync complete for ${id}:`, result.synced, "events");

      // Refresh the list
      await loadIntegrations();
    } catch (err) {
      console.error(`[Integrations] Sync failed for ${id}:`, err);
    } finally {
      setSyncing(null);
    }
  }

  async function testConnection(id: string) {
    try {
      // GET with query params — another pattern for API schema detection
      const res = await fetch(`/api/integrations/${id}/test?timeout=5000&verbose=true`);
      const data = await res.json();

      if (data.success) {
        console.info(`[Integrations] Connection test passed for ${id}`);
      } else {
        console.warn(`[Integrations] Connection test failed for ${id}:`, data.error);
      }
    } catch (err) {
      console.error(`[Integrations] Test failed for ${id}:`, err);
    }
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Integrations</h2>

      {/* Integration cards */}
      <div className="space-y-4">
        {loading ? (
          <div className="text-center py-12 text-slate-400">Loading integrations...</div>
        ) : (
          integrations.map((integration) => (
            <div key={integration.id} className="bg-slate-800 rounded-xl p-6 border border-slate-700">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className={`w-3 h-3 rounded-full ${
                    integration.status === "connected" ? "bg-green-400" :
                    integration.status === "error" ? "bg-red-400" :
                    "bg-slate-500"
                  }`} />
                  <div>
                    <h3 className="text-white font-medium">{integration.name}</h3>
                    <p className="text-sm text-slate-400">
                      {integration.status === "connected" ? `Last sync: ${integration.lastSync}` :
                       integration.status === "error" ? "Connection error" :
                       "Not connected"}
                    </p>
                  </div>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => testConnection(integration.id)}
                    className="px-3 py-1.5 text-xs bg-slate-700 hover:bg-slate-600 rounded-lg text-slate-300 transition-colors"
                  >
                    Test
                  </button>
                  <button
                    onClick={() => syncIntegration(integration.id)}
                    disabled={syncing === integration.id}
                    className="px-3 py-1.5 text-xs bg-orange-600 hover:bg-orange-700 disabled:opacity-50 rounded-lg text-white transition-colors"
                  >
                    {syncing === integration.id ? "Syncing..." : "Sync Now"}
                  </button>
                </div>
              </div>
              {integration.events > 0 && (
                <div className="mt-3 pt-3 border-t border-slate-700">
                  <span className="text-xs text-slate-400">{integration.events.toLocaleString()} events synced</span>
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
