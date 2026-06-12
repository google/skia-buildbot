import { Component, inject, signal, computed, OnInit, Signal } from '@angular/core';
import { DatePipe } from '@angular/common';
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
import { CreateTryJobRequest, VariantConfig, BuildInfo } from '../gateway/gateway';

export enum Field {
  JobName = 'jobName',
  Attempts = 'attempts',
  BugId = 'bugId',
  Bot = 'bot',
  Benchmark = 'benchmark',
  Story = 'story',
  StoryTags = 'storyTags',
  Baseline = 'baseline',
  Experiment = 'experiment',
  Commit = 'commit',
  Patch = 'patch',
  BenchmarkRunnerArgs = 'benchmarkRunnerArgs',
  ExtraBrowserArgs = 'extraBrowserArgs',
  JsFlags = 'jsFlags',
  EnableFeatures = 'enableFeatures',
  DisableFeatures = 'disableFeatures',
}

const variantGroupConfig = (commitRequired = false) => ({
  [Field.Commit]: ['', commitRequired ? [Validators.required] : []],
  [Field.Patch]: [''],
  [Field.JsFlags]: [''],
  [Field.EnableFeatures]: [''],
  [Field.DisableFeatures]: [''],
  [Field.ExtraBrowserArgs]: [''],
  [Field.BenchmarkRunnerArgs]: [''],
});

