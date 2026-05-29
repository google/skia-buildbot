import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { JobTableComponent } from './job-table/job-table.component';
import { ColumnSelectorComponent } from './column-selector/column-selector.component';
import { JobsService } from './jobs.service';

@Component({
  selector: 'app-job-list',
  standalone: true,
  imports: [
    RouterLink,
    MatButtonModule,
    MatIconModule,
    MatButtonToggleModule,
    JobTableComponent,
    ColumnSelectorComponent,
  ],
  templateUrl: './job-list.component.html',
  styleUrls: ['./job-list.component.css'],
})
export class JobListComponent {
  private jobsService = inject(JobsService);

  readonly showOnlyUserJobs = this.jobsService.showOnlyUserJobs;

  onToggleChange(value: boolean) {
    this.jobsService.setShowOnlyUserJobs(value);
  }
}
