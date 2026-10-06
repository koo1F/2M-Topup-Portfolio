"use client";

import React from "react";
import { ArrowUpRight, ArrowDownLeft, CheckCircle2, XCircle, Clock } from "lucide-react";

export interface Transaction {
  id: number;
  type: string;
  amount: number;
  status: string;
  reference_id: string;
  created_at: string;
}

interface TransactionListProps {
  transactions: Transaction[];
  loading: boolean;
}

export default function TransactionList({ transactions, loading }: TransactionListProps) {
  if (loading) {
    return (
      <div className="space-y-3">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="flex justify-between items-center p-4 bg-white rounded-lg border border-hairline animate-pulse">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-canvas-parchment rounded-sm" />
              <div className="space-y-2">
                <div className="h-4 w-24 bg-canvas-parchment rounded" />
                <div className="h-3 w-32 bg-canvas-parchment rounded" />
              </div>
            </div>
            <div className="space-y-2 flex flex-col items-end">
              <div className="h-4 w-16 bg-canvas-parchment rounded" />
              <div className="h-3 w-12 bg-canvas-parchment rounded" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (transactions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center bg-white rounded-lg border border-hairline">
        <div className="w-14 h-14 bg-canvas-parchment rounded-full flex items-center justify-center text-ink mb-4">
          <ArrowUpRight className="w-6 h-6" />
        </div>
        <h3 className="text-body-strong text-ink">No Transactions Yet</h3>
        <p className="text-caption-apple text-ink-muted-48 mt-1 max-w-sm">
          Your wallet transactions will appear here as soon as you deposit or make payments.
        </p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg border border-hairline overflow-hidden select-none">
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="border-b border-hairline text-caption-strong text-ink bg-canvas-parchment/60">
              <th className="p-4 font-semibold">ID</th>
              <th className="p-4 font-semibold">Type</th>
              <th className="p-4 font-semibold">Amount</th>
              <th className="p-4 font-semibold">Status</th>
              <th className="p-4 font-semibold">Reference ID</th>
              <th className="p-4 text-right font-semibold">Date</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-hairline text-caption-apple text-ink">
            {transactions.map((tx) => {
              const isDeposit = tx.type === "DEPOSIT" || tx.type === "REFUND";
              const formattedDate = new Date(tx.created_at).toLocaleString("en-US", {
                dateStyle: "medium",
                timeStyle: "short",
              });

              return (
                <tr key={tx.id} className="hover:bg-canvas-parchment/30 transition-colors duration-150">
                  <td className="p-4 font-mono text-xs text-ink-muted-48">
                    #{String(tx.id).padStart(6, "0")}
                  </td>
                  <td className="p-4">
                    <div className="flex items-center gap-3">
                      <div className={`p-2 rounded-sm flex items-center justify-center border ${
                        isDeposit
                          ? "bg-emerald-50 text-emerald-600 border-emerald-200"
                          : "bg-rose-50 text-rose-600 border-rose-200"
                      }`}>
                        {isDeposit ? <ArrowDownLeft className="w-3.5 h-3.5" /> : <ArrowUpRight className="w-3.5 h-3.5" />}
                      </div>
                      <span className="font-semibold text-ink">{tx.type}</span>
                    </div>
                  </td>
                  <td className="p-4 font-mono font-bold">
                    <span className={isDeposit ? "text-emerald-600" : "text-rose-600"}>
                      {isDeposit ? "+" : "-"}
                      {tx.amount.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </span>
                  </td>
                  <td className="p-4">
                    <div className="flex items-center gap-2">
                      {tx.status === "SUCCESS" && (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          Success
                        </span>
                      )}
                      {tx.status === "PENDING" && (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200">
                          <Clock className="w-3.5 h-3.5 animate-pulse" />
                          Pending
                        </span>
                      )}
                      {tx.status === "FAILED" && (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
                          <XCircle className="w-3.5 h-3.5" />
                          Failed
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="p-4 text-xs font-mono text-ink-muted-48">
                    {tx.reference_id}
                  </td>
                  <td className="p-4 text-right text-xs text-ink-muted-48 font-medium">
                    {formattedDate}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
