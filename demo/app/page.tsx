"use client";

import { useState, useEffect } from "react";

// BUG: Chart loads after 2s delay, causing massive CLS (layout shift)
// The container has no fixed height, so content jumps when chart appears

export default function Dashboard() {
  const [chartData, setChartData] = useState<number[] | null>(null);
  const [stats] = useState({
    users: 1248,
    revenue: 48520,
    orders: 384,
    conversion: 3.2,
  });

  useEffect(() => {
    // Simulate slow chart data load — causes CLS
    const timer = setTimeout(() => {
      setChartData([12, 19, 3, 5, 2, 3, 9, 15, 22, 18, 25, 30]);
    }, 2000);
    return () => clearTimeout(timer);
  }, []);

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Dashboard</h2>

      {/* Stats cards */}
      <div className="grid grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Users" value={stats.users.toLocaleString()} change="+12%" />
        <StatCard label="Revenue" value={`$${stats.revenue.toLocaleString()}`} change="+8%" />
        <StatCard label="Orders" value={stats.orders.toString()} change="+23%" />
        <StatCard label="Conversion" value={`${stats.conversion}%`} change="-0.5%" negative />
      </div>

      {/* BUG: No fixed height on chart container = CLS when data loads */}
      <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Revenue Over Time</h3>
        {chartData ? (
          <div className="flex items-end gap-2 h-64">
            {chartData.map((val, i) => (
              <div key={i} className="flex-1 flex flex-col items-center gap-1">
                <div
                  className="w-full bg-orange-500 rounded-t transition-all"
                  style={{ height: `${(val / 30) * 100}%` }}
                />
                <span className="text-xs text-slate-400">
                  {["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"][i]}
                </span>
              </div>
            ))}
          </div>
        ) : null /* BUG: No placeholder/skeleton — content jumps when chart appears */}
      </div>

      {/* Recent activity table */}
      <div className="mt-8 bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Recent Activity</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-slate-400 border-b border-slate-700">
              <th className="text-left py-2">User</th>
              <th className="text-left py-2">Action</th>
              <th className="text-left py-2">Date</th>
              <th className="text-left py-2">Status</th>
            </tr>
          </thead>
          <tbody className="text-slate-300">
            <tr className="border-b border-slate-700/50">
              <td className="py-3">John Doe</td>
              <td>Created project</td>
              <td>2 min ago</td>
              <td><span className="text-green-400">Completed</span></td>
            </tr>
            <tr className="border-b border-slate-700/50">
              <td className="py-3">Jane Smith</td>
              <td>Updated billing</td>
              <td>15 min ago</td>
              <td><span className="text-green-400">Completed</span></td>
            </tr>
            <tr className="border-b border-slate-700/50">
              <td className="py-3">Bob Wilson</td>
              <td>Deleted account</td>
              <td>1 hr ago</td>
              <td><span className="text-red-400">Failed</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

function StatCard({ label, value, change, negative = false }: {
  label: string;
  value: string;
  change: string;
  negative?: boolean;
}) {
  return (
    <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
      <p className="text-sm text-slate-400">{label}</p>
      <p className="text-2xl font-bold text-white mt-1">{value}</p>
      <p className={`text-sm mt-1 ${negative ? "text-red-400" : "text-green-400"}`}>
        {change} vs last month
      </p>
    </div>
  );
}
