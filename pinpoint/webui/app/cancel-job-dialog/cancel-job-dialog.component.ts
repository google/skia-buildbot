import { Component, inject, signal } from '@angular/core';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { JobsService } from '../job-list/jobs.service';

@Component({
  selector: 'app-cancel-job-dialog',
  standalone: true,
  imports: [MatDialogModule, MatButtonModule, MatIconModule, MatProgressSpinnerModule],
  templateUrl: './cancel-job-dialog.component.html',
  styleUrls: ['./cancel-job-dialog.component.css'],
})
export class CancelJobDialogComponent {
  private dialogRef = inject(MatDialogRef<CancelJobDialogComponent>);

  readonly data = inject<{ jobId: string; jobName: string }>(MAT_DIALOG_DATA);

  private jobsService = inject(JobsService);

  private _loading = signal<boolean>(false);

  readonly loading = this._loading.asReadonly();

  private _error = signal<string>('');

  readonly error = this._error.asReadonly();

  async onConfirm() {
    this._loading.set(true);
    this._error.set('');
    try {
      await this.jobsService.cancelJob(this.data.jobId);
      this.dialogRef.close(true);
    } catch (err: any) {
      this._error.set(err?.message || 'Failed to cancel the job. Please try again.');
    } finally {
      this._loading.set(false);
    }
  }

  onDismiss() {
    this.dialogRef.close(false);
  }
}
