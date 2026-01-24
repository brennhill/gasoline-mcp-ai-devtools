"use client";

import { useState } from "react";

// BUG: Save button triggers an unhandled promise rejection
// No try/catch — the rejection is unhandled and button appears to do nothing

export default function SettingsPage() {
  const [formData, setFormData] = useState({
    displayName: "Demo User",
    email: "demo@whatsaas.io",
    timezone: "America/New_York",
    language: "en",
    notifications: true,
    darkMode: true,
  });

  function handleChange(field: string, value: string | boolean) {
    setFormData((prev) => ({ ...prev, [field]: value }));
  }

  function handleSave() {
    // BUG: No try/catch, no .catch() — unhandled promise rejection
    saveSettings(formData);
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Settings</h2>

      <div className="max-w-2xl space-y-6">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Profile</h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm text-slate-400 mb-1">Display Name</label>
              <input
                type="text"
                value={formData.displayName}
                onChange={(e) => handleChange("displayName", e.target.value)}
                className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white focus:outline-none focus:border-orange-500"
              />
            </div>
            <div>
              <label className="block text-sm text-slate-400 mb-1">Email</label>
              <input
                type="email"
                value={formData.email}
                onChange={(e) => handleChange("email", e.target.value)}
                className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white focus:outline-none focus:border-orange-500"
              />
            </div>
          </div>
        </div>

        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Preferences</h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm text-slate-400 mb-1">Timezone</label>
              <select
                value={formData.timezone}
                onChange={(e) => handleChange("timezone", e.target.value)}
                className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white focus:outline-none focus:border-orange-500"
              >
                <option value="America/New_York">Eastern (ET)</option>
                <option value="America/Chicago">Central (CT)</option>
                <option value="America/Denver">Mountain (MT)</option>
                <option value="America/Los_Angeles">Pacific (PT)</option>
              </select>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-slate-300">Email Notifications</span>
              <button
                onClick={() => handleChange("notifications", !formData.notifications)}
                className={`w-10 h-6 rounded-full transition-colors ${
                  formData.notifications ? "bg-orange-500" : "bg-slate-600"
                } relative`}
              >
                <span className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${
                  formData.notifications ? "left-5" : "left-1"
                }`} />
              </button>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-slate-300">Dark Mode</span>
              <button
                onClick={() => handleChange("darkMode", !formData.darkMode)}
                className={`w-10 h-6 rounded-full transition-colors ${
                  formData.darkMode ? "bg-orange-500" : "bg-slate-600"
                } relative`}
              >
                <span className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${
                  formData.darkMode ? "left-5" : "left-1"
                }`} />
              </button>
            </div>
          </div>
        </div>

        <button
          data-testid="save-button"
          onClick={handleSave}
          className="px-6 py-2 bg-orange-500 hover:bg-orange-600 text-white rounded-lg font-medium transition-colors"
        >
          Save Changes
        </button>
      </div>
    </div>
  );
}

// BUG: This async function rejects, but the caller doesn't await or catch it
async function saveSettings(data: Record<string, unknown>): Promise<void> {
  const res = await fetch("/api/settings", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    throw new Error("Failed to save settings: database connection timeout");
  }
}
