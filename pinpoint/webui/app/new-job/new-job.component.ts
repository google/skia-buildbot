import { Component, inject, signal, computed, OnInit, Signal, WritableSignal } from '@angular/core';
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
import { startWith, debounceTime } from 'rxjs';
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
import {
  CreateTryJobRequest,
  VariantConfig,
  BuildInfo,
  GetCommitResponse,
} from '../gateway/gateway';

export enum Variant {
  Baseline = 'baseline',
  Experiment = 'experiment',
}

export enum Field {
  JobName = 'jobName',
  Attempts = 'attempts',
  BugId = 'bugId',
  Bot = 'bot',
  Benchmark = 'benchmark',
  Story = 'story',
  StoryTags = 'storyTags',
  Commit = 'commit',
  Patch = 'patch',
  BenchmarkRunnerArgs = 'benchmarkRunnerArgs',
  ExtraBrowserArgs = 'extraBrowserArgs',
  JsFlags = 'jsFlags',
  EnableFeatures = 'enableFeatures',
  DisableFeatures = 'disableFeatures',
}

// Delay before querying the backend during the user input.
export const INPUT_DEBOUNCE_TIME_MS = 500;

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

  readonly Variant = Variant;

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

  baselineCommitInfo = signal<GetCommitResponse | null>(null);

  experimentCommitInfo = signal<GetCommitResponse | null>(null);

  jobForm: FormGroup = this.formBuilder.group({
    [Field.JobName]: [''],
    // 150 is the maximum number of attempts the legacy backend allows.
    [Field.Attempts]: [30, [Validators.required, Validators.min(1), Validators.max(150)]],
    [Field.BugId]: ['', Validators.min(1)],
    [Field.Bot]: ['', [Validators.required, this.autocompleteValidator(this.bots)]],
    [Field.Benchmark]: ['', [Validators.required, this.autocompleteValidator(this.benchmarks)]],
    [Field.Story]: [''],
    [Field.StoryTags]: [''],
    [Variant.Baseline]: this.formBuilder.group(variantGroupConfig(true)),
    [Variant.Experiment]: this.formBuilder.group(variantGroupConfig()),
  });

  botQuery = this.inputFieldSignal(Field.Bot);

  filteredBots = this.filterValuesByInput(this.botQuery, this.bots);

  benchmarkQuery = this.inputFieldSignal(Field.Benchmark);

  filteredBenchmarks = this.filterValuesByInput(this.benchmarkQuery, this.benchmarks);

  storyQuery = this.inputFieldSignal(Field.Story);

  filteredStories = this.filterValuesByInput(this.storyQuery, this.stories);

  storyTagsQuery = this.inputFieldSignal(Field.StoryTags);

  filteredStoryTags = this.filterValuesByInput(this.storyTagsQuery, this.storyTags);

  baselineCommitQuery = this.inputFieldSignal([Variant.Baseline, Field.Commit]);

  filteredBaselineCommits = this.filterBuildsByInput(this.baselineCommitQuery, this.recentBuilds);

  experimentCommitQuery = this.inputFieldSignal([Variant.Experiment, Field.Commit]);

  filteredExperimentCommits = this.filterBuildsByInput(
    this.experimentCommitQuery,
    this.recentBuilds
  );

  ngOnInit() {
    this.loadBots();
    this.loadBenchmarks();
    this.setupBenchmarkChangeListener();
    this.setupBotChangeListener();
    this.setupCommitChangeListeners();
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
    if (this.submitting()) {
      return;
    }

    const baselineCommit = this.baselineCommitInfo()?.gitHash;
    if (!baselineCommit) {
      const baselineCommitControl = this.jobForm.get([Variant.Baseline, Field.Commit])!;
      baselineCommitControl.setErrors({ ...baselineCommitControl.errors, invalidCommit: true });
    }

    const experimentCommit = this.experimentCommitInfo()?.gitHash;
    const experimentCommitControl = this.jobForm.get([Variant.Experiment, Field.Commit])!;
    if (experimentCommitControl.value && !experimentCommit) {
      experimentCommitControl.setErrors({ ...experimentCommitControl.errors, invalidCommit: true });
    }

    if (this.jobForm.invalid) {
      this.jobForm.markAllAsTouched();
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
      base: this.getVariantConfig({ ...form[Variant.Baseline], commit: baselineCommit }),
      experiment: this.getVariantConfig({
        ...form[Variant.Experiment],
        commit: experimentCommit || baselineCommit,
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

  getClNumber(reviewUrl: string): string {
    return reviewUrl.split('/').at(-1) ?? '';
  }

  experimentPanelError(): boolean {
    return this.jobForm.get([Variant.Experiment, Field.Commit])!.invalid;
  }

  private setupCommitChangeListeners() {
    this.setupCommitListener([Variant.Baseline, Field.Commit], this.baselineCommitInfo);
    this.setupCommitListener([Variant.Experiment, Field.Commit], this.experimentCommitInfo);
  }

  private setupCommitListener(
    name: string[],
    commitInfoSignal: WritableSignal<GetCommitResponse | null>
  ) {
    const control = this.jobForm.get(name)!;

    const clearCommitError = () => {
      const errors = control.errors;
      if (errors) {
        delete errors['invalidCommit'];
        control.setErrors(Object.keys(errors).length ? errors : null);
      }
    };

    // Clear commit details and errors immediately after the value changes.
    control.valueChanges.subscribe(() => {
      commitInfoSignal.set(null);
      clearCommitError();
    });

    // Delay the backend query while the user might be typing.
    control.valueChanges
      .pipe(debounceTime(INPUT_DEBOUNCE_TIME_MS))
      .subscribe(async (value: string) => {
        const commit = value.trim();
        if (!commit) {
          return;
        }

        try {
          const resp = await this.gatewayService.GetCommit({ commit: commit });
          if (control.value === value) {
            commitInfoSignal.set(resp);
            clearCommitError();
          }
        } catch (error) {
          if (control.value === value) {
            console.error(`Failed to fetch commit info for ${commit}: `, error);
            commitInfoSignal.set(null);
            control.setErrors({ ...control.errors, invalidCommit: true });
          }
        }
      });
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