@Component({
  selector: 'app-new-job',
  standalone: true,
  imports: [
    DatePipe,
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
  readonly Field = Field;

  private formBuilder = inject(FormBuilder);

  private gatewayService = inject(GatewayService);

  submitting = signal(false);

  createdJobId = signal<string>('');

  errorMessage = signal<string>('');

  bots = signal<string[]>([]);

  benchmarks = signal<string[]>([]);

  stories = signal<string[]>([]);

  storyTags = signal<string[]>([]);

  recentBuilds = signal<BuildInfo[]>([]);

  jobForm: FormGroup = this.formBuilder.group({
    [Field.JobName]: [''],
    // 150 is the maximum number of attempts the legacy backend allows.
    [Field.Attempts]: [30, [Validators.required, Validators.min(1), Validators.max(150)]],
    [Field.BugId]: ['', Validators.min(1)],
    [Field.Bot]: ['', [Validators.required, this.autocompleteValidator(this.bots)]],
    [Field.Benchmark]: ['', [Validators.required, this.autocompleteValidator(this.benchmarks)]],
    [Field.Story]: [''],
    [Field.StoryTags]: [''],
    [Field.Baseline]: this.formBuilder.group(variantGroupConfig(true)),
    [Field.Experiment]: this.formBuilder.group(variantGroupConfig()),
  });

  botQuery = this.inputFieldSignal(Field.Bot);

  filteredBots = this.filterValuesByInput(this.botQuery, this.bots);

  benchmarkQuery = this.inputFieldSignal(Field.Benchmark);

  filteredBenchmarks = this.filterValuesByInput(this.benchmarkQuery, this.benchmarks);

  storyQuery = this.inputFieldSignal(Field.Story);

  filteredStories = this.filterValuesByInput(this.storyQuery, this.stories);

  storyTagsQuery = this.inputFieldSignal(Field.StoryTags);

  filteredStoryTags = this.filterValuesByInput(this.storyTagsQuery, this.storyTags);

  baselineCommitQuery = this.inputFieldSignal([Field.Baseline, Field.Commit]);

  filteredBaselineCommits = this.filterBuildsByInput(this.baselineCommitQuery, this.recentBuilds);

  experimentCommitQuery = this.inputFieldSignal([Field.Experiment, Field.Commit]);

  filteredExperimentCommits = this.filterBuildsByInput(
    this.experimentCommitQuery,
    this.recentBuilds
  );

  ngOnInit() {
    this.loadBots();
    this.loadBenchmarks();
    this.setupBenchmarkChangeListener();
    this.setupBotChangeListener();
  }

  private async loadBots() {
    try {
      const response = await this.gatewayService.ListBotConfigurations({});
      this.bots.set(response.configurations.sort());
      this.jobForm.get(Field.Bot)?.updateValueAndValidity();
    } catch (error) {
      console.error('Failed to fetch bots: ', error);
    }
  }

  private async loadBenchmarks() {
    try {
      const response = await this.gatewayService.ListBenchmarks({});
      this.benchmarks.set(response.benchmarks.sort());
      this.jobForm.get(Field.Benchmark)?.updateValueAndValidity();
    } catch (error) {
      console.error('Failed to fetch benchmarks: ', error);
    }
  }

  private setupBenchmarkChangeListener() {
    this.jobForm.get(Field.Benchmark)?.valueChanges.subscribe((benchmark) => {
      this.stories.set([]);
      this.storyTags.set([]);
      if (this.benchmarks().includes(benchmark)) {
        this.loadBenchmarkDetails(benchmark);
      } else {
        this.jobForm.get(Field.Story)?.setValue('');
        this.jobForm.get(Field.StoryTags)?.setValue('');
      }
    });
  }

  private setupBotChangeListener() {
    this.jobForm.get(Field.Bot)?.valueChanges.subscribe((bot) => {
      this.recentBuilds.set([]);
      this.jobForm.get([Field.Baseline, Field.Commit])?.setValue('');
      this.jobForm.get([Field.Experiment, Field.Commit])?.setValue('');
      if (this.bots().includes(bot)) {
        this.loadRecentBuilds(bot);
      }
    });
  }

  private async loadRecentBuilds(bot: string) {
    try {
      const response = await this.gatewayService.ListRecentBuilds({ configuration: bot });
      this.recentBuilds.set(response.builds.sort((a, b) => b.buildNumber - a.buildNumber));
    } catch (error) {
      console.error('Failed to fetch recent builds: ', error);
    }
  }

  private async loadBenchmarkDetails(benchmark: string) {
    try {
      const response = await this.gatewayService.GetBenchmark({ benchmark });
      this.stories.set(response.stories);
      this.storyTags.set(response.storyTags);

      const storyField = this.jobForm.get(Field.Story)!;
      if (!response.stories.includes(storyField.value)) {
        storyField?.setValue('');
      }

      const storyTagsField = this.jobForm.get(Field.StoryTags)!;
      if (!response.storyTags.includes(storyTagsField.value)) {
        storyTagsField?.setValue('');
      }
    } catch (error) {
      console.error('Failed to fetch benchmark details: ', error);
    }
  }

  private getVariantConfig(formGroup: any): VariantConfig {
    return {
      commit: formGroup[Field.Commit] || '',
      patch: formGroup[Field.Patch] || '',
      extraArgs: {
        benchmarkRunnerArgs: formGroup[Field.BenchmarkRunnerArgs] || '',
        extraBrowserArgs: formGroup[Field.ExtraBrowserArgs] || '',
        jsFlags: formGroup[Field.JsFlags] || '',
        enableFeatures: formGroup[Field.EnableFeatures] || '',
        disableFeatures: formGroup[Field.DisableFeatures] || '',
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
      benchmark: form[Field.Benchmark],
      configuration: form[Field.Bot],
      story: form[Field.Story],
      storyTags: form[Field.StoryTags] || '',
      attemptCount: Number(form[Field.Attempts]),
      base: this.getVariantConfig(form[Field.Baseline]),
      experiment: this.getVariantConfig({
        ...form[Field.Experiment],
        commit: form[Field.Experiment][Field.Commit] || form[Field.Baseline][Field.Commit],
      }),
      bugId: form[Field.BugId] ? Number(form[Field.BugId]) : undefined,
      jobName: form[Field.JobName] || '',
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

  private inputFieldSignal(name: string | string[]): Signal<string> {
    const control = this.jobForm.get(name);
    if (!control) {
      throw new Error(`Input filed "${name}" not found.`);
    }
    return toSignal(control.valueChanges.pipe(startWith('')), { initialValue: '' });
  }

  private filterBuildsByInput(
    input: Signal<string>,
    builds: Signal<BuildInfo[]>
  ): Signal<BuildInfo[]> {
    return computed(() => {
      const query = input().trim().toLowerCase();
      return builds().filter(
        (c) => c.gitHash.startsWith(query) || c.buildNumber.toString().startsWith(query)
      );
    });
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
