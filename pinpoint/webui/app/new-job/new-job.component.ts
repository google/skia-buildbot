import { Component, inject } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatCardModule } from '@angular/material/card';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatTooltipModule } from '@angular/material/tooltip';

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
    RouterLink,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatCardModule,
    MatExpansionModule,
    MatTooltipModule,
  ],
  templateUrl: './new-job.component.html',
  styleUrls: ['./new-job.component.css'],
})
export class NewJobComponent {
  private formBuilder = inject(FormBuilder);

  jobForm: FormGroup = this.formBuilder.group({
    jobName: [''],
    // 150 is the maximum number of attempts the legacy backend allows.
    attempts: [30, [Validators.required, Validators.min(1), Validators.max(150)]],
    bugId: ['', Validators.min(1)],
    bot: ['', Validators.required],
    benchmark: ['', Validators.required],
    story: ['', Validators.required],
    storyTags: [''],
    baseline: this.formBuilder.group(variantGroupConfig(true)),
    experiment: this.formBuilder.group(variantGroupConfig()),
  });

  submitJob() {
    if (this.jobForm.valid) {
      console.log(JSON.stringify(this.jobForm.value, null, 2));
      alert('Job successfully created!');
    }
  }
}
