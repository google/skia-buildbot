/**
 * @module modules/bisect-dialog-sk
 * @description <h2><code>bisect-dialog-sk</code></h2>
 *
 * Dialog to show when user wants to bisect with the corresponding plot data from chart-tooltip.
 * The bisect logic is only specific to Chrome.
 *
 * Takes the following inputs:
 * Request parameters:
 *   test path: the full string of trace name from chart-tooltip
 *   bug_id: Bug ID number, as a string (when submitting the form).
 *   start_revision: start revision time from input
 *   end_revision: end revision time from input
 *   story: the last sub_test name from test path
 * Once a validated user submits this dialog, there'll be an attempt to post a bisect
 * request. If succesful, user will see a pinpoint job URL. If unsuccesful, an
 * error message popup will appear.
 */

import { html, LitElement } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { live } from 'lit/directives/live.js';
import { Task, TaskStatus } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { BisectJobCreateRequest } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';

import '@material/web/icon/icon.js';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../infra-sk/modules/alogin-sk/alogin-sk';

const STATISTIC_VALUES = ['avg', 'count', 'max', 'min', 'std', 'sum'];

export interface BisectPreloadParams {
  testPath?: string | '';
  startCommit?: string | '';
  endCommit?: string | '';
  bugId?: string | '';
  story?: string | '';
  anomalyId: string | '';
}

@customElement('bisect-dialog-sk')
export class BisectDialogSk extends LitElement {
  @property({ type: String })
  bugId: string = '';

  @property({ type: String })
  startCommit: string = '';

  @property({ type: String })
  endCommit: string = '';

  @property({ type: String })
  story: string = '';

  @state()
  user: string = '';

  @property({ type: String })
  testPath: string = '';

  @property({ type: String })
  anomalyId: string = '';

  @query('#bisect-dialog')
  private _dialog!: HTMLDialogElement;

  @state()
  private jobId: string = '';

  @state()
  private patch: string = '';

  @state()
  private _opened: boolean = false;

  @state()
  private jobUrl: string = '';

  createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    LoggedIn()
      .then((status: LoginStatus) => {
        this.user = status.email;
      })
      .catch(errorMessage);
  }

  setBisectInputParams(preloadBisectInputs: BisectPreloadParams): void {
    this.testPath = preloadBisectInputs.testPath!;
    this.startCommit = preloadBisectInputs.startCommit!;
    this.endCommit = preloadBisectInputs.endCommit!;
    this.bugId = preloadBisectInputs.bugId!;
    this.story = preloadBisectInputs.story!;
    this.anomalyId = preloadBisectInputs.anomalyId!;
    this.patch = '';
  }

  open(): void {
    this._opened = true;
    this.patch = '';
    this.jobUrl = '';
    this._dialog?.showModal();
  }

  private closeBisectDialog(): void {
    this._opened = false;
    this._dialog?.close();
  }

  private _postBisectTask = new Task(this, {
    task: async ([req]: readonly [BisectJobCreateRequest], { signal }: { signal: AbortSignal }) => {
      try {
        const response = await fetch('/_/bisect/create', {
          method: 'POST',
          signal,
          body: JSON.stringify(req),
          headers: {
            'Content-Type': 'application/json',
          },
        });
        const json = await jsonOrThrow(response);
        this.jobId = json.jobId;
        this.jobUrl = json.jobUrl;
      } catch (msg: any) {
        if (msg.name === 'AbortError') {
          throw msg;
        }
        errorMessage(msg);
        throw msg;
      }
    },
    autoRun: false,
  });

  get opened() {
    return this._dialog?.open;
  }

  postBisect(): void {
    if (!this.opened) {
      return;
    }

    const parameters = this.testPath.split('/');

    const test = parameters.at(3) || '';

    const parts: string[] = test.split(':');
    // Pop up the last element from the array if exists
    const tail = parts.pop();

    let chart: string = test;
    let statistic: string = '';
    if (tail !== undefined) {
      chart = STATISTIC_VALUES.includes(tail) ? parts.join('_') : test;
      statistic = STATISTIC_VALUES.includes(tail) ? tail : '';
    }

    // Replace ':' with '_' for reduce errors when querying test paths in the legacy table
    // TODO(jiaxindong) b/431213645 follow up with the team about data backfill
    this.story = (parameters.pop() || '').replace(/:/g, '_');

    const req: BisectJobCreateRequest = {
      comparison_mode: 'performance',
      start_git_hash: this.startCommit,
      end_git_hash: this.endCommit,
      configuration: this.testPath.split('/')[1],
      benchmark: this.testPath.split('/')[2],
      story: this.story,
      chart: chart,
      statistic: statistic,
      comparison_magnitude: '',
      pin: this.patch,
      project: 'chromium',
      bug_id: this.bugId,
      user: this.user,
      alert_ids: '[' + this.anomalyId + ']',
      test_path: this.testPath,
    };

    const validations = [
      { value: req.test_path, message: 'Test path is missing in the request.' },
      { value: req.start_git_hash, message: 'Start commit is required.' },
      { value: req.end_git_hash, message: 'End commit is required.' },
      { value: req.configuration, message: 'Configuration is missing in the request.' },
      { value: req.benchmark, message: 'Benchmark is missing in the request.' },
      { value: req.story, message: 'Story is missing in the request.' },
      { value: req.chart, message: 'Chart is missing in the request.' },
      { value: req.user, message: 'User is not logged in.' },
    ];

    for (const rule of validations) {
      if (!rule.value) {
        errorMessage(rule.message);
        return;
      }
    }

    this._postBisectTask.run([req]);
  }

  /** Clear Bisect Dialog fields */
  reset(): void {
    this.startCommit = '';
    this.endCommit = '';
    this.story = '';
    this.testPath = '';
    this.anomalyId = '';
    this.patch = '';
    this.jobUrl = '';
  }

  render() {
    const isLoading = this._postBisectTask.status === TaskStatus.PENDING;
    return html`
      <dialog id="bisect-dialog">
        <h2>Bisection</h2>
        <button id="bisectCloseIcon" @click=${this.closeBisectDialog}>
          <close-icon-sk></close-icon-sk>
        </button>
        <form id="bisect-form" @submit=${(e: Event) => e.preventDefault()}>
          <h3>Test Path</h3>
          <input
            id="testpath"
            type="text"
            .value=${live(this.testPath)}
            @input=${(e: Event) => {
              this.testPath = (e.target as HTMLInputElement).value;
            }} />
          <h3>Bug ID</h3>
          <input
            id="bug-id"
            type="text"
            .value=${live(this.bugId)}
            @input=${(e: Event) => {
              this.bugId = (e.target as HTMLInputElement).value;
            }} />
          <h3>Start Commit</h3>
          <input
            id="start-commit"
            type="text"
            .value=${live(this.startCommit)}
            @input=${(e: Event) => {
              this.startCommit = (e.target as HTMLInputElement).value;
            }} />
          <h3>End Commit</h3>
          <input
            id="end-commit"
            type="text"
            .value=${live(this.endCommit)}
            @input=${(e: Event) => {
              this.endCommit = (e.target as HTMLInputElement).value;
            }} />
          <h3>Story</h3>
          <input
            id="story"
            type="text"
            .value=${live(this.story)}
            @input=${(e: Event) => {
              this.story = (e.target as HTMLInputElement).value;
            }} />
          <h3>Patch to apply to the entire job (optional)</h3>
          <input
            id="patch"
            type="text"
            .value=${live(this.patch)}
            @input=${(e: Event) => {
              this.patch = (e.target as HTMLInputElement).value;
            }} />
          <div class="footer">
            <button
              id="submit-button"
              type="submit"
              @click=${this.postBisect}
              ?disabled=${isLoading}>
              Bisect
            </button>
            <spinner-sk id="dialog-spinner" ?active=${isLoading}></spinner-sk>
            ${this.jobUrl
              ? html`<a id="pinpoint-job-url" href="${this.jobUrl}" target="_blank">
                  Pinpoint Job Created<md-icon id="icon">open_in_new</md-icon>
                </a>`
              : ''}
          </div>
        </form>
      </dialog>
    `;
  }
}
