import { resolve } from "node:path";
import { config as loadEnv } from "dotenv";
import { Command } from "commander";
import { initCommand } from "./commands/init.js";
import { pushCommand } from "./commands/push.js";
import { pullCommand } from "./commands/pull.js";
import { diffCommand } from "./commands/diff.js";
import { tenantsCommand } from "./commands/tenant.js";
import { jobsCommand } from "./commands/jobs.js";

// Load environment variables from .env file in the user's working directory
// INIT_CWD is set by npm/npx to the original directory where the command was run
const workingDir = process.env.INIT_CWD || process.cwd();
loadEnv({ path: resolve(workingDir, ".env") });

// Re-export config helper for use in atomicbase.config.ts files
export { defineConfig } from "./config.js";
export type { AtomicbaseConfig } from "./config.js";

const program = new Command();

program
  .name("atomicbase")
  .description("CLI for Atomicbase schema management")
  .version("0.1.0")
  .option("-k, --insecure", "Skip SSL certificate verification")
  .hook("preAction", (thisCommand) => {
    // Set env var before config loads so all commands pick it up
    if (thisCommand.opts().insecure) {
      process.env.ATOMICBASE_INSECURE = "1";
      console.log("Warning: SSL certificate verification disabled\n");
    }
  })
  .action(() => {
    // Show help when no command is provided
    program.help();
  });

// Register commands
program.addCommand(initCommand);
program.addCommand(pushCommand);
program.addCommand(pullCommand);
program.addCommand(diffCommand);
program.addCommand(tenantsCommand);
program.addCommand(jobsCommand);

program.parse();
