import { NextResponse } from "next/server";

// BUG: No authentication, no rate limiting, returns PII in response bodies

const mockEvents = [
  { id: 1, name: "Alice Johnson", email: "alice@example.com", action: "page_view", timestamp: "2 min ago" },
  { id: 2, name: "Bob Smith", email: "bob@example.com", action: "click_cta", timestamp: "5 min ago" },
  { id: 3, name: "Carol White", email: "carol@example.com", action: "form_submit", timestamp: "12 min ago" },
  { id: 4, name: "Dave Brown", email: "dave@example.com", action: "purchase", timestamp: "23 min ago" },
  { id: 5, name: "Eve Davis", email: "eve@example.com", action: "signup", timestamp: "1 hr ago" },
];

export async function GET() {
  // BUG: Returns full PII including emails in response
  return NextResponse.json({
    events: mockEvents,
    total: mockEvents.length,
    tracking_id: "UA-DEMO-123456",
  });
}

export async function POST(request: Request) {
  const body = await request.json();

  // BUG: Logs PII to server console
  console.log("[Analytics API] Event received:", body.event, "from:", body.user_email);

  // BUG: Echoes back PII in response
  return NextResponse.json({
    status: "recorded",
    event_id: `evt_${Date.now()}`,
    user: body.user_email,
    phone: body.user_phone, // BUG: Echoes phone back
  });
}
