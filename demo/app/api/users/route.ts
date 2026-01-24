import { NextRequest, NextResponse } from "next/server";

// BUG: Returns 500 when searching for "admin"
// The error response body contains a detailed message that Gasoline captures

const users = [
  { id: 1, name: "Alice Johnson", email: "alice@example.com", role: "Admin", status: "Active" },
  { id: 2, name: "Bob Smith", email: "bob@example.com", role: "User", status: "Active" },
  { id: 3, name: "Carol White", email: "carol@example.com", role: "User", status: "Inactive" },
  { id: 4, name: "Dave Brown", email: "dave@example.com", role: "Moderator", status: "Active" },
  { id: 5, name: "Eve Davis", email: "eve@example.com", role: "User", status: "Active" },
  { id: 6, name: "Frank Miller", email: "frank@example.com", role: "User", status: "Active" },
  { id: 7, name: "Grace Lee", email: "grace@example.com", role: "Admin", status: "Active" },
];

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q")?.toLowerCase() || "";

  // BUG: Intentionally crash when searching for "admin"
  if (q === "admin") {
    return NextResponse.json(
      {
        error: "Internal Server Error",
        message: "FATAL: permission denied for relation users_admin_view — role 'app_readonly' does not have SELECT privilege on admin audit table. Query attempted: SELECT * FROM users_admin_view WHERE role = 'admin'. This view requires elevated 'app_admin' credentials which are not configured in the current connection pool.",
        code: "INSUFFICIENT_PRIVILEGE",
        hint: "Check database role permissions. The 'users_admin_view' requires app_admin role, not app_readonly.",
      },
      { status: 500 }
    );
  }

  const filtered = q
    ? users.filter(
        (u) =>
          u.name.toLowerCase().includes(q) ||
          u.email.toLowerCase().includes(q) ||
          u.role.toLowerCase().includes(q)
      )
    : users;

  return NextResponse.json({ users: filtered });
}
