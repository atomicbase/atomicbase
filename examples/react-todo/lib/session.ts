import { sha256 } from "@oslojs/crypto/sha2";
import { encodeBase32LowerCaseNoPadding, encodeHexLowerCase } from "@oslojs/encoding";
import { cookies } from "next/headers";
import { primaryDb } from "./db";
import { eq } from "@atomicbase/sdk";

const SESSION_COOKIE_NAME = "session";
const SESSION_EXPIRY_MS = 1000 * 60 * 60 * 24 * 30; // 30 days

export interface Session {
  id: string;
  userId: number;
  expiresAt: Date;
}

export interface User {
  id: number;
  googleId: string;
  email: string;
  name: string;
  picture: string | null;
  tenantName: string;
}

// Database record types
interface SessionRecord {
  id: string;
  user_id: number;
  expires_at: number;
  created_at: number;
}

interface UserRecord {
  id: number;
  google_id: string;
  email: string;
  name: string;
  picture: string | null;
  tenant_name: string;
  created_at: string;
}

// Generate a random session token
export function generateSessionToken(): string {
  const bytes = new Uint8Array(20);
  crypto.getRandomValues(bytes);
  return encodeBase32LowerCaseNoPadding(bytes);
}

// Hash token to create session ID (following Lucia pattern)
function hashToken(token: string): string {
  const encoder = new TextEncoder();
  const hash = sha256(encoder.encode(token));
  return encodeHexLowerCase(hash);
}

// Create a new session for a user
export async function createSession(userId: number): Promise<{ token: string; session: Session }> {
  const token = generateSessionToken();
  const sessionId = hashToken(token);
  const expiresAt = new Date(Date.now() + SESSION_EXPIRY_MS);

  const { error } = await primaryDb.from("sessions").insert({
    id: sessionId,
    user_id: userId,
    expires_at: Math.floor(expiresAt.getTime() / 1000),
    created_at: Math.floor(Date.now() / 1000),
  });

  if (error) {
    throw new Error(`Failed to create session: ${error.message}`);
  }

  return {
    token,
    session: {
      id: sessionId,
      userId,
      expiresAt,
    },
  };
}

// Validate session token and return session + user if valid
export async function validateSession(
  token: string
): Promise<{ session: Session; user: User } | null> {
  const sessionId = hashToken(token);

  // Get session from database
  const { data: sessionData } = await primaryDb
    .from<SessionRecord>("sessions")
    .select()
    .where(eq("id", sessionId))
    .maybeSingle();

  if (!sessionData) return null;

  // Check expiration
  const expiresAt = new Date(sessionData.expires_at * 1000);
  if (Date.now() >= expiresAt.getTime()) {
    await invalidateSession(sessionId);
    return null;
  }

  // Get user
  const { data: userData } = await primaryDb
    .from<UserRecord>("users")
    .select()
    .where(eq("id", sessionData.user_id))
    .maybeSingle();

  if (!userData) return null;

  return {
    session: {
      id: sessionId,
      userId: sessionData.user_id,
      expiresAt,
    },
    user: {
      id: userData.id,
      googleId: userData.google_id,
      email: userData.email,
      name: userData.name,
      picture: userData.picture,
      tenantName: userData.tenant_name,
    },
  };
}

// Invalidate a session
export async function invalidateSession(sessionId: string): Promise<void> {
  await primaryDb.from("sessions").delete().where(eq("id", sessionId));
}

// Cookie management
export async function setSessionCookie(token: string): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.set(SESSION_COOKIE_NAME, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_EXPIRY_MS / 1000,
  });
}

export async function getSessionCookie(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get(SESSION_COOKIE_NAME)?.value;
}

export async function deleteSessionCookie(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
}
