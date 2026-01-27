import { Google } from "arctic";

let _google: Google | null = null;

export function getGoogle(): Google {
  if (!_google) {
    if (!process.env.GOOGLE_CLIENT_ID || !process.env.GOOGLE_CLIENT_SECRET) {
      throw new Error("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required");
    }
    const redirectUri = `${process.env.NEXT_PUBLIC_APP_URL}/login/google/callback`;
    _google = new Google(
      process.env.GOOGLE_CLIENT_ID,
      process.env.GOOGLE_CLIENT_SECRET,
      redirectUri
    );
  }
  return _google;
}

// For backwards compatibility - lazy proxy
export const google = new Proxy({} as Google, {
  get(_, prop) {
    const instance = getGoogle();
    const value = instance[prop as keyof Google];
    if (typeof value === "function") {
      return value.bind(instance);
    }
    return value;
  },
});
