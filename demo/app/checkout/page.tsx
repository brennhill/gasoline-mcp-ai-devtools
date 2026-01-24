"use client";

import { useState } from "react";

// BUG: This checkout form sends PII (card numbers, SSN, email) in plain text
// to an insecure HTTP endpoint. Demonstrates security_audit tool.

export default function CheckoutPage() {
  const [form, setForm] = useState({
    name: "",
    email: "",
    phone: "",
    address: "",
    city: "",
    zip: "",
    cardNumber: "",
    cardExpiry: "",
    cardCvc: "",
    ssn: "",
  });
  const [step, setStep] = useState<"form" | "processing" | "error">("form");
  const [errorMessage, setErrorMessage] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setStep("processing");

    // BUG: Logs full card number to console
    console.log("[Checkout] Processing payment for:", form.email, "card:", form.cardNumber);

    try {
      // BUG: Sends PII including card number and SSN to server
      const res = await fetch("/api/checkout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          customer: {
            name: form.name,
            email: form.email,
            phone: form.phone,
            ssn: form.ssn, // BUG: SSN should never be sent to payment endpoint
            address: `${form.address}, ${form.city} ${form.zip}`,
          },
          payment: {
            card_number: form.cardNumber, // BUG: Full card number in request body
            expiry: form.cardExpiry,
            cvc: form.cardCvc, // BUG: CVC in request body
          },
          amount: 99.99,
          currency: "USD",
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `Payment failed: ${res.status}`);
      }

      // BUG: Also sends data to analytics with PII
      await fetch("/api/analytics", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          event: "purchase_complete",
          user_email: form.email,
          user_phone: form.phone,
          card_last4: form.cardNumber.slice(-4),
          amount: 99.99,
        }),
      });
    } catch (err) {
      console.error("[Checkout] Payment error:", err);
      setErrorMessage(err instanceof Error ? err.message : "Payment failed");
      setStep("error");
    }
  }

  function updateField(field: string, value: string) {
    setForm(prev => ({ ...prev, [field]: value }));
  }

  if (step === "processing") {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <div className="w-8 h-8 border-2 border-orange-500 border-t-transparent rounded-full animate-spin" />
        <p className="text-slate-400 mt-4">Processing payment...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold text-white mb-6">Checkout</h2>

      {step === "error" && (
        <div className="mb-6 p-4 bg-red-900/30 border border-red-700 rounded-lg text-red-300">
          {errorMessage}
          <button onClick={() => setStep("form")} className="ml-4 underline text-sm">Try again</button>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Contact Information */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Contact Information</h3>
          <div className="grid grid-cols-2 gap-4">
            <Input label="Full Name" name="name" value={form.name} onChange={v => updateField("name", v)} placeholder="John Doe" />
            <Input label="Email" name="email" type="email" value={form.email} onChange={v => updateField("email", v)} placeholder="john@example.com" />
            <Input label="Phone" name="phone" type="tel" value={form.phone} onChange={v => updateField("phone", v)} placeholder="+1-555-0123" />
            {/* BUG: SSN field in a checkout form — should never be here */}
            <Input label="SSN (for verification)" name="ssn" value={form.ssn} onChange={v => updateField("ssn", v)} placeholder="123-45-6789" />
          </div>
        </div>

        {/* Shipping Address */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Shipping Address</h3>
          <div className="space-y-4">
            <Input label="Street Address" name="address" value={form.address} onChange={v => updateField("address", v)} placeholder="123 Main St" fullWidth />
            <div className="grid grid-cols-2 gap-4">
              <Input label="City" name="city" value={form.city} onChange={v => updateField("city", v)} placeholder="San Francisco" />
              <Input label="ZIP Code" name="zip" value={form.zip} onChange={v => updateField("zip", v)} placeholder="94102" />
            </div>
          </div>
        </div>

        {/* Payment — BUG: No PCI compliance, raw card numbers */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Payment Details</h3>
          <div className="space-y-4">
            <Input label="Card Number" name="cardNumber" value={form.cardNumber} onChange={v => updateField("cardNumber", v)} placeholder="4242 4242 4242 4242" fullWidth data-testid="card-number" />
            <div className="grid grid-cols-2 gap-4">
              <Input label="Expiry" name="cardExpiry" value={form.cardExpiry} onChange={v => updateField("cardExpiry", v)} placeholder="12/28" />
              <Input label="CVC" name="cardCvc" value={form.cardCvc} onChange={v => updateField("cardCvc", v)} placeholder="123" />
            </div>
          </div>
          {/* BUG: No HTTPS indicator, no tokenization notice */}
          <p className="text-xs text-slate-500 mt-3">Your payment is processed directly.</p>
        </div>

        {/* Order Summary */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Order Summary</h3>
          <div className="flex justify-between text-slate-300 mb-2">
            <span>Pro Plan (monthly)</span>
            <span>$99.99</span>
          </div>
          <div className="flex justify-between text-slate-300 mb-2">
            <span>Tax</span>
            <span>$0.00</span>
          </div>
          <div className="flex justify-between text-white font-bold pt-2 border-t border-slate-700">
            <span>Total</span>
            <span>$99.99</span>
          </div>
        </div>

        <button
          type="submit"
          data-testid="submit-payment"
          className="w-full py-3 bg-orange-600 hover:bg-orange-700 rounded-lg text-white font-semibold transition-colors"
        >
          Pay $99.99
        </button>
      </form>
    </div>
  );
}

function Input({ label, name, value, onChange, placeholder, type = "text", fullWidth = false, ...props }: {
  label: string;
  name: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
  fullWidth?: boolean;
  [key: string]: unknown;
}) {
  return (
    <div className={fullWidth ? "col-span-2" : ""}>
      <label className="block text-sm text-slate-400 mb-1">{label}</label>
      <input
        type={type}
        name={name}
        data-testid={name}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-orange-500"
        {...props}
      />
    </div>
  );
}
