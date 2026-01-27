import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Check if accessing dashboard
  if (pathname.startsWith("/dashboard")) {
    // Check for session cookie
    const sessionToken = request.cookies.get("session")?.value;

    if (!sessionToken) {
      // Redirect to home page if not authenticated
      const url = new URL("/", request.url);
      return NextResponse.redirect(url);
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*"],
};
