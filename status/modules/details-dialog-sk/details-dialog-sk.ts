import { $$ } from 'common-sk/modules/dom';
/**
 * @module modules/details-dialog-sk
 * @description <h2><code>details-dialog-sk</code></h2>
 *
 * @evt
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Commit, TaskSpec, TaskSpecDetails } from '../commits-data-sk/commits-data-sk';
import { Task, Comment } from '../rpc';
import { until } from 'lit-html/directives/until.js';

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import '../comments-sk';
import 'elements-sk/styles/buttons';
import { CommentData } from '../comments-sk/comments-sk';
import '../../../infra-sk/modules/task-driver-sk';
import { swarmingUrl, taskSchedulerUrl } from '../settings';
import 'elements-sk/icon/launch-icon-sk';
import { errorMessage } from 'elements-sk/errorMessage';

export interface DisplayCommit {
  shortHash: string;
  shortAuthor: string;
  shortSubject: string;
}

interface Action {
  buttonText: string;
  handler: () => void;
}

export class DetailsDialogSk extends ElementSk {
  private static template = (el: DetailsDialogSk) =>
    html`
      <div class="dialog">
        <div class="content horizontal layout wrap">
          <div class="selection-summary flex">${el.top}</div>
          ${el.action
            ? html`<button class="show-matches action" @click=${el.action.handler}>
                ${el.action.buttonText}
              </button>`
            : html``}
        </div>
        <br />
        <hr />
        ${el.middle}
        <br />
        <hr />
        <div class="buttomPanel flexRows">
          <comments-sk
            .commentData=${el.commentData}
            .allowAdd=${true}
            .allowDelete=${true}
            .showIgnoreFailure=${true}
            .showFlaky=${true}
            .editRights=${true}
          ></comments-sk>
        </div>
      </div>
    `;

  private commentData?: CommentData;
  private _repo: string = '';
  private top: TemplateResult = html`<p>Hello world, this is the top section</p>`;
  private middle: TemplateResult = html`<p>Hollo again, this is the middle section</p>`;
  private action: Action | null = null;

  constructor() {
    super(DetailsDialogSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private open() {
    this._render();
    //($$('dialog', this) as HTMLDialogElement).showModal();
    (<HTMLElement>this).style.display = 'block';
  }

  close() {
    this.action = null;
    this.top = html``;
    this.middle = html``;

    (<HTMLElement>this).style.display = 'none';
  }

  displayTask(task: Task, comments: Array<Comment>, commitsByHash: Map<string, DisplayCommit>) {
    this.commentData = {
      comments: comments,
      taskId: task.id,
      commit: '',
      taskSpec: '',
      repo: this.repo,
    };
    const td = fetch(`/json/td/${task.id}`)
      .then(jsonOrThrow)
      .then((td) => {
        return html`<task-driver-sk id="tdStatus" .data=${td} embedded></task-driver-sk>`;
      });
    // We don't catch failures, since we don't want the promise to resolve (and be used below)
    // unless the task - driver - sk has data.
    this.top = html`${until(
      td,
      html`
        <h3 id="nonTaskDriverHeader">
          <a target="_blank" href="${'TODO WESTON'}/task?id=${task.swarmingTaskId}">
            <span>${task.name}</span><launch-icon-sk></launch-icon-sk>
          </a>
        </h3>
        <div id="nonTaskDriverDetails">
          <table>
            <tr>
              <td>Status:</td>
              <td class=${`task-${(task.status || 'PENDING').toLowerCase()}`}>${task.status}</td>
            </tr>
            <tr>
              <td>Context:</td>
              <td>
                <a href=${this.taskUrl(task)} target="_blank">View on Task Scheduler</a>
              </td>
            </tr>
            <tr>
              <td>Other Tasks Like This:</td>
              <td>
                <a target="_blank" rel="noopener" href=${this.swarmingUrl(task)}>
                  View on Swarming
                </a>
              </td>
            </tr>
          </table>
        </div>
      `
    )}`;

    this.middle = html`
      <h3>Blamelist</h3>
      <table>
        ${task.commits?.map((hash: string) => {
          const commit = commitsByHash.get(hash);
          return html`
            <tr>
              <td>
                <a href="${'TODO repobase needs to be obtained'}">${commit?.shortHash || ''}</a>
              </td>
              <td>${commit?.shortAuthor || ''}</td>
              <td>${commit?.shortSubject || ''}</td>
            </tr>
          `;
        })}
      </table>
    `;

    this.action = { buttonText: 'Re-run Job', handler: () => this.rerunJob(task) };
    this.open();
  }

  rerunJob(task: Task) {
    if (!task || !task.name || !task.revision) {
      errorMessage("Invalid task, can't be re-run");
    }
    // TODO(borenet): This is not correct in some cases. Now that we have a
    // link from Task to Job in the scheduler, we should be able to come up
    // with a better way to "re-open" a failed Job, essentially resetting
    // the attempt counts so that we can retry the failed task(s).
    let job = task.name;
    const uploadPrefix = 'Upload-';
    if (job.indexOf(uploadPrefix) == 0) {
      job = job.substring(uploadPrefix.length);
    }
    fetch(`${taskSchedulerUrl()}/trigger?submit=true&job=${job}@${task.revision}`).catch(
      errorMessage
    );
  }

  displayTaskSpec(taskspec: TaskSpecDetails) {
    this.commentData = {
      comments: taskspec.comments,
      taskId: '',
      commit: '',
      taskSpec: taskspec.name,
      repo: this.repo,
    };
    this.open();
  }

  displayCommit(commit: Commit, comments: Array<Comment>) {
    this.commentData = {
      comments: comments,
      taskId: '',
      commit: commit.hash,
      taskSpec: '',
      repo: this.repo,
    };
    this.open();
  }

  private taskUrl(task: Task) {
    const url = taskSchedulerUrl();
    if (!task || !task.id || !url) {
      return '';
    }
    return `${url}/task/${task.id}`;
  }

  private swarmingUrl(task: Task) {
    return `${swarmingUrl()}/tasklist?f=sk_name%3A${task.name}`;
  }

  get repo(): string {
    return this._repo;
  }
  set repo(value) {
    this._repo = value;
  }
}

define('details-dialog-sk', DetailsDialogSk);
