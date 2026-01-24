import { NextResponse } from "next/server";

export async function GET() {
  return NextResponse.json({
    total: 6,
    connected: 4,
    errored: 1,
    disconnected: 1,
    events_today: 428,
    events_week: 8589,
  });
}
