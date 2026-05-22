import { Injectable, inject, signal } from '@angular/core';
import { GatewayService } from '../gateway/gateway.service';
import { JobSummary, Pagination, JobType } from '../gateway/gateway';

@Injectable({
  providedIn: 'root',
})
export class JobsService {
  private gatewayService = inject(GatewayService);

  jobs = signal<JobSummary[]>([]);

  loading = signal<boolean>(true);

  error = signal<string | null>(null);

  pagination = signal<Pagination | undefined>(undefined);

  async loadJobs(nextCursor?: string, prevCursor?: string) {
    this.loading.set(true);
    this.error.set(null);

    try {
      // TODO(b/511988008): Make "user", "configuration", "jobType" fields optional.
      const response = await this.gatewayService.QueryJobList({
        user: '',
        configuration: '',
        jobType: JobType.JOB_TYPE_UNSPECIFIED,
        pagination: {
          nextCursor: nextCursor || '',
          prevCursor: prevCursor || '',
        },
      });

      const responseJobs = response.jobs || [];
      this.jobs.set(responseJobs);
      this.pagination.set(response.pagination);
    } catch (err: any) {
      console.error('Failed to load jobs:', err);
      this.error.set(err?.message || 'An unexpected error occurred.');
    } finally {
      this.loading.set(false);
    }
  }
}
