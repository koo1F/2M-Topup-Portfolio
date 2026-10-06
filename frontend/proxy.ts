import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function proxy(request: NextRequest) {
  const token = request.cookies.get("token")?.value;
  const { pathname } = request.nextUrl;

  const protectedPaths = ["/dashboard", "/wallet", "/payment"];
  const authPaths = ["/login", "/register"];

  // Check if path is protected and user is not authenticated
  if (protectedPaths.some((path) => (pathname === path || pathname.startsWith(`${path}/`)))) {
    if (!token) {
      const loginUrl = new URL("/login", request.url);
      return NextResponse.redirect(loginUrl);
    }
  }

  // Check if path is login/register and user is already authenticated
  if (authPaths.some((path) => (pathname === path || pathname.startsWith(`${path}/`)))) {
    if (token) {
      const dashboardUrl = new URL("/dashboard", request.url);
      return NextResponse.redirect(dashboardUrl);
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     */
    "/((?!api|_next/static|_next/image|favicon.ico).*)",
  ],
};

