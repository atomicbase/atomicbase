import { Command } from "commander";
import { initCommand } from "./commands/init.js";
import { pushCommand } from "./commands/push.js";
import { pullCommand } from "./commands/pull.js";
import { diffCommand } from "./commands/diff.js";
import { tenantsCommand } from "./commands/tenant.js";

const program = new Command();

program
  .name("atomicbase")
  .description("CLI for Atomicbase schema management")
  .version("0.1.0")
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

program.parse();
