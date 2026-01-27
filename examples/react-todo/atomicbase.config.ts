import { defineConfig } from "@atomicbase/cli";

export default defineConfig({
  schemas: "./schemas",
  url: process.env.ATOMICBASE_URL
});
