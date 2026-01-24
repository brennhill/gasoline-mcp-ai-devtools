import { NextResponse } from "next/server";

// BUG: Sometimes returns null data to trigger the race condition
// on the reports page (data.reports.map when data is null)

export async function GET() {
  // Simulate variable latency — sometimes fast, sometimes slow
  const delay = Math.random() * 500 + 200;
  await new Promise((r) => setTimeout(r, delay));

  // BUG: Randomly return null to simulate race condition
  // In production this might be a cache miss or stale response
  if (Math.random() < 0.4) {
    return NextResponse.json(null);
  }

  return NextResponse.json({
    reports: [
      { id: 1, title: "Q4 Revenue Analysis", date: "Jan 15, 2025", status: "Published", downloads: 234 },
      { id: 2, title: "User Growth Report", date: "Jan 10, 2025", status: "Published", downloads: 189 },
      { id: 3, title: "Churn Analysis Draft", date: "Jan 8, 2025", status: "Draft", downloads: 0 },
      { id: 4, title: "Feature Usage Metrics", date: "Jan 5, 2025", status: "Published", downloads: 412 },
      { id: 5, title: "Support Ticket Trends", date: "Dec 28, 2024", status: "Archived", downloads: 67 },
    ],
  });
}
