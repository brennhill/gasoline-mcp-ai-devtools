"use client";

import { useState, useEffect } from "react";

// BUG: Race condition — API sometimes returns null, and component tries to render
// data.reports.map() on the null response, crashing with "Cannot read properties of null"

interface Report {
  id: number;
  title: string;
  date: string;
  status: string;
  downloads: number;
}

export default function ReportsPage() {
  const [data, setData] = useState<{ reports: Report[] } | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    fetch("/api/reports")
      .then((res) => res.json())
      .then((json) => {
        setData(json);
        setLoaded(true);
      });
  }, []);

  // Show loading state until fetch completes
  if (!loaded) {
    return (
      <div>
        <h2 className="text-2xl font-bold text-white mb-6">Reports</h2>
        <div className="bg-slate-800 rounded-xl p-12 border border-slate-700 text-center">
          <p className="text-slate-400">Loading reports...</p>
        </div>
      </div>
    );
  }

  // BUG: After fetch, data might be null (API randomly returns null)
  // This crashes with "Cannot read properties of null (reading 'reports')"
  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Reports</h2>

      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-slate-400 border-b border-slate-700 bg-slate-800/50">
              <th className="text-left px-4 py-3">Title</th>
              <th className="text-left px-4 py-3">Date</th>
              <th className="text-left px-4 py-3">Status</th>
              <th className="text-left px-4 py-3">Downloads</th>
            </tr>
          </thead>
          <tbody className="text-slate-300">
            {/* BUG: data could be null here — crashes with "Cannot read properties of null (reading 'reports')" */}
            {data!.reports.map((report) => (
              <tr key={report.id} className="border-b border-slate-700/50 hover:bg-slate-700/30">
                <td className="px-4 py-3 text-white">{report.title}</td>
                <td className="px-4 py-3">{report.date}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-1 rounded-full text-xs ${
                    report.status === "Published" ? "bg-green-900/50 text-green-300" :
                    report.status === "Draft" ? "bg-yellow-900/50 text-yellow-300" :
                    "bg-slate-700 text-slate-400"
                  }`}>
                    {report.status}
                  </span>
                </td>
                <td className="px-4 py-3">{report.downloads}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
