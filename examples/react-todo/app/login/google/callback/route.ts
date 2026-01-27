import { decodeIdToken, OAuth2RequestError } from "arctic";
import { google } from "@/lib/google";
import { cookies } from "next/headers";
import { primaryDb, client } from "@/lib/db";
import { createSession, setSessionCookie } from "@/lib/session";
import { eq } from "@atomicbase/sdk";

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

export async function GET(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  const state = url.searchParams.get("state");

  const cookieStore = await cookies();
  const storedState = cookieStore.get("google_oauth_state")?.value;
  const codeVerifier = cookieStore.get("google_oauth_code_verifier")?.value;

  // Validate state
  if (!code || !state || !storedState || !codeVerifier || state !== storedState) {
    return new Response("Invalid OAuth state", { status: 400 });
  }

  // Clean up OAuth cookies
  cookieStore.delete("google_oauth_state");
  cookieStore.delete("google_oauth_code_verifier");

  try {
    // Exchange code for tokens
    const tokens = await google.validateAuthorizationCode(code, codeVerifier);
    const idToken = tokens.idToken();
    const claims = decodeIdToken(idToken) as GoogleIdTokenClaims;

    const googleId = claims.sub;
    const email = claims.email;
    const name = claims.name;
    const picture = claims.picture ?? null;

    // Check if user exists
    const { data: existingUser } = await primaryDb
      .from<UserRecord>("users")
      .select()
      .where(eq("google_id", googleId))
      .maybeSingle();

    let userId: number;

    if (existingUser) {
      // Existing user - update profile info
      userId = existingUser.id;
      await primaryDb.from("users").update({ email, name, picture }).where(eq("id", userId));
    } else {
      // New user - create user and tenant database
      const tenantName = `user-${googleId}`;

      // Create user record
      const { data: insertResult } = await primaryDb.from("users").insert({
        google_id: googleId,
        email,
        name,
        picture,
        tenant_name: tenantName,
      });

      if (!insertResult) {
        throw new Error("Failed to create user");
      }

      userId = (insertResult as InsertResult).last_insert_id;

      // Create tenant database for user's todos
      await client.tenants.create({
        name: tenantName,
        template: "tenant",
      });
    }

    // Create session
    const { token } = await createSession(userId);
    await setSessionCookie(token);

    return Response.redirect(new URL("/dashboard", request.url));
  } catch (error) {
    console.error("OAuth callback error:", error);

    if (error instanceof OAuth2RequestError) {
      return new Response("Invalid OAuth code", { status: 400 });
    }

    return new Response("Internal server error", { status: 500 });
  }
}
