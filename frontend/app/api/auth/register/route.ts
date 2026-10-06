import { NextResponse } from "next/server";
import { getApiUrl } from "@/lib/api";

export async function POST(request: Request) {
  try {
    const { email, password } = await request.json();

    const apiUrl = getApiUrl();
    const backendRes = await fetch(`${apiUrl}/api/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    if (!backendRes.ok) {
      const errorData = await backendRes.json().catch(() => ({}));
      return NextResponse.json(
        { error: errorData.error || "Failed to register" },
        { status: backendRes.status }
      );
    }

    const data = await backendRes.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error("Register proxy error:", error);
    return NextResponse.json(
      { error: "Internal server error connecting to backend" },
      { status: 500 }
    );
  }
}
