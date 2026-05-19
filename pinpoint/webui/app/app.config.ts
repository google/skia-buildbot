import { ApplicationConfig, provideZonelessChangeDetection } from '@angular/core';
import { provideRouter, Routes } from '@angular/router';
import { provideHttpClient } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { JobListComponent } from './job-list/job-list.component';
import { NewJobComponent } from './new-job/new-job.component';

const routes: Routes = [
  { path: '', component: JobListComponent },
  { path: 'new', component: NewJobComponent },
];

export const appConfig: ApplicationConfig = {
  providers: [
    provideZonelessChangeDetection(),
    provideRouter(routes),
    provideHttpClient(),
    provideAnimationsAsync(),
  ],
};
