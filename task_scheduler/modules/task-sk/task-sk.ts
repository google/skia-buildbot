/**
 * @module modules/task-sk
 * @description <h2><code>task-sk</code></h2>
 *
 * Displays basic information about a task, including a graph display of its
 * context within the job(s) which utilize it.
 *
 * @attr {string} swarming - URL of the Swarming server.
 * @attr {string} taskID - Unique ID of the task to display.
 */
import { diffDate } from 'common-sk/modules/human';
import { define } from 'elements-sk/define';
import 'elements-sk/styles/table';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from 'common-sk/modules/dom';
import {
  GetTaskSchedulerService,
  Job,
  Task,
  TaskSchedulerService,
  TaskStatus,
  GetTaskResponse,
  GetJobResponse,
} from '../rpc';
import { TaskGraphSk } from '../task-graph-sk/task-graph-sk';
import '../task-graph-sk';
import '../../../infra-sk/modules/human-date-sk';

const taskStatusToTextColor = new Map<TaskStatus, [string, string]>();
taskStatusToTextColor.set(TaskStatus.TASK_STATUS_PENDING, [
  'pending',
  'rgb(255, 255, 255)',
]);
taskStatusToTextColor.set(TaskStatus.TASK_STATUS_RUNNING, [
  'running',
  'rgb(248, 230, 180)',
]);
taskStatusToTextColor.set(TaskStatus.TASK_STATUS_SUCCESS, [
  'succeeded',
  'rgb(209, 228, 188)',
]);
taskStatusToTextColor.set(TaskStatus.TASK_STATUS_FAILURE, [
  'failed',
  'rgb(217, 95, 2)',
]);
taskStatusToTextColor.set(TaskStatus.TASK_STATUS_MISHAP, [
  'mishap',
  'rgb(117, 112, 179)',
]);

export class TaskSk extends ElementSk {
  private static template = (ele: TaskSk) => html`
    <div class="container">
      <h2>Task Information</h2>
      <table>
        <tr>
          <td>ID</td>
          <td>${ele.task!.id}</td>
        </tr>
        <tr>
          <td>Name</td>
          <td>${ele.task!.taskKey!.name}</td>
        </tr>
        <tr>
          <td>Status</td>
          <td style="background-color:${ele.statusColor}">${ele.statusText}</td>
        </tr>
        <tr>
          <td>Created</td>
          <td>
            <human-date-sk .date="${ele.task!.createdAt!}"></human-date-sk>
          </td>
        </tr>
        ${ele.task!.finishedAt
          ? html`
              <tr>
                <td>Finished</td>
                <td>
                  <human-date-sk
                    .date="${ele.task!.finishedAt!}"
                  ></human-date-sk>
                </td>
              </tr>
            `
          : html``}
        <tr>
          <td>Duration</td>
          <td>${ele.duration}</td>
        </tr>
        <tr>
          <td>Repo</td>
          <td>
            <a href="${ele.task!.taskKey!.repoState!.repo}" target="_blank"
              >${ele.task!.taskKey!.repoState!.repo}</a
            >
          </td>
        </tr>
        <tr>
          <td>Revision</td>
          <td>
            <a href="${ele.revisionLink}" target="_blank"
              >${ele.task!.taskKey!.repoState!.revision}</a
            >
          </td>
        </tr>
        <tr>
          <td>Swarming Task</td>
          <td>
            <a href="${ele.swarmingTaskLink}" target="_blank"
              >${ele.task!.swarmingTaskId}</a
            >
          </td>
        </tr>
        <tr>
          <td>Jobs</td>
          <td>
            ${ele.jobs.map(
              (job: Job) => html` <a href="/job/${job.id}">${job.name}</a> `
            )}
          </td>
        </tr>
        ${ele.isTryJob
          ? html`
              <tr>
                <td>Codereview Link</td>
                <td>
                  <a href="${ele.codereviewLink}" target="_blank"
                    >${ele.codereviewLink}</a
                  >
                </td>
              </tr>
              <tr>
                <td>Codereview Server</td>
                <td>${ele.task!.taskKey!.repoState!.patch!.server}</td>
              </tr>
              <tr>
                <td>Issue</td>
                <td>${ele.task!.taskKey!.repoState!.patch!.issue}</td>
              </tr>
              <tr>
                <td>Patchset</td>
                <td>${ele.task!.taskKey!.repoState!.patch!.patchset}</td>
              </tr>
            `
          : html``}
      </table>
    </div>

    <div class="container">
      <h2>Context</h2>
      <task-graph-sk></task-graph-sk>
    </div>
  `;

  private codereviewLink: string = '';
  private duration: string = '';
  private isTryJob: boolean = false;
  private jobs: Job[] = [];
  private revisionLink: string = '';
  private _rpc: TaskSchedulerService | null = null;
  private statusColor: string = '';
  private statusText: string = '';
  private swarmingTaskLink: string = '';
  private task: Task | null = null;

  constructor() {
    super(TaskSk.template);
  }

  get taskID(): string {
    return this.getAttribute('task-id') || '';
  }

  set taskID(taskID: string) {
    this.setAttribute('task-id', taskID);
    this.reload();
  }

  get swarming(): string {
    return this.getAttribute('swarming') || '';
  }

  set swarming(swarming: string) {
    this.setAttribute('swarming', swarming);
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

  private reload() {
    if (!this.taskID || !this.rpc) {
      return;
    }
    this.rpc!.getTask({
      id: this.taskID,
      includeStats: false,
    }).then((taskResp: GetTaskResponse) => {
      this.task = taskResp.task!;
      const start = new Date(this.task.createdAt!);
      const end = this.task.finishedAt
        ? new Date(this.task.finishedAt)
        : new Date(Date.now()); // Use Date.now so that it can be mocked.
      this.duration = diffDate(start.getTime(), end.getTime());
      const rs = this.task.taskKey!.repoState!;
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
      [this.statusText, this.statusColor] = taskStatusToTextColor.get(
        this.task.status
      )!;
      this.swarmingTaskLink = `https://${this.swarming}/task?id=${this.task.swarmingTaskId}`;
      const jobReqs = this.task.jobs!.map((jobID: string) =>
        this.rpc!.getJob({ id: jobID })
      );
      Promise.all(jobReqs).then((jobResps: GetJobResponse[]) => {
        this.jobs = jobResps
          .map((resp: GetJobResponse) => resp.job!)
          .sort((a: Job, b: Job) => (a.name < b.name ? -1 : 1));
        this._render();
        const graph = $$<TaskGraphSk>('task-graph-sk', this);
        graph?.draw(this.jobs, this.swarming, taskResp.task);
      });
    });
  }
}

define('task-sk', TaskSk);
