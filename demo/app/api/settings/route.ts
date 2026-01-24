import { NextResponse } from "next/server";

// BUG: Always returns 503 to trigger unhandled promise rejection on the client

export async function PUT() {
  // Simulate a database timeout error
  await new Promise((r) => setTimeout(r, 800));

  return NextResponse.json(
    {
      error: "Service Unavailable",
      message: "Database connection pool exhausted. All 20 connections are in use. Query timeout after 5000ms. Last query: UPDATE user_settings SET preferences = $1 WHERE user_id = $2. Consider increasing max_pool_size or optimizing long-running transactions.",
      code: "CONNECTION_POOL_EXHAUSTED",
      retryAfter: 30,
    },
    { status: 503 }
  );
}
