import { NextResponse } from "next/server";

export async function GET(request: Request, { params }: { params: { id: string } }) {
  const { id } = params;
  const url = new URL(request.url);
  const timeout = parseInt(url.searchParams.get("timeout") || "5000");
  const verbose = url.searchParams.get("verbose") === "true";

  // Simulate connection test
  await new Promise(r => setTimeout(r, Math.min(timeout, 500)));

  // PagerDuty and Jira fail
  if (id === "pagerduty") {
    return NextResponse.json({
      success: false,
      error: "No API key configured",
      latency_ms: null,
      ...(verbose && { details: { attempted_at: new Date().toISOString(), endpoint: "https://api.pagerduty.com/services" } }),
    });
  }

  if (id === "jira") {
    return NextResponse.json({
      success: false,
      error: "OAuth token expired",
      latency_ms: null,
      ...(verbose && { details: { attempted_at: new Date().toISOString(), endpoint: "https://whatsaas.atlassian.net/rest/api/3/myself" } }),
    });
  }

  const latency = Math.floor(Math.random() * 200) + 20;
  return NextResponse.json({
    success: true,
    latency_ms: latency,
    ...(verbose && { details: { attempted_at: new Date().toISOString(), response_code: 200, api_version: "v2" } }),
  });
}
