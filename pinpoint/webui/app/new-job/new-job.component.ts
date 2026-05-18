import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-new-job',
  standalone: true,
  imports: [RouterLink],
  templateUrl: './new-job.component.html',
})
export class NewJobComponent {
  submitJob() {
    alert('Not implemented');
  }
}
