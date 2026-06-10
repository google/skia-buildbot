import { Component, inject, signal, computed, OnInit, Signal } from '@angular/core';
import {
  FormBuilder,
  FormGroup,
  ReactiveFormsModule,
  Validators,
  AbstractControl,
  ValidationErrors,
  ValidatorFn,
} from '@angular/forms';
import { toSignal } from '@angular/core/rxjs-interop';
import { startWith } from 'rxjs';
import { HttpErrorResponse } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatCardModule } from '@angular/material/card';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
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
    MatAutocompleteModule,
  ],
  templateUrl: './new-job.component.html',
  styleUrls: ['./new-job.component.css'],
})
export class NewJobComponent implements OnInit {
  private formBuilder = inject(FormBuilder);

  private gatewayService = inject(GatewayService);

  submitting = signal(false);

  createdJobId = signal<string>('');

  errorMessage = signal<string>('');

  bots = signal<string[]>([]);

  benchmarks = signal<string[]>([]);

  jobForm: FormGroup = this.formBuilder.group({
    jobName: [''],
    // 150 is the maximum number of attempts the legacy backend allows.
    attempts: [30, [Validators.required, Validators.min(1), Validators.max(150)]],
    bugId: ['', Validators.min(1)],
    bot: ['', [Validators.required, this.autocompleteValidator(this.bots)]],
    benchmark: ['', [Validators.required, this.autocompleteValidator(this.benchmarks)]],
    story: [''],
    storyTags: [''],
    baseline: this.formBuilder.group(variantGroupConfig(true)),
    experiment: this.formBuilder.group(variantGroupConfig()),
  });

  botQuery = this.inputFieldSignal('bot');

  filteredBots = this.filterValuesByInput(this.botQuery, this.bots);

  benchmarkQuery = this.inputFieldSignal('benchmark');

  filteredBenchmarks = this.filterValuesByInput(this.benchmarkQuery, this.benchmarks);

  ngOnInit() {
    this.loadBots();
    this.loadBenchmarks();
  }

  private async loadBots() {
    try {
      const response = await this.gatewayService.ListBotConfigurations({});
      this.bots.set(response.configurations.sort());
      this.jobForm.get('bot')?.updateValueAndValidity();
    } catch (error) {
      console.error('Failed to fetch bots: ', error);
    }
  }

  private async loadBenchmarks() {
    try {
      const response = await this.gatewayService.ListBenchmarks({});
      this.benchmarks.set(response.benchmarks.sort());
      this.jobForm.get('benchmark')?.updateValueAndValidity();
    } catch (error) {
      console.error('Failed to fetch benchmarks: ', error);
    }
  }

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

  private filterValuesByInput(input: Signal<string>, values: Signal<string[]>): Signal<string[]> {
    return computed(() => values().filter((v) => this.fuzzyMatch(input(), v)));
  }

  private fuzzyMatch(input: string, value: string): boolean {
    input = input.trim().toLowerCase();
    value = value.trim().toLowerCase();
    let j = -1;
    for (let i = 0; i < input.length; ++i) {
      j = value.indexOf(input[i], j + 1);
      if (j < 0) {
        return false;
      }
    }
    return true;
  }

  private inputFieldSignal(name: string): Signal<string> {
    const control = this.jobForm.get(name);
    if (!control) {
      throw new Error(`Input filed "${name}" not found.`);
    }
    return toSignal(control.valueChanges.pipe(startWith('')), { initialValue: '' });
  }

  private autocompleteValidator(validOptions: Signal<string[]>): ValidatorFn {
    return (control: AbstractControl): ValidationErrors | null => {
      const value = control.value;
      if (!value) {
        return null;
      }
      return validOptions().includes(value) ? null : { invalidAutocomplete: true };
    };
  }
}
