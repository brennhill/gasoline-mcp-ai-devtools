import { NextRequest, NextResponse } from "next/server";

// BUG: Server expects user_id as number but client sends it as string
// Returns 422 with validation error when type is wrong

export async function POST(request: NextRequest) {
  const body = await request.json();

  // Validate user_id is a number (not a string)
  if (typeof body.user_id !== "number") {
    return NextResponse.json(
      {
        error: "Validation Error",
        message: `Field 'user_id' must be of type 'number', got '${typeof body.user_id}'. Value received: ${JSON.stringify(body.user_id)}. Please ensure the client sends user_id as an integer, not a string. Example: { "user_id": 42 } not { "user_id": "42" }`,
        field: "user_id",
        expected: "number",
        received: typeof body.user_id,
        value: body.user_id,
      },
      { status: 422 }
    );
  }

  // Would normally save to DB
  return NextResponse.json({ success: true, event_id: Date.now() });
}
