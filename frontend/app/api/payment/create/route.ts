import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import { getApiUrl } from "@/lib/api";

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const idempotencyKey = request.headers.get("Idempotency-Key");

    if (!idempotencyKey) {
      return NextResponse.json({ error: "Idempotency-Key is required" }, { status: 400 });
    }

    const cookieStore = await cookies();
    const token = cookieStore.get("token")?.value;

    if (!token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const apiUrl = getApiUrl();
    const backendRes = await fetch(`${apiUrl}/api/payment/create`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${token}`,
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(body),
    });

    if (!backendRes.ok) {
      const errorData = await backendRes.json().catch(() => ({}));
      return NextResponse.json(
        { error: errorData.error || "Failed to create payment" },
        { status: backendRes.status }
      );
    }

    const data = await backendRes.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error("Payment create proxy error:", error);
    return NextResponse.json(
      { error: "Internal server error connecting to backend" },
      { status: 500 }
    );
  }
}
