import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { JobTableComponent } from './job-table/job-table.component';

@Component({
  selector: 'app-job-list',
  standalone: true,
  imports: [RouterLink, MatButtonModule, MatIconModule, JobTableComponent],
  templateUrl: './job-list.component.html',
  styleUrls: ['./job-list.component.css'],
})
export class JobListComponent {}
