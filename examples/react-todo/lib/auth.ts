import { cache } from "react";
import { getSessionCookie, validateSession, type User, type Session } from "./session";

// Cached function to get current session (prevents multiple DB calls per request)
export const getCurrentSession = cache(
  async (): Promise<{ session: Session | null; user: User | null }> => {
    const token = await getSessionCookie();
    if (!token) {
      return { session: null, user: null };
    }

    const result = await validateSession(token);
    if (!result) {
      return { session: null, user: null };
    }

    return result;
  }
);

// Check if user is authenticated
export async function isAuthenticated(): Promise<boolean> {
  const { session } = await getCurrentSession();
  return session !== null;
}

// Require authentication (for use in server components/actions)
export async function requireAuth(): Promise<{ session: Session; user: User }> {
  const { session, user } = await getCurrentSession();
  if (!session || !user) {
    throw new Error("Unauthorized");
  }
  return { session, user };
}
