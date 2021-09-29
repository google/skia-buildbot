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
import { toParamSet, fromParamSet } from 'common-sk/modules/query';

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
      ${ele.jobs.map(
    (job: TriggerJob, index: number) => html`
      <tr>
        <td>
          <input
              class="job_specs_input"
              type="text"
              .value="${job.jobName}"
              @change="${(ev: any) => {
      job.jobName = ev.currentTarget.value;
      ele.updateURL();
    }}"
              >
          </input>
        </td>
        <td>
          <input
              class="commit_input"
              type="text"
              .value="${job.commitHash}"
              @change="${(ev: any) => {
      job.commitHash = ev.currentTarget.value;
      ele.updateURL();
    }}"
              >
          </input>
        </td>
        <td>
          <button @click="${() => {
      ele.removeJob(index);
    }}">
            <delete-icon-sk></delete-icon-sk>
          </button>
        </td>
      </tr>
    `,
  )}
    </table>
    <button @click="${ele.addJob}">
      <add-icon-sk></add-icon-sk>
    </button>
    <button @click="${ele.triggerJobs}">
      <send-icon-sk></send-icon-sk>
      Trigger Jobs
    </button>
    ${ele.triggeredJobs && ele.triggeredJobs.length > 0
    ? html`
          <div class="container">
            <h2>Triggered Jobs</h2>
            ${ele.triggeredJobs.map(
      (job: TriggeredJob) => html`
                <div class="triggered_job">
                  <a href="/job/${job.id}">${job.name} @ ${job.commit}</a>
                </div>
              `,
    )}
          </div>
        `
    : html``}
  `;

  private initialLoad: boolean = true;

  private waitingForRPCs: boolean = false;

  private jobs: TriggerJob[] = [{ jobName: '', commitHash: '' }];

  private _rpc: TaskSchedulerService | null = null;

  private triggeredJobs: TriggeredJob[] = [];

  constructor() {
    super(JobTriggerSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    if (this.initialLoad && window.location.search) {
      const params = toParamSet(window.location.search.substring(1));
      const jobs = params.job;
      if (jobs) {
        this.jobs = jobs.map((jobStr: string) => {
          const split = jobStr.split('@');
          return {
            jobName: split[0],
            commitHash: split[1],
          };
        });

        const submit = params.submit;
        if (submit && submit[0] == 'true') {
          if (this.rpc) {
            this.triggerJobs();
          } else {
            this.waitingForRPCs = true;
          }
        }
      }
    }
    this.initialLoad = false;
    this._render();
  }

  get rpc(): TaskSchedulerService | null {
    return this._rpc;
  }

  set rpc(rpc: TaskSchedulerService | null) {
    this._rpc = rpc;
    if (this.waitingForRPCs) {
      this.triggerJobs();
    }
  }

  private addJob() {
    this.jobs.push({ jobName: '', commitHash: '' });
    this.updateURL();
    this._render();
  }

  private removeJob(index: number) {
    this.jobs.splice(index, 1);
    this.updateURL();
    this._render();
  }

  private updateURL() {
    const ps = {
      job: this.jobs
        .filter((job: TriggerJob) => job.jobName && job.commitHash)
        .map((job: TriggerJob) => `${job.jobName}@${job.commitHash}`),
    };
    if (ps.job.length > 0) {
      const url = `${window.location.origin
        + window.location.pathname
      }?${
        fromParamSet(ps)}`;
      window.history.pushState({ path: url }, '', url);
    }
  }

  private triggerJobs() {
    if (!this.rpc) {
      return;
    }
    const jobs = this.jobs.filter(
      (job: TriggerJob) => job.jobName && job.commitHash,
    );
    if (jobs.length == 0) {
      return;
    }
    const req: TriggerJobsRequest = {
      jobs: jobs,
    };
    this.rpc.triggerJobs(req).then((resp: TriggerJobsResponse) => {
      this.triggeredJobs = resp.jobIds!.map((id: string, index: number) => ({
        name: this.jobs[index].jobName,
        commit: this.jobs[index].commitHash,
        id: id,
      }));
      // TODO(borenet): If I render with an empty TriggerJob, the values of the
      // input fields spill over from the previous set of jobs, so I first
      // render with an empty list.
      this.jobs = [];
      this._render();
      this.jobs = [{ jobName: '', commitHash: '' }];
      this.updateURL();
      this._render();
    });
  }
}

define('job-trigger-sk', JobTriggerSk);
