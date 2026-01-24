import { NextResponse } from "next/server";

export async function POST(request: Request, { params }: { params: { id: string } }) {
  const body = await request.json();
  const { id } = params;

  // Simulate sync delay
  await new Promise(r => setTimeout(r, 800));

  // Jira always fails (to demo error states)
  if (id === "jira") {
    return NextResponse.json(
      { error: "Connection refused: authentication token expired", code: "auth_expired" },
      { status: 401 }
    );
  }

  const synced = Math.floor(Math.random() * 50) + 5;
  return NextResponse.json({
    status: "complete",
    integration_id: id,
    synced,
    since: body.since,
    duration_ms: Math.floor(Math.random() * 2000) + 200,
  });
}
