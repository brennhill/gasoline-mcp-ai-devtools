"use client";

import { useState } from "react";

// BUG: Multiple accessibility violations
// - Missing aria-labels on inputs
// - Broken tabIndex ordering
// - No error announcements (aria-live)
// - Color contrast issues
// - Form inputs without associated labels

export default function BillingPage() {
  const [cardNumber, setCardNumber] = useState("");
  const [expiry, setExpiry] = useState("");
  const [cvv, setCvv] = useState("");
  const [name, setName] = useState("");

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Billing</h2>

      <div className="max-w-2xl">
        {/* Current plan */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700 mb-6">
          <div className="flex justify-between items-center">
            <div>
              <h3 className="text-lg font-semibold text-white">Pro Plan</h3>
              <p className="text-slate-400 text-sm mt-1">$49/month, billed monthly</p>
            </div>
            {/* BUG: Button color has poor contrast ratio against background */}
            <button className="px-4 py-2 text-slate-500 bg-slate-700 rounded-lg text-sm">
              Change Plan
            </button>
          </div>
        </div>

        {/* Payment form with a11y violations */}
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Payment Method</h3>

          <div className="space-y-4">
            {/* BUG: No <label> element, no aria-label */}
            <div>
              <span className="block text-sm text-slate-400 mb-1">Card Number</span>
              {/* BUG: tabIndex broken — should be natural order but is set to 3 */}
              <input
                tabIndex={3}
                type="text"
                placeholder="1234 5678 9012 3456"
                value={cardNumber}
                onChange={(e) => setCardNumber(e.target.value)}
                className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-orange-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              {/* BUG: No label, no aria-label */}
              <div>
                <span className="block text-sm text-slate-400 mb-1">Expiry</span>
                <input
                  tabIndex={1}
                  type="text"
                  placeholder="MM/YY"
                  value={expiry}
                  onChange={(e) => setExpiry(e.target.value)}
                  className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-orange-500"
                />
              </div>

              {/* BUG: No label, no aria-label */}
              <div>
                <span className="block text-sm text-slate-400 mb-1">CVV</span>
                <input
                  tabIndex={2}
                  type="text"
                  placeholder="123"
                  value={cvv}
                  onChange={(e) => setCvv(e.target.value)}
                  className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-orange-500"
                />
              </div>
            </div>

            {/* BUG: No label, no aria-label */}
            <div>
              <span className="block text-sm text-slate-400 mb-1">Cardholder Name</span>
              <input
                tabIndex={4}
                type="text"
                placeholder="John Doe"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-orange-500"
              />
            </div>

            {/* BUG: No aria-live region for form errors, no error announcements */}
            <button
              data-testid="update-payment"
              className="w-full px-6 py-3 bg-orange-500 hover:bg-orange-600 text-white rounded-lg font-medium transition-colors"
            >
              Update Payment Method
            </button>
          </div>
        </div>

        {/* Invoice history */}
        <div className="mt-6 bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Invoice History</h3>
          {/* BUG: Image without alt text */}
          <table className="w-full text-sm">
            <thead>
              <tr className="text-slate-400 border-b border-slate-700">
                <th className="text-left py-2">Date</th>
                <th className="text-left py-2">Amount</th>
                <th className="text-left py-2">Status</th>
                <th className="text-left py-2">Action</th>
              </tr>
            </thead>
            <tbody className="text-slate-300">
              <tr className="border-b border-slate-700/50">
                <td className="py-3">Jan 1, 2025</td>
                <td>$49.00</td>
                <td><span className="text-green-400">Paid</span></td>
                {/* BUG: Link without href — not keyboard accessible */}
                <td><a className="text-orange-400 cursor-pointer">Download</a></td>
              </tr>
              <tr className="border-b border-slate-700/50">
                <td className="py-3">Dec 1, 2024</td>
                <td>$49.00</td>
                <td><span className="text-green-400">Paid</span></td>
                <td><a className="text-orange-400 cursor-pointer">Download</a></td>
              </tr>
              <tr className="border-b border-slate-700/50">
                <td className="py-3">Nov 1, 2024</td>
                <td>$49.00</td>
                <td><span className="text-green-400">Paid</span></td>
                <td><a className="text-orange-400 cursor-pointer">Download</a></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
