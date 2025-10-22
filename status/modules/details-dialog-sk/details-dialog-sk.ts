/**
 * @module modules/details-dialog-sk
 * @description <h2><code>details-dialog-sk</code></h2>
 *
 * @property repo {string} - The repo associated with the tasks/taskspecs/commits that will be
 * displayed.
 */
import { html, TemplateResult } from 'lit/html.js';
import { until } from 'lit/directives/until.js';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { $$ } from '../../../infra-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';
import { Commit } from '../util';
import { Task, Comment } from '../rpc';
import { CommentData } from '../comments-sk/comments-sk';
import { logsUrl, revisionUrlTemplate, swarmingUrl, taskSchedulerUrl } from '../settings';

import '../comments-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/content-copy-icon-sk';
import '../../../elements-sk/modules/icons/launch-icon-sk';
import '../../../infra-sk/modules/task-driver-sk';
import { Status } from '../../../infra-sk/modules/json';

// Type defining the text and action of the upper-right button of the dialog.
// For reverts of commits and re-running of tasks.
interface Action {
  buttonText: string;
  handler: () => void;
}

export class DetailsDialogSk extends ElementSk {
  // This template is essentially a title section with optional action button, an optional details
  // section, and a comments-sk section. Task, TaskSpec, and Commits set the appropriate sections
  // before rendering.
  private static template = (el: DetailsDialogSk) => html`
    <div class="dialog" @click=${(e: Event) => e.stopPropagation()}>
      <header>
        <div>${el.titleSection}</div>
        <div class="spacer"></div>
        ${el.actionButton
          ? html`<button class="action" @click=${el.actionButton.handler}>
              ${el.actionButton.buttonText}
            </button>`
          : html``}
        <button class="close" @click=${() => el.close()}>
          <close-icon-sk></close-icon-sk>
        </button>
      </header>
      ${el.detailsSection
        ? [
            el.detailsSection,
            html`
              <br />
              <hr />
            `,
          ]
        : html``}
      <div>
        <comments-sk
          .commentData=${el.commentData}
          .allowAdd=${true}
          .allowDelete=${true}
          .showIgnoreFailure=${el.showCommentsIgnoreFailure}
          .showFlaky=${el.showCommentsFlaky}
          .editRights=${el.canEditComments}></comments-sk>
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
    LoggedIn().then((res: Status) => {
      this.canEditComments = res.email !== '';
      this._render();
    });
    this._render();
  }

  private open() {
    this._render();
    (<HTMLElement>this).style.display = 'block';
    document.addEventListener('keydown', this.keydown);
  }

  close() {
    (<HTMLElement>this).style.display = 'none';
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
      .then(
        (td) => html`<br /><task-driver-sk id="tdStatus" .data=${td} embedded></task-driver-sk>`
      );
    // We don't catch failures, since we don't want the promise to resolve (and be used below)
    // unless the task-driver-sk has data.
    this.titleSection = html`${until(
      td,
      html`
        <h3>
          <span>${task.name}</span
          ><a
            target="_blank"
            rel="noopener noreferrer"
            href="${logsUrl(this.repo, task.swarmingTaskId)}"
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
                  href="${this.swarmingTaskUrl(task.taskExecutor, task.swarmingTaskId)}">
                  View on Swarming
                </a>
              </td>
            </tr>
            <tr>
              <td>Other Tasks Like This:</td>
              <td>
                <a
                  target="_blank"
                  rel="noopener noreferrer"
                  href=${this.swarmingTaskListUrl(task.taskExecutor, task.name)}>
                  View on Swarming
                </a>
              </td>
            </tr>
          </table>
        </div>
      `
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

    this.actionButton = {
      buttonText: 'Re-run Job',
      handler: () => this.rerunJob(task),
    };
    this.open();
  }

  displayTaskSpec(taskExecutor: string, taskspec: string, comments: Comment[]) {
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
      <a
        href="${this.swarmingTaskListUrl(taskExecutor, taskspec)}"
        target="_blank"
        rel="noopener noreferrer">
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
          rel="noopener noreferrer">
          ${commit.hash}
        </a>
        <content-copy-icon-sk
          class="small-icon clickable"
          @click=${() => {
            navigator.clipboard.writeText(commit.hash);
          }}></content-copy-icon-sk>
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
      this.actionButton = {
        buttonText: 'Revert',
        handler: () => this.revertCommit(commit),
      };
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
    if (job.indexOf(uploadPrefix) === 0) {
      job = job.substring(uploadPrefix.length);
    }
    const url = `${taskSchedulerUrl()}/trigger?submit=true&job=${job}@${task.revision}`;
    const win = window.open(url, '_blank') as Window;
    win.focus();
  }

  private revertCommit(commit: Commit) {
    const url =
      commit.patchStorage === 'gerrit'
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

  private swarmingTaskUrl(taskExecutor: string, swarmingTaskId: string) {
    return `${swarmingUrl(taskExecutor)}/task?id=${swarmingTaskId}`;
  }

  private swarmingTaskListUrl(taskExecutor: string, taskSpec: string) {
    return `${swarmingUrl(taskExecutor)}/tasklist?f=sk_name%3A${taskSpec}`;
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
