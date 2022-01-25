/**
 * @module modules/details-dialog-sk
 * @description <h2><code>details-dialog-sk</code></h2>
 *
 * @property repo {string} - The repo associated with the tasks/taskspecs/commits that will be
 * displayed.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, TemplateResult } from 'lit-html';
import { until } from 'lit-html/directives/until.js';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Login } from '../../../infra-sk/modules/login';
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';
import { Commit } from '../util';
import { Task, Comment } from '../rpc';
import { CommentData } from '../comments-sk/comments-sk';
import {
  logsUrl, revisionUrlTemplate, swarmingUrl, taskSchedulerUrl,
} from '../settings';

import '../comments-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/icon/close-icon-sk';
import 'elements-sk/icon/content-copy-icon-sk';
import 'elements-sk/icon/launch-icon-sk';
import '../../../infra-sk/modules/task-driver-sk';

// Type defining the text and action of the upper-right button of the dialog.
// For reverts of commits and re-running of tasks.
interface Action {
  buttonText: string;
  handler: ()=> void;
}

export class DetailsDialogSk extends ElementSk {
  // This template is essentially a title section with optional action button, an optional details
  // section, and a comments-sk section. Task, TaskSpec, and Commits set the appropriate sections
  // before rendering.
  private static template = (el: DetailsDialogSk) => html`
      <div class="dialog" @click=${(e: Event) => e.stopPropagation()}>
        <button class=close @click=${() => el.close()}><close-icon-sk></close-icon-sk></button>
        </br>
        <div class="horizontal">
          <div class="flex titleContainer">${el.titleSection}</div>
          ${
            el.actionButton
              ? html`<button class="action" @click=${el.actionButton.handler}>
                  ${el.actionButton.buttonText}
                </button>`
              : html``
          }
        </div>
        <br />
        <hr />
        ${
          el.detailsSection
            ? [
              el.detailsSection,
              html`
                  <br />
                  <hr />
                `,
            ]
            : html``
        }
        <div>
          <comments-sk
            .commentData=${el.commentData}
            .allowAdd=${true}
            .allowDelete=${true}
            .showIgnoreFailure=${el.showCommentsIgnoreFailure}
            .showFlaky=${el.showCommentsFlaky}
            .editRights=${el.canEditComments}
          ></comments-sk>
        </div>
      </div>
    `;

  private titleSection: TemplateResult = html``;

  private detailsSection: TemplateResult | null = null;

  private actionButton: Action | null = null;

  private showCommentsIgnoreFailure: boolean = false;

  private showCommentsFlaky: boolean = false;

  private canEditComments = false;

  private _repo: string = '';

  private commentData?: CommentData;

  constructor() {
    super(DetailsDialogSk.template);
    this._upgradeProperty('repo');
  }

  connectedCallback() {
    super.connectedCallback();
    Login.then((res: any) => {
      this.canEditComments = res.Email !== '';
      this._render();
    });
    this._render();
  }

  private open() {
    this._render();
    (<HTMLElement> this).style.display = 'block';
    document.addEventListener('keydown', this.keydown);
  }

  close() {
    (<HTMLElement> this).style.display = 'none';
    document.removeEventListener('keydown', this.keydown);
  }

  private keydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      this.close();
    }
  };

  // reset clears the computed templates to ensure we don't have any leftovers if switching from
  // e.g.a task view to a taskspec view.
  private reset() {
    this.actionButton = null;
    this.titleSection = html``;
    this.detailsSection = null;
    this.showCommentsFlaky = false;
    this.showCommentsIgnoreFailure = false;
    const commentInput = $$('comments-sk input-sk', this) as HTMLInputElement;
    if (commentInput) {
      commentInput.value = '';
    }
  }

  displayTask(task: Task, comments: Array<Comment>, commitsByHash: Map<string, Commit>) {
    this.reset();
    this.commentData = {
      comments: comments,
      taskId: task.id,
      commit: '',
      taskSpec: '',
      repo: this.repo,
    };
    const td = fetch(`/json/td/${task.id}`)
      .then(jsonOrThrow)
      .then((td) => html`<br /><task-driver-sk id="tdStatus" .data=${td} embedded></task-driver-sk>`);
    // We don't catch failures, since we don't want the promise to resolve (and be used below)
    // unless the task-driver-sk has data.
    this.titleSection = html`${until(
      td,
      html`
        <h3>
          <span>${task.name}</span
          ><a target="_blank" rel="noopener noreferrer" href="${logsUrl(task.swarmingTaskId)}"
            ><launch-icon-sk></launch-icon-sk>
          </a>
        </h3>
        <div>
          <table>
            <tr>
              <td>Status:</td>
              <td class=${`task-${(task.status || 'PENDING').toLowerCase()}`}>${task.status}</td>
            </tr>
            <tr>
              <td>Context:</td>
              <td>
                <a href=${this.taskUrl(task)} target="_blank" rel="noopener noreferrer">
                  View on Task Scheduler
                </a>
              </td>
            </tr>
            <tr>
              <td>This Task:</td>
              <td>
                <a
                  target="_blank"
                  rel="noopener noreferrer"
                  href="${swarmingUrl()}/task?id=${task.swarmingTaskId}"
                >
                  View on Swarming
                </a>
              </td>
            </tr>
            <tr>
              <td>Other Tasks Like This:</td>
              <td>
                <a target="_blank" rel="noopener noreferrer" href=${this.swarmingUrl(task.name)}>
                  View on Swarming
                </a>
              </td>
            </tr>
          </table>
        </div>
      `,
    )}`;

    this.detailsSection = html`
      <h3>Blamelist</h3>
      <table class="blamelist">
        ${task.commits?.map((hash: string) => {
    const commit = commitsByHash.get(hash);
    return html`
            <tr>
              <td>
                <a href="${revisionUrlTemplate(this.repo)}${hash}">${commit?.shortHash || ''}</a>
              </td>
              <td>${commit?.shortAuthor || ''}</td>
              <td>${commit?.shortSubject || ''}</td>
            </tr>
          `;
  })}
      </table>
    `;

    this.actionButton = { buttonText: 'Re-run Job', handler: () => this.rerunJob(task) };
    this.open();
  }

  displayTaskSpec(taskspec: string, comments: Comment[]) {
    this.reset();
    this.showCommentsFlaky = true;
    this.showCommentsIgnoreFailure = true;
    this.commentData = {
      comments: comments,
      taskId: '',
      commit: '',
      taskSpec: taskspec,
      repo: this.repo,
    };
    this.titleSection = html` <h3>
      <a href="${this.swarmingUrl(taskspec)}" target="_blank" rel="noopener noreferrer">
        ${taskspec}
      </a>
    </h3>`;
    this.open();
  }

  displayCommit(commit: Commit, comments: Array<Comment>) {
    this.reset();
    this.showCommentsIgnoreFailure = true;
    this.commentData = {
      comments: comments,
      taskId: '',
      commit: commit.hash,
      taskSpec: '',
      repo: this.repo,
    };
    this.titleSection = html`
      <p>
        <a
          href="${revisionUrlTemplate(this.repo)}${commit.hash}"
          target="_blank"
          rel="noopener noreferrer"
        >
          ${commit.hash}
        </a>
        <content-copy-icon-sk
          class="small-icon clickable"
          @click=${() => {
    navigator.clipboard.writeText(commit.hash);
  }}
        ></content-copy-icon-sk>
        <br />
        ${commit.author}
        <br />
        <span title="${commit.timestamp!}">${this.humanDate(commit.timestamp!)}</span>
      </p>
    `;

    this.detailsSection = html`
      <h3>${escapeAndLinkify(commit.subject)}</h3>
      <p>${escapeAndLinkify(commit.body)}</p>
    `;
    if (commit.issue) {
      this.actionButton = { buttonText: 'Revert', handler: () => this.revertCommit(commit) };
    }
    this.open();
  }

  private rerunJob(task: Task) {
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
    const url = `${taskSchedulerUrl()}/trigger?submit=true&job=${job}@${task.revision}`;
    const win = window.open(url, '_blank') as Window;
    win.focus();
  }

  private revertCommit(commit: Commit) {
    const url = commit.patchStorage === 'gerrit'
      ? `https://skia-review.googlesource.com/c/${commit.issue}/?revert`
      : `https://codereview.chromium.org/${commit.issue}/revert`;
    const win = window.open(url, '_blank') as Window;
    win.focus();
  }

  private taskUrl(task: Task) {
    const url = taskSchedulerUrl();
    if (!task || !task.id || !url) {
      return '';
    }
    return `${url}/task/${task.id}`;
  }

  private swarmingUrl(taskSpec: string) {
    return `${swarmingUrl()}/tasklist?f=sk_name%3A${taskSpec}`;
  }

  private humanDate(timestamp: string) {
    const date = new Date(timestamp);
    const str = date.toString();
    return `${date.toLocaleString()} ${str.substring(str.indexOf('('))}`;
  }

  get repo(): string {
    return this._repo;
  }

  set repo(value) {
    this._repo = value;
  }
}

define('details-dialog-sk', DetailsDialogSk);
