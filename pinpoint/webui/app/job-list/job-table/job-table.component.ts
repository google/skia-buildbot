import { Component, OnInit, AfterViewInit, inject, ViewChild } from '@angular/core';
import { DatePipe } from '@angular/common';
import { MatTableModule, MatTableDataSource } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatPaginator, MatPaginatorModule } from '@angular/material/paginator';
import { MatFormFieldModule } from '@angular/material/form-field';
import { JobSummary, JobType, JobStatus } from '../../gateway/gateway';
import { JobsService } from '../jobs.service';

export enum JobTableColumn {
  Name = 'name',
  Benchmark = 'benchmark',
  Configuration = 'configuration',
  Story = 'story',
  JobType = 'jobType',
  User = 'user',
  Created = 'created',
  JobStatus = 'jobStatus',
}

@Component({
  selector: 'app-job-table',
  standalone: true,
  imports: [
    MatTableModule,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatProgressBarModule,
    MatPaginatorModule,
    MatFormFieldModule,
    DatePipe,
  ],
  templateUrl: './job-table.component.html',
  styleUrls: ['./job-table.component.css'],
})
export class JobTableComponent implements OnInit, AfterViewInit {
  readonly JobTableColumn = JobTableColumn;

  private jobsService = inject(JobsService);

  jobs = this.jobsService.jobs;

  loading = this.jobsService.loading;

  error = this.jobsService.error;

  pagination = this.jobsService.pagination;

  dataSource = new MatTableDataSource<JobSummary>([]);

  displayedColumns: string[] = [
    JobTableColumn.Name,
    JobTableColumn.Benchmark,
    JobTableColumn.Configuration,
    JobTableColumn.Story,
    JobTableColumn.JobType,
    JobTableColumn.User,
    JobTableColumn.Created,
    JobTableColumn.JobStatus,
  ];

  @ViewChild(MatPaginator) paginator!: MatPaginator;

  ngOnInit() {
    this.loadJobs();
  }

  ngAfterViewInit() {
    this.dataSource.paginator = this.paginator;
  }

  async loadJobs(nextCursor?: string, prevCursor?: string) {
    await this.jobsService.loadJobs(nextCursor, prevCursor);
    this.dataSource.data = this.jobs();
  }

  jobStatusToLabel(status: JobStatus): string {
    switch (status) {
      case JobStatus.JOB_STATUS_QUEUED:
        return '⏳ Queued';
      case JobStatus.JOB_STATUS_RUNNING:
        return '🔄 Running';
      case JobStatus.JOB_STATUS_COMPLETED:
        return '✅ Completed';
      case JobStatus.JOB_STATUS_FAILED:
        return '❌ Failed';
      case JobStatus.JOB_STATUS_CANCELLED:
        return '🚫 Cancelled';
      case JobStatus.JOB_STATUS_UNSPECIFIED:
      case JobStatus.UNRECOGNIZED:
      default:
        console.error('Unknown job status:', status);
        return '-';
    }
  }

  getJobTypeLabel(jobType: JobType): string {
    switch (jobType) {
      case JobType.JOB_TYPE_BISECT:
        return 'Bisect';
      case JobType.JOB_TYPE_TRY:
        return 'Try';
      case JobType.JOB_TYPE_UNSPECIFIED:
      case JobType.UNRECOGNIZED:
      default:
        console.error('Unknown job type:', jobType);
        return '-';
    }
  }
}
