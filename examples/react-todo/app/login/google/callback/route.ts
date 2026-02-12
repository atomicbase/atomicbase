import { decodeIdToken, OAuth2RequestError } from "arctic";
import { google } from "@/lib/google";
import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { primaryDb, client } from "@/lib/db";
import { createSession } from "@/lib/session";
import { eq } from "@atomicbase/sdk";

const SESSION_COOKIE_NAME = "session";
const SESSION_EXPIRY_MS = 1000 * 60 * 60 * 24 * 30; // 30 days

interface GoogleIdTokenClaims {
  sub: string; // Google user ID
  email: string;
  name: string;
  picture?: string;
}

interface UserRecord {
  id: number;
  google_id: string;
  email: string;
  name: string;
  picture: string | null;
  tenant_name: string;
}

interface InsertResult {
  last_insert_id: number;
}

function redirectToError(request: Request, message: string): Response {
  const url = new URL("/login/error", request.url);
  url.searchParams.set("message", message);
  return NextResponse.redirect(url);
}

export async function GET(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  const state = url.searchParams.get("state");

  const cookieStore = await cookies();
  const storedState = cookieStore.get("google_oauth_state")?.value;
  const codeVerifier = cookieStore.get("google_oauth_code_verifier")?.value;

  // Validate state
  if (!code || !state || !storedState || !codeVerifier || state !== storedState) {
    return redirectToError(request, "Invalid login session. Please try again.");
  }

  try {
    // Exchange code for tokens
    const tokens = await google.validateAuthorizationCode(code, codeVerifier);
    const idToken = tokens.idToken();
    const claims = decodeIdToken(idToken) as GoogleIdTokenClaims;

    const googleId = claims.sub;
    const email = claims.email;
    const name = claims.name;
    const picture = claims.picture ?? null;

    // Check if user exists by email
    const { data: existingUser, error: fetchError } = await primaryDb
      .from<UserRecord>("users")
      .select()
      .where(eq("email", email))
      .maybeSingle();

    if (fetchError) {
      console.error("Failed to fetch user:", fetchError);
      return redirectToError(request, "Failed to check user account. Please try again.");
    }

    let userId: number;

    if (existingUser) {
      // Existing user - update profile info (including google_id in case it changed)
      userId = existingUser.id;
      const { error: updateError } = await primaryDb
        .from("users")
        .update({ google_id: googleId, email, name, picture })
        .where(eq("id", userId));

      if (updateError) {
        console.error("Failed to update user:", updateError);
        return redirectToError(request, "Failed to update user profile. Please try again.");
      }
    } else {
      // New user - create user and database
      const databaseName = `user-${googleId}`;

      // Create user record
      const { data: insertResult, error: insertError } = await primaryDb.from("users").insert({
        google_id: googleId,
        email,
        name,
        picture,
        tenant_name: databaseName,
        created_at: new Date().toISOString(),
      });

      if (insertError || !insertResult) {
        console.error("Failed to create user:", insertError);
        return redirectToError(request, "Failed to create user account. Please try again.");
      }

      userId = (insertResult as InsertResult).last_insert_id;

      // Create database for user's todos
      const { error: databaseError } = await client.databases.create({
        name: databaseName,
        template: "todos",
      });

      if (databaseError) {
        console.error("Failed to create database:", databaseError);
        return redirectToError(request, "Failed to set up your workspace. Please try again.");
      }
    }

    // Create session
    const { token } = await createSession(userId);

    // Use NextResponse to properly attach cookies to redirect
    const response = NextResponse.redirect(new URL("/dashboard", request.url));

    // Set session cookie
    response.cookies.set(SESSION_COOKIE_NAME, token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: SESSION_EXPIRY_MS / 1000,
    });

    // Clean up OAuth cookies
    response.cookies.delete("google_oauth_state");
    response.cookies.delete("google_oauth_code_verifier");

    return response;
  } catch (error) {
    console.error("OAuth callback error:", error);

    if (error instanceof OAuth2RequestError) {
      return redirectToError(request, "Login authorization failed. Please try again.");
    }

    return redirectToError(request, "An unexpected error occurred. Please try again.");
  }
}
