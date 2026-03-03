/**
 * @module modules/pinpoint-try-job-dialog-sk
 * @description <h2><code>pinpoint-try-job-dialog-sk</code></h2>
 *
 * pinpoint-try-job-dialog-sk allows a user to trigger a Pinpoint A/B Try job.
 * While try jobs support more use cases, the use case for this dialog in perf
 * is for requesting additional traces on benchmark runs. We should avoid
 * building on top of this job dialog as we migrate towards the newer Pinpoint
 * frontend.
 *
 * pinpoint-try-job-dialog-sk is based off of bisect-dialog-sk.
 *
 * Request parameters:
 *   test path: the full string of trace name from chart-tooltip
 *   baseCommit: base commit
 *   endCommit: end commit
 *   story: the last sub_test name from test path
 * Once a validated user submits this dialog, there'll be an attempt to post an A/B job
 * request. If successful, the user will see a Pinpoint job created.
 */

import '../../../elements-sk/modules/select-sk';
import { html, LitElement } from 'lit';
import { property, state, query, customElement } from 'lit/decorators.js';
import { live } from 'lit/directives/live.js';
import { Task, TaskStatus } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { TryJobCreateRequest, CreatePinpointResponse } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';

import '@material/web/icon/icon.js';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../infra-sk/modules/alogin-sk/alogin-sk';

export interface TryJobPreloadParams {
  testPath?: string | '';
  baseCommit?: string | '';
  endCommit?: string | '';
  story?: string | '';
}

@customElement('pinpoint-try-job-dialog-sk')
export class PinpointTryJobDialogSk extends LitElement {
  @state()
  private jobUrl: string = '';

  @property({ type: String })
  testPath: string = '';

  @property({ type: String })
  baseCommit: string = '';

  @property({ type: String })
  endCommit: string = '';

  @state()
  private traceArgs: string = 'toplevel,toplevel.flow,disabled-by-default-toplevel.flow';

  @state()
  private traceHelpLink: string =
    'https://source.chromium.org/chromium/chromium/src/+/main:base/trace_event/trace_config.h;' +
    'l=167;drc=04e98bf9e13012fda48001096174a4500ff27866;bpv=1';

  @property({ type: String })
  story: string = '';

  @state()
  private user: string = '';

  @query('#pinpoint-try-job-dialog')
  private dialog!: HTMLDialogElement;

  @query('#pinpoint-try-job-form')
  private form!: HTMLFormElement;

  @state()
  private _request: TryJobCreateRequest | null = null;

  private _tryJobTask = new Task(this, {
    task: async (
      [req]: [TryJobCreateRequest | null | undefined],
      { signal }: { signal: AbortSignal }
    ) => {
      if (!req) {
        return null;
      }
      const response = await fetch('/_/try/', {
        method: 'POST',
        body: JSON.stringify(req),
        headers: {
          'Content-Type': 'application/json',
        },
        signal,
      });
      const json = await jsonOrThrow(response);
      return json as CreatePinpointResponse;
    },
    args: (): [TryJobCreateRequest | null] => [this._request],
  });

  connectedCallback() {
    super.connectedCallback();
    LoggedIn()
      .then((status: LoginStatus) => {
        this.user = status.email;
      })
      .catch(errorMessage);
  }

  createRenderRoot() {
    return this;
  }

  setTryJobInputParams(params: TryJobPreloadParams): void {
    this.testPath = params.testPath || '';
    this.baseCommit = params.baseCommit || '';
    this.endCommit = params.endCommit || '';
    this.story = params.story || '';
    this.jobUrl = '';
    // Reset task state
    this._request = null;
  }

  async open(): Promise<void> {
    await this.updateComplete;
    this.dialog.showModal();
  }

  private closeDialog(): void {
    this.dialog.close();
  }

  private onDialogClick(event: MouseEvent) {
    const rect = this.dialog.getBoundingClientRect();
    if (
      event.clientX < rect.left ||
      event.clientX > rect.right ||
      event.clientY < rect.top ||
      event.clientY > rect.bottom
    ) {
      this.closeDialog();
    }
  }

  private onSubmit(e: Event) {
    e.preventDefault();
    const req: TryJobCreateRequest = {
      name: 'Tracing Debug',
      base_git_hash: this.baseCommit,
      end_git_hash: this.endCommit,
      base_patch: '',
      experiment_patch: '',
      configuration: this.testPath.split('/')[1] || '',
      benchmark: this.testPath.split('/')[2] || '',
      story: this.story,
      repository: 'chromium',
      bug_id: '',
      user: this.user,
      extra_test_args: `["--extra-chrome-categories","${this.traceArgs}"]`,
    };
    req.name = `${req.name} on ${req.configuration}/${req.benchmark}/${req.story}`;

    this._request = req;
  }

  render() {
    return html`
      <dialog id='pinpoint-try-job-dialog' @click=${this.onDialogClick}>
        <h2>Debug Traces</h2>
        <button id="pinpoint-try-job-dialog-close" @click=${this.closeDialog}>
          <close-icon-sk></close-icon-sk>
        </button>
        <form id="pinpoint-try-job-form" @submit=${this.onSubmit}>
          <h3>Base Commit</h3>
          <input
            id="base-commit"
            type="text"
            .value=${live(this.baseCommit)}
            @input=${(e: InputEvent) => (this.baseCommit = (e.target as HTMLInputElement).value)}
          ></input>
          <h3>Experiment Commit</h3>
          <input
            id="exp-commit"
            type="text"
            .value=${live(this.endCommit)}
            @input=${(e: InputEvent) => (this.endCommit = (e.target as HTMLInputElement).value)}
          ></input>
          <h3>Tracing arguments</h3>
          <p>Learn more about
            <a href="${this.traceHelpLink}" target="_blank">
              filter strings<md-icon id="icon">open_in_new</md-icon>
            </a>
          <p>
          <input
            id="trace-args"
            type="text"
            .value=${live(this.traceArgs)}
            @input=${(e: InputEvent) => (this.traceArgs = (e.target as HTMLInputElement).value)}
          ></input>
          <div class=footer>
            <button
              id="pinpoint-try-job-dialog-submit"
              type="Submit"
              .disabled=${this._tryJobTask.status === TaskStatus.PENDING}
            >Generate</button>
            <spinner-sk
              id="dialog-spinner"
              .active=${this._tryJobTask.status === TaskStatus.PENDING}
            ></spinner-sk>
            ${this._renderTaskStatus()}
          </div>
        </form>
      </dialog>
    `;
  }

  private _renderTaskStatus() {
    return this._tryJobTask.render({
      pending: () => html``,
      complete: (json) => {
        console.log('Task completed with:', json);
        if (!json) return html``;
        const url = (json as CreatePinpointResponse).jobUrl;
        return html`<a href="${url}" target="_blank">
          Pinpoint Job Created<md-icon id="icon">open_in_new</md-icon>
        </a>`;
      },
      error: (e: any) => {
        console.error('Task error:', e);
        errorMessage(e);
        return html``;
      },
      initial: () => html``,
    });
  }
}
