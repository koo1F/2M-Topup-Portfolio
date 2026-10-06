"use client";

import React, { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { XCircle, CreditCard } from "lucide-react";

function PaymentCancelContent() {
  const searchParams = useSearchParams();
  const paymentId = searchParams.get("payment_id");

  return (
    <div className="max-w-md w-full bg-slate-900/60 rounded-3xl p-8 border border-slate-800/80 shadow-2xl text-center space-y-6 flex flex-col items-center">
      <div className="w-16 h-16 bg-rose-500/10 rounded-full flex items-center justify-center text-rose-500 border border-rose-500/20">
        <XCircle className="w-10 h-10" />
      </div>
      
      <div className="space-y-2">
        <h2 className="text-2xl font-bold text-white">Payment Cancelled</h2>
        <p className="text-sm text-slate-400">
          คุณได้ยกเลิกขั้นตอนการชำระเงิน หรือเซสชันหมดอายุแล้ว หากมีข้อผิดพลาดกรุณาลองใหม่อีกครั้ง
        </p>
        {paymentId && (
          <p className="text-xs text-slate-500 font-mono mt-2 bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-900 inline-block">
            Payment ID: {paymentId}
          </p>
        )}
      </div>

      <div className="pt-4 flex gap-4 w-full justify-center">
        <Link 
          href="/payment" 
          className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold rounded-2xl transition duration-150 flex items-center gap-2 shadow-lg shadow-indigo-600/20 w-full justify-center cursor-pointer"
        >
          <CreditCard className="w-4 h-4" />
          กลับไปหน้าเติมเงิน
        </Link>
      </div>
    </div>
  );
}

export default function PaymentCancelPage() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col items-center justify-center p-4">
      <Suspense fallback={
        <div className="max-w-md w-full bg-slate-900/60 rounded-3xl p-8 border border-slate-800/80 shadow-2xl text-center flex flex-col items-center justify-center py-12">
          <div className="w-12 h-12 border-4 border-indigo-500/30 border-t-indigo-500 rounded-full animate-spin"></div>
          <h2 className="text-xl font-bold text-white mt-4">Loading cancellation...</h2>
        </div>
      }>
        <PaymentCancelContent />
      </Suspense>
    </div>
  );
}
