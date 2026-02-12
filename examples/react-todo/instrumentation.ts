export async function register() {
  // Only run on the server
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { ensurePrimaryDatabase } = await import("./lib/db");
    await ensurePrimaryDatabase();
  }
}
