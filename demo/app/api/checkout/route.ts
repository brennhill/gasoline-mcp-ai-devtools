import { NextResponse } from "next/server";

// BUG: Processes raw card numbers, logs PII, no encryption

export async function POST(request: Request) {
  const body = await request.json();

  // BUG: Logs full card number and SSN
  console.log("[Checkout API] Payment attempt:", {
    customer: body.customer?.email,
    card: body.payment?.card_number, // Full card number logged!
    ssn: body.customer?.ssn, // SSN logged!
    amount: body.amount,
  });

  // Simulate processing delay
  await new Promise(r => setTimeout(r, 1500));

  // BUG: 30% chance of failure to show error handling
  if (Math.random() < 0.3) {
    return NextResponse.json(
      { error: "Payment declined: insufficient funds", code: "card_declined" },
      { status: 402 }
    );
  }

  // BUG: Echoes card data back in response
  return NextResponse.json({
    status: "success",
    transaction_id: `txn_${Date.now()}`,
    amount: body.amount,
    last4: body.payment?.card_number?.slice(-4),
    customer_email: body.customer?.email,
  });
}
