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
  if (options.limit) params.set('limit', options.limit.toString());
  if (options.offset) params.set('offset', options.offset.toString());

  const response = await fetch(`/json/jobs/list?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list jobs: ${response.statusText}`);
  }
  return response.json();
}
