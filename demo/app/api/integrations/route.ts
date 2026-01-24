import { NextResponse } from "next/server";

const integrations = [
  { id: "slack", name: "Slack", status: "connected", lastSync: "2 min ago", events: 1247 },
  { id: "github", name: "GitHub", status: "connected", lastSync: "15 min ago", events: 892 },
  { id: "jira", name: "Jira", status: "error", lastSync: "2 hrs ago", events: 456 },
  { id: "datadog", name: "Datadog", status: "connected", lastSync: "5 min ago", events: 3891 },
  { id: "pagerduty", name: "PagerDuty", status: "disconnected", lastSync: "Never", events: 0 },
  { id: "sentry", name: "Sentry", status: "connected", lastSync: "1 min ago", events: 2103 },
];

export async function GET() {
  return NextResponse.json({ integrations, total: integrations.length });
}
