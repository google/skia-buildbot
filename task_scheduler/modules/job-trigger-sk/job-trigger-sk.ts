/**
 * @module modules/job-trigger-sk
 * @description <h2><code>job-trigger-sk</code></h2>
 *
 * Provides an interface for triggering jobs.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  TriggerJob,
  TaskSchedulerService,
  GetTaskSchedulerService,
  TriggerJobsRequest,
  TriggerJobsResponse,
 } from '../rpc';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/send-icon-sk';
import 'elements-sk/styles/buttons';

interface TriggeredJob {
  name: string;
  commit: string;
  id: string;
}

export class JobTriggerSk extends ElementSk {
  private static template = (ele: JobTriggerSk) => html`
  <table>
    <tr>
      <th>Job</th>
      <th>Commit</th>
    </tr>
    ${ele.jobs.map((job: TriggerJob, index: number) => html`
      <tr>
        <td>
          <input
              class="job_specs_input"
              type="text"
              .value="${job.jobName}"
              @change="${(ev: any) => {job.jobName = ev.currentTarget.value}}"
              >
          </input>
        </td>
        <td>
          <input
              class="commit_input"
              type="text"
              .value="${job.commitHash}"
              @change="${(ev: any) => {job.commitHash = ev.currentTarget.value}}"
              >
          </input>
        </td>
        <td>
          <button @click="${() => {ele.removeJob(index)}}">
            <delete-icon-sk></delete-icon-sk>
          </button>
        </td>
      </tr>
    `)}
  </table>
  <button @click="${ele.addJob}">
    <add-icon-sk></add-icon-sk>
  </button>
  <button @click="${ele.triggerJobs}">
    <send-icon-sk></send-icon-sk>
    Trigger Jobs
  </button>
  ${ele.triggeredJobs && ele.triggeredJobs.length > 0 ? html`
    <div class="container">
      <h2>Triggered Jobs</h2>
      ${ele.triggeredJobs.map((job: TriggeredJob) => html`
        <div class="triggered_job">
          <a href="/job/${job.id}">${job.name} @ ${job.commit}</a>
        </div>
      `)}
    </div>
  ` : html``}
  `;

  private jobs: TriggerJob[] = [{jobName: "", commitHash: ""}];
  private rpc: TaskSchedulerService = GetTaskSchedulerService(this);
  private triggeredJobs: TriggeredJob[] = [];

  constructor() {
    super(JobTriggerSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private addJob() {
    this.jobs.push({jobName: "", commitHash: ""});
    this._render();
  }

  private removeJob(index: number) {
    this.jobs.splice(index, 1);
    this._render();
  }

  private triggerJobs() {
    const req: TriggerJobsRequest = {
      jobs: this.jobs,
    }
    this.rpc.triggerJobs(req).then((resp: TriggerJobsResponse) => {
      this.triggeredJobs = resp.jobIds!.map((id: string, index: number) => {
        return {
          name: this.jobs[index].jobName,
          commit: this.jobs[index].commitHash,
          id: id,
        };
      });
      // TODO(borenet): If I render with an empty TriggerJob, the values of the
      // input fields spill over from the previous set of jobs, so I first
      // render with an empty list.
      this.jobs = [];
      this._render();
      this.jobs = [{jobName: "", commitHash: ""}];
      this._render();
    })
  }
};

define('job-trigger-sk', JobTriggerSk);
