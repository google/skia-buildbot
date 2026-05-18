import { Component, OnInit, inject, signal } from '@angular/core';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-job-list',
  standalone: true,
  imports: [RouterLink],
  templateUrl: './job-list.component.html',
})
export class JobListComponent implements OnInit {
  private http = inject(HttpClient);

  jobsText = signal('Loading jobs...');

  ngOnInit() {
    this.http.get('/pinpoint/v1/jobs').subscribe({
      next: (data: any) => {
        this.jobsText.set(JSON.stringify(data, null, 2));
      },
      error: (err: HttpErrorResponse) => {
        this.jobsText.set('Failed to load jobs: ' + err.message);
      },
    });
  }
}
