//Contains functions for interacting with the Pinpoint backend API.

export interface Job {
  job_id: string;
  job_name: string;
  job_type: string;
  benchmark: string;
  created_date: string;
  job_status: string;
  bot_name: string;
  user: string;
}

export interface ListJobsOptions {
  searchTerm?: string;
  benchmark?: string;
  botName?: string;
  user?: string;
  limit?: number;
  offset?: number;
}

/**
 * Fetches a list of jobs from the backend.
 * @param options - The query parameters for the request.
 * @returns A promise that resolves to an array of jobs.
 */
export async function listJobs(options: ListJobsOptions): Promise<Job[]> {
  const params = new URLSearchParams();
  if (options.searchTerm) params.set('search_term', options.searchTerm);
  if (options.benchmark) params.set('benchmark', options.benchmark);
  if (options.botName) params.set('bot_name', options.botName);
  if (options.user) params.set('user', options.user);
  if (options.limit) params.set('limit', options.limit.toString());
  if (options.offset) params.set('offset', options.offset.toString());

  const response = await fetch(`/json/jobs/list?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list jobs: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of benchmarks to run jobs against.
 * @returns A promise that resolves to an array of benchmarks.
 */
export async function listBenchmarks(): Promise<string[]> {
  const response = await fetch(`/benchmarks`);
  if (!response.ok) {
    throw new Error(`Failed to list benchmarks: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of bots to run jobs on based on a chosen benchmark.
 * If given an empty benchmark, the function will return all bots.
 * @returns A promise that resolves to an array of strings.
 */
export async function listBots(benchmark: string): Promise<string[]> {
  const params = new URLSearchParams();
  params.set('benchmark', benchmark);

  const response = await fetch(`/bots?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list bots: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of stories to run jobs on based on a chosen benchmark.
 * @returns A promise that resolves to an array of strings.
 */
export async function listStories(benchmark: string): Promise<string[]> {
  const params = new URLSearchParams();
  if (benchmark === '') {
    throw new Error(`Failed to list stories: No benchmark provided`);
  }

  params.set('benchmark', benchmark);
  const response = await fetch(`/stories?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list stories: ${response.statusText}`);
  }
  return response.json();
}
