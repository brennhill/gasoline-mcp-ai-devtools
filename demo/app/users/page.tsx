"use client";

import { useState } from "react";

// BUG: Searching for "admin" triggers a 500 error from the API

const mockUsers = [
  { id: 1, name: "Alice Johnson", email: "alice@example.com", role: "Admin", status: "Active" },
  { id: 2, name: "Bob Smith", email: "bob@example.com", role: "User", status: "Active" },
  { id: 3, name: "Carol White", email: "carol@example.com", role: "User", status: "Inactive" },
  { id: 4, name: "Dave Brown", email: "dave@example.com", role: "Moderator", status: "Active" },
  { id: 5, name: "Eve Davis", email: "eve@example.com", role: "User", status: "Active" },
];

export default function UsersPage() {
  const [query, setQuery] = useState("");
  const [users, setUsers] = useState(mockUsers);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSearch(searchQuery: string) {
    setQuery(searchQuery);
    if (!searchQuery.trim()) {
      setUsers(mockUsers);
      setError(null);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch(`/api/users?q=${encodeURIComponent(searchQuery)}`);
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || `HTTP ${res.status}`);
      }
      const data = await res.json();
      setUsers(data.users);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Users</h2>

      {/* Search */}
      <div className="mb-6">
        <input
          data-testid="search"
          type="text"
          placeholder="Search users..."
          value={query}
          onChange={(e) => handleSearch(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") handleSearch(query); }}
          className="w-full max-w-md px-4 py-2 bg-slate-800 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-orange-500"
        />
      </div>

      {/* Error state */}
      {error && (
        <div data-testid="error-message" className="mb-4 p-4 bg-red-900/30 border border-red-700 rounded-lg text-red-300">
          Error: {error}
        </div>
      )}

      {/* Users table */}
      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-slate-400 border-b border-slate-700 bg-slate-800/50">
              <th className="text-left px-4 py-3">Name</th>
              <th className="text-left px-4 py-3">Email</th>
              <th className="text-left px-4 py-3">Role</th>
              <th className="text-left px-4 py-3">Status</th>
            </tr>
          </thead>
          <tbody className="text-slate-300">
            {loading ? (
              <tr><td colSpan={4} className="px-4 py-8 text-center text-slate-400">Loading...</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={4} className="px-4 py-8 text-center text-slate-400">No users found</td></tr>
            ) : (
              users.map((user) => (
                <tr key={user.id} className="border-b border-slate-700/50 hover:bg-slate-700/30">
                  <td className="px-4 py-3">{user.name}</td>
                  <td className="px-4 py-3">{user.email}</td>
                  <td className="px-4 py-3">
                    <span className="px-2 py-1 rounded-full text-xs bg-slate-700">{user.role}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded-full text-xs ${
                      user.status === "Active" ? "bg-green-900/50 text-green-300" : "bg-slate-700 text-slate-400"
                    }`}>
                      {user.status}
                    </span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
