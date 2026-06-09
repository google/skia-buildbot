import { Component, inject, signal } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { HttpErrorResponse } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatCardModule } from '@angular/material/card';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { GatewayService } from '../gateway/gateway.service';
import { CreateTryJobRequest, VariantConfig } from '../gateway/gateway';

const variantGroupConfig = (commitRequired = false) => ({
  commit: ['', commitRequired ? [Validators.required] : []],
  patch: [''],
  jsFlags: [''],
  enableFeatures: [''],
  disableFeatures: [''],
  extraBrowserArgs: [''],
  benchmarkRunnerArgs: [''],
});

@Component({
  selector: 'app-new-job',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatCardModule,
    MatExpansionModule,
    MatTooltipModule,
    MatIconModule,
    MatProgressSpinnerModule,
  ],
  templateUrl: './new-job.component.html',
  styleUrls: ['./new-job.component.css'],
})
export class NewJobComponent {
  private formBuilder = inject(FormBuilder);

  private gatewayService = inject(GatewayService);

  submitting = signal(false);

  createdJobId = signal<string>('');

  errorMessage = signal<string>('');

  jobForm: FormGroup = this.formBuilder.group({
    jobName: [''],
    // 150 is the maximum number of attempts the legacy backend allows.
    attempts: [30, [Validators.required, Validators.min(1), Validators.max(150)]],
    bugId: ['', Validators.min(1)],
    bot: ['', Validators.required],
    benchmark: ['', Validators.required],
    story: [''],
    storyTags: [''],
    baseline: this.formBuilder.group(variantGroupConfig(true)),
    experiment: this.formBuilder.group(variantGroupConfig()),
  });

  private getVariantConfig(formGroup: any): VariantConfig {
    return {
      commit: formGroup.commit || '',
      patch: formGroup.patch || '',
      extraArgs: {
        benchmarkRunnerArgs: formGroup.benchmarkRunnerArgs || '',
        extraBrowserArgs: formGroup.extraBrowserArgs || '',
        jsFlags: formGroup.jsFlags || '',
        enableFeatures: formGroup.enableFeatures || '',
        disableFeatures: formGroup.disableFeatures || '',
      },
    };
  }

  async submitJob() {
    if (this.jobForm.invalid) {
      this.jobForm.markAllAsTouched();
      return;
    }

    if (this.submitting()) {
      return;
    }

    this.submitting.set(true);
    this.createdJobId.set('');
    this.errorMessage.set('');

    const form = this.jobForm.value;
    const request: CreateTryJobRequest = {
      benchmark: form.benchmark,
      configuration: form.bot,
      story: form.story,
      storyTags: form.storyTags || '',
      attemptCount: Number(form.attempts),
      base: this.getVariantConfig(form.baseline),
      experiment: this.getVariantConfig({
        ...form.experiment,
        commit: form.experiment.commit || form.baseline.commit,
      }),
      bugId: form.bugId ? Number(form.bugId) : undefined,
      jobName: form.jobName || '',
      // Let the backend to set the current user email.
      user: '',
    };

    try {
      const response = await this.gatewayService.CreateTryJob(request);
      this.createdJobId.set(response.jobId);
    } catch (error: unknown) {
      console.error('Filed creating a new job: ', error);
      if (error instanceof HttpErrorResponse) {
        this.errorMessage.set(error.error?.message || error.message);
      } else if (error instanceof Error) {
        this.errorMessage.set(error.message);
      } else {
        this.errorMessage.set('An unexpected error occurred.');
      }
    } finally {
      this.submitting.set(false);
    }
  }
}
