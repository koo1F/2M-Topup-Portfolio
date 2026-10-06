import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import { getApiUrl } from "@/lib/api";

export async function POST(request: Request) {
  try {
    const { email, password } = await request.json();

    const apiUrl = getApiUrl();
    const backendRes = await fetch(`${apiUrl}/api/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    if (!backendRes.ok) {
      const errorData = await backendRes.json().catch(() => ({}));
      return NextResponse.json(
        { error: errorData.error || "Invalid credentials" },
        { status: backendRes.status }
      );
    }

    const data = await backendRes.json();
    const token = data.token;

    if (!token) {
      return NextResponse.json(
        { error: "Token not received from backend" },
        { status: 500 }
      );
    }

    // Set httpOnly cookie
    const cookieStore = await cookies();
    cookieStore.set("token", token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24, // 24 hours
    });

    return NextResponse.json({ success: true });
  } catch (error: unknown) {
    console.error("Login proxy error:", error);
    return NextResponse.json(
      { error: "Internal server error connecting to backend" },
      { status: 500 }
    );
  }
}
