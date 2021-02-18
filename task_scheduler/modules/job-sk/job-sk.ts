/**
 * @module modules/job-sk
 * @description <h2><code>job-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { diffDate } from 'common-sk/modules/human';
import { define } from 'elements-sk/define';
import 'elements-sk/styles/table';
import 'elements-sk/styles/buttons';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from 'common-sk/modules/dom';
import {
  GetTaskSchedulerService,
  Job,
  TaskSchedulerService,
  GetJobResponse,
  JobStatus,
  CancelJobResponse,
} from '../rpc';
import { TaskGraphSk } from '../task-graph-sk/task-graph-sk';
import '../task-graph-sk';
import '../../../infra-sk/modules/human-date-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/search-icon-sk';
import 'elements-sk/icon/timeline-icon-sk';

const jobStatusToTextClass = new Map<JobStatus, [string, string]>();
jobStatusToTextClass.set(JobStatus.JOB_STATUS_IN_PROGRESS, [
  'in progress',
  'bg-in-progress',
]);
jobStatusToTextClass.set(JobStatus.JOB_STATUS_SUCCESS, [
  'succeeded',
  'bg-success',
]);
jobStatusToTextClass.set(JobStatus.JOB_STATUS_FAILURE, [
  'failed',
  'bg-failure',
]);
jobStatusToTextClass.set(JobStatus.JOB_STATUS_MISHAP, ['mishap', 'bg-mishap']);
jobStatusToTextClass.set(JobStatus.JOB_STATUS_CANCELED, [
  'canceled',
  'bg-canceled',
]);

export class JobSk extends ElementSk {
  private static template = (ele: JobSk) => html`
    <div>
      <h2>Job ${ele.job!.name}</h2>
      <button>
        <timeline-icon-sk></timeline-icon-sk>
        <a href="/job/${ele.job!.id}/timeline">View Timeline</a>
      </button>
      ${ele.job!.status == JobStatus.JOB_STATUS_IN_PROGRESS
        ? html`
            <button id="cancelButton" @click="${() => ele.cancel()}">
              <delete-icon-sk></delete-icon-sk>
              Cancel Job
            </button>
          `
        : html``}
      <table>
        <tr>
          <td>ID</td>
          <td>${ele.job!.id}</td>
          <td></td>
        </tr>
        <tr>
          <td>Name</td>
          <td>${ele.job!.name}</td>
          <td>
            <a
              href="/jobs/search?name=${encodeURIComponent(ele.job!.name)}"
              target="_blank"
            >
              <button><search-icon-sk></search-icon-sk>Search Jobs</button>
            </a>
          </td>
        </tr>
        <tr>
          <td>Status</td>
          <td class="${ele.statusClass}">${ele.statusText}</td>
          <td></td>
        </tr>
        <tr>
          <td>Requested</td>
          <td>
            <human-date-sk .date="${ele.job!.requestedAt!}"></human-date-sk>
          </td>
          <td></td>
        </tr>
        <tr>
          <td>Created</td>
          <td>
            <human-date-sk .date="${ele.job!.createdAt!}"></human-date-sk>
          </td>
          <td></td>
        </tr>
        ${ele.job!.finishedAt && new Date(ele.job!.finishedAt).getTime() > 0
          ? html`
              <tr>
                <td>Finished</td>
                <td>
                  <human-date-sk .date="${ele.job!.finishedAt!}">
                  </human-date-sk>
                </td>
                <td></td>
              </tr>
            `
          : html``}
        <tr>
          <td>Duration</td>
          <td>${ele.duration}</td>
          <td></td>
        </tr>
        <tr>
          <td>Repo</td>
          <td>
            <a href="${ele.job!.repoState!.repo}" target="_blank">
              ${ele.job!.repoState!.repo}
            </a>
          </td>
          <td></td>
        </tr>
        <tr>
          <td>Revision</td>
          <td>
            <a href="${ele.revisionLink}" target="_blank">
              ${ele.job!.repoState!.revision}
            </a>
          </td>
          <td>
            <a
              href="/jobs/search?revision=${encodeURIComponent(
                ele.job!.repoState!.revision
              )}"
              target="_blank"
            >
              <button><search-icon-sk></search-icon-sk>Search Jobs</button>
            </a>
          </td>
        </tr>
        ${ele.isTryJob
          ? html`
              <tr>
                <td>Codereview Link</td>
                <td>
                  <a href="${ele.codereviewLink}" target="_blank">
                    ${ele.codereviewLink}
                  </a>
                </td>
                <td>
                  <a
                    href="/jobs/search?server=${encodeURIComponent(
                      ele.job!.repoState!.patch!.server
                    )}&issue=${encodeURIComponent(
                      ele.job!.repoState!.patch!.issue
                    )}&patchset=${encodeURIComponent(
                      ele.job!.repoState!.patch!.patchset
                    )}"
                    target="_blank"
                  >
                    <button>
                      <search-icon-sk></search-icon-sk>Search Jobs
                    </button>
                  </a>
                </td>
              </tr>
              <tr>
                <td>Codereview Server</td>
                <td>${ele.job!.repoState!.patch!.server}</td>
                <td></td>
              </tr>
              <tr>
                <td>Issue</td>
                <td>${ele.job!.repoState!.patch!.issue}</td>
                <td></td>
              </tr>
              <tr>
                <td>Patchset</td>
                <td>${ele.job!.repoState!.patch!.patchset}</td>
                <td></td>
              </tr>
            `
          : html``}
        ${ele.job!.buildbucketBuildId ? html`
          <tr>
            <td>Buildbucket Build ID</td>
            <td>${ele.job!.buildbucketBuildId}</td>
            <td></td>
          </tr>
        `: html``}
        <tr>
          <td>Manually forced</td>
          <td>${ele.job!.isForce ? 'true' : 'false'}</td>
          <td></td>
        </tr>
      </table>
    </div>

    <div>
      <h2>Tasks</h2>
      <task-graph-sk></task-graph-sk>
    </div>
  `;

  private codereviewLink: string = '';
  private duration: string = '';
  private isTryJob: boolean = false;
  private job: Job | null = null;
  private revisionLink: string = '';
  private _rpc: TaskSchedulerService | null = null;
  private statusClass: string = '';
  private statusText: string = '';

  constructor() {
    super(JobSk.template);
  }

  get jobID(): string {
    return this.getAttribute('job-id') || '';
  }

  set jobID(jobID: string) {
    this.setAttribute('job-id', jobID);
    this.reload();
  }

  get swarming(): string {
    return this.getAttribute('swarming') || '';
  }

  set swarming(swarming: string) {
    this.setAttribute('swarming', swarming);
    this._render();
  }

  get rpc(): TaskSchedulerService | null {
    return this._rpc;
  }

  set rpc(rpc: TaskSchedulerService | null) {
    this._rpc = rpc;
  }

  connectedCallback() {
    super.connectedCallback();
    this.rpc = GetTaskSchedulerService(this);
    this.reload();
  }

  private updateFrom(job: Job) {
    this.job = job;
    const start = new Date(this.job.createdAt!);
    const end =
      this.job.finishedAt && new Date(this.job.finishedAt).getTime() > 0
        ? new Date(this.job.finishedAt)
        : new Date(Date.now()); // Use Date.now so that it can be mocked.
    this.duration = diffDate(start.getTime(), end.getTime());
    const rs = this.job.repoState!;
    this.revisionLink = `${rs.repo}/+show/${rs.revision}`;
    if (
      rs.patch &&
      rs.patch.issue != '' &&
      rs.patch.patchset != '' &&
      rs.patch.server != ''
    ) {
      this.isTryJob = true;
      const p = rs.patch!;
      this.codereviewLink = `${p.server}/c/${p.issue}/${p.patchset}`;
    }
    [this.statusText, this.statusClass] = jobStatusToTextClass.get(
      this.job.status
    )!;
    this._render();
    const graph = $$<TaskGraphSk>('task-graph-sk', this);
    graph?.draw([this.job], this.swarming);
  }

  private reload() {
    if (!this.jobID || !this.rpc) {
      return;
    }
    this.rpc!.getJob({
      id: this.jobID,
    }).then((jobResp: GetJobResponse) => {
      this.updateFrom(jobResp.job!);
    });
  }

  private cancel() {
    if (!this.job) {
      return;
    }
    this.rpc
      ?.cancelJob({
        id: this.job.id,
      })
      .then((resp: CancelJobResponse) => {
        this.updateFrom(resp.job!);
      });
  }
}

define('job-sk', JobSk);
