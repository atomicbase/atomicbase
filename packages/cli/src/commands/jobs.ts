import { Command } from "commander";
import { loadConfig } from "../config.js";
import { ApiClient, ApiError, type Job } from "../api.js";

/**
 * Format a date string for display.
 */
function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return "-";
  const date = new Date(dateStr);
  return date.toLocaleString();
}

/**
 * Get status display with state.
 */
function formatStatus(job: Job): string {
  if (job.status === "complete" && job.state) {
    return `${job.status} (${job.state})`;
  }
  return job.status;
}

/**
 * List all jobs.
 */
async function listJobs(options: { status?: string }): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  try {
    const jobs = await api.listJobs(options.status);

    if (jobs.length === 0) {
      console.log("No jobs found.");
      return;
    }

    console.log("Jobs:\n");
    console.log("  ID    TEMPLATE   VERSION      STATUS              PROGRESS     CREATED");
    console.log("  " + "-".repeat(80));

    for (const job of jobs) {
      const id = String(job.id).padEnd(5);
      const templateId = String(job.templateId).padEnd(10);
      const version = `v${job.fromVersion} -> v${job.toVersion}`.padEnd(12);
      const status = formatStatus(job).padEnd(19);
      const progress = job.totalDbs > 0
        ? `${job.completedDbs}/${job.totalDbs}`.padEnd(12)
        : "-".padEnd(12);
      const created = formatDate(job.createdAt);
      console.log(`  ${id} ${templateId} ${version} ${status} ${progress} ${created}`);
    }

    console.log(`\n  Total: ${jobs.length} job(s)`);
  } catch (err) {
    console.error("Failed to list jobs:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Get job details.
 */
async function getJob(jobId: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  const id = parseInt(jobId, 10);
  if (isNaN(id)) {
    console.error("Invalid job ID. Must be a number.");
    process.exit(1);
  }

  try {
    const job = await api.getJob(id);

    console.log(`Job #${job.id}\n`);
    console.log(`  Template ID:    ${job.templateId}`);
    console.log(`  Version:        v${job.fromVersion} -> v${job.toVersion}`);
    console.log(`  Status:         ${formatStatus(job)}`);
    console.log(`  Progress:       ${job.completedDbs}/${job.totalDbs} completed, ${job.failedDbs} failed`);
    console.log(`  Created:        ${formatDate(job.createdAt)}`);
    if (job.startedAt) {
      console.log(`  Started:        ${formatDate(job.startedAt)}`);
    }
    if (job.completedAt) {
      console.log(`  Completed:      ${formatDate(job.completedAt)}`);
    }

    if (job.sql && job.sql.length > 0) {
      console.log(`\n  SQL Statements (${job.sql.length}):`);
      for (const stmt of job.sql) {
        console.log(`    ${stmt}`);
      }
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      console.error(`Job #${jobId} not found.`);
      process.exit(1);
    }
    console.error("Failed to get job:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

/**
 * Retry a failed job.
 */
async function retryJob(jobId: string): Promise<void> {
  const config = await loadConfig();
  const api = new ApiClient(config);

  const id = parseInt(jobId, 10);
  if (isNaN(id)) {
    console.error("Invalid job ID. Must be a number.");
    process.exit(1);
  }

  console.log(`Retrying job #${id}...`);

  try {
    const result = await api.retryJob(id);

    if (result.retriedCount === 0) {
      console.log("No failed tenants to retry.");
    } else {
      console.log(`✓ Retrying ${result.retriedCount} tenant(s)`);
      console.log(`  New job ID: ${result.jobId}`);
    }
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 404) {
        console.error(`Job #${jobId} not found.`);
        process.exit(1);
      }
      if (err.code === "ATOMICBASE_BUSY") {
        console.error("A migration is already in progress. Wait for it to complete.");
        process.exit(1);
      }
    }
    console.error("Failed to retry job:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

// Main jobs command with subcommands
export const jobsCommand = new Command("jobs")
  .description("Manage migration jobs")
  .argument("[job_id]", "Job ID to get details for")
  .option("-s, --status <status>", "Filter by status (pending, running, complete)")
  .action(async (jobId?: string, options?: { status?: string }) => {
    if (jobId) {
      // atomicbase jobs <job_id>
      await getJob(jobId);
    } else {
      // atomicbase jobs (list all)
      await listJobs(options ?? {});
    }
  });

// jobs list
jobsCommand
  .command("list")
  .alias("ls")
  .description("List all jobs")
  .option("-s, --status <status>", "Filter by status (pending, running, complete)")
  .action((options: { status?: string }) => listJobs(options));

// jobs retry <job_id>
jobsCommand
  .command("retry <job_id>")
  .description("Retry failed tenants in a job")
  .action(retryJob);
