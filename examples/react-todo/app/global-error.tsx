"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Global error:", error);
  }, [error]);

  return (
    <html lang="en">
      <body className="antialiased">
        <div className="min-h-screen bg-zinc-50 dark:bg-zinc-950 p-8 flex items-center justify-center">
          <div className="max-w-md w-full bg-white dark:bg-zinc-900 rounded-lg border shadow-sm p-6">
            <div className="text-center">
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-100 dark:bg-red-900/20">
                <svg
                  className="h-6 w-6 text-red-600 dark:text-red-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <h1 className="text-xl font-semibold mb-2">Something went wrong</h1>
              <p className="text-zinc-500 dark:text-zinc-400 mb-6">
                {error.message || "An unexpected error occurred."}
              </p>
              <div className="flex justify-center gap-2">
                <button
                  onClick={() => window.location.href = "/"}
                  className="px-4 py-2 text-sm font-medium border rounded-md hover:bg-zinc-50 dark:hover:bg-zinc-800"
                >
                  Go Home
                </button>
                <button
                  onClick={reset}
                  className="px-4 py-2 text-sm font-medium bg-zinc-900 text-white rounded-md hover:bg-zinc-800 dark:bg-zinc-50 dark:text-zinc-900 dark:hover:bg-zinc-200"
                >
                  Try Again
                </button>
              </div>
            </div>
          </div>
        </div>
      </body>
    </html>
  );
}
