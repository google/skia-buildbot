import { html, css, LitElement } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import { Task, TaskStatus } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { BisectJobCreateRequest, TryJobCreateRequest, CreatePinpointResponse } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../window/window';

import { MdDialog } from '@material/web/dialog/dialog.js';
import '@material/web/dialog/dialog.js';
import '@material/web/tabs/tabs.js';
import '@material/web/tabs/primary-tab.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/button/filled-button.js';
import '@material/web/button/text-button.js';
import '@material/web/progress/circular-progress.js';
import '@material/web/icon/icon.js';
import '@material/web/checkbox/checkbox.js';
import { parseTestPath } from '../common/test-path';

export type PinpointMode = 'bisect' | 'try';

export interface PinpointPreloadParams {
  testPath?: string;
  startCommit?: string;
  baseCommit?: string; // alias for startCommit
  endCommit?: string;
  bugId?: string;
  story?: string;
  anomalyId?: string;
  configuration?: string;
  benchmark?: string;
}

@customElement('pinpoint-dialog-sk')
export class PinpointDialogSk extends LitElement {
  @state() mode: PinpointMode = 'bisect';

  @state() useNewPinpoint = false;

  @state() testPath = '';

  @state() bugId = '';

  @state() startCommit = '';

  @state() endCommit = '';

  @state() story = '';

  @state() patch = '';

  @state() configuration = '';

  @state() benchmark = '';

  // Default TraceEvent categories used by Chromium performance profiling to record top-level task execution and task flow events.
  @state() traceArgs = 'toplevel,toplevel.flow,disabled-by-default-toplevel.flow';

  @state() anomalyId = '';

  @state() user = '';

  @query('#dialog') private _dialog!: MdDialog | null;

  @state() private _bisectRequest: BisectJobCreateRequest | null = null;

  @state() private _tryRequest: TryJobCreateRequest | null = null;

  private _postBisectTask = new Task(this, {
    task: async ([req]: [BisectJobCreateRequest | null], { signal }) => {
      if (!req) return null;
      const resp = await fetch('/_/bisect/create', {
        method: 'POST',
        signal,
        body: JSON.stringify(req),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      return (await jsonOrThrow(resp)) as CreatePinpointResponse;
    },
    args: (): [BisectJobCreateRequest | null] => [this._bisectRequest],
  });

  private _postTryTask = new Task(this, {
    task: async ([req]: [TryJobCreateRequest | null], { signal }) => {
      if (!req) return null;
      const resp = await fetch('/_/try/', {
        method: 'POST',
        signal,
        body: JSON.stringify(req),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      return (await jsonOrThrow(resp)) as CreatePinpointResponse;
    },
    args: (): [TryJobCreateRequest | null] => [this._tryRequest],
  });

  static styles = css`
    :host {
      display: block;
    }

    md-dialog {
      --md-dialog-container-color: var(--surface);
      --md-dialog-headline-color: var(--on-surface);
      --md-dialog-supporting-text-color: var(--secondary);

      min-width: 480px;
      max-width: 560px;
    }

    md-tabs {
      margin-bottom: 16px;

      --md-tabs-primary-tab-label-text-color: var(--secondary);
      --md-tabs-primary-tab-active-label-text-color: var(--primary);
      --md-tabs-primary-tab-active-indicator-color: var(--primary);
    }

    .form-container {
      display: flex;
      flex-direction: column;
      gap: 16px;
      padding: 4px 0;
    }

    .form-row {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }

    md-outlined-text-field {
      width: 100%;

      --md-outlined-text-field-container-shape: 4px;
      --md-outlined-text-field-outline-color: var(--outline);
      --md-outlined-text-field-focus-outline-color: var(--primary);
      --md-outlined-text-field-input-text-color: var(--on-surface);
      --md-outlined-text-field-label-text-color: var(--secondary);
      --md-outlined-text-field-focus-label-text-color: var(--primary);
    }

    .help-text {
      font-size: 12px;
      color: var(--secondary);
      margin-top: -10px;
      padding-left: 4px;
    }

    .help-text a {
      color: var(--primary);
      text-decoration: none;
    }

    .help-text a:hover {
      text-decoration: underline;
    }

    .success-banner {
      background-color: color-mix(in srgb, var(--primary) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--primary) 20%, transparent);
      border-radius: 8px;
      padding: 12px 16px;
      display: flex;
      flex-direction: column;
      gap: 4px;
      margin-top: 16px;
    }

    .success-title {
      font-size: 13px;
      font-weight: 600;
      color: var(--on-surface);
    }

    .success-link {
      font-size: 13px;
      color: var(--primary);
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-family: monospace;
    }

    .success-link:hover {
      text-decoration: underline;
    }

    .dialog-footer {
      display: flex;
      justify-content: flex-end;
      gap: 12px;
      margin-top: 16px;
    }

    md-filled-button {
      --md-filled-button-container-color: var(--primary);
      --md-filled-button-label-text-color: var(--on-primary);
      --md-filled-button-container-shape: 20px;
    }

    md-text-button {
      --md-text-button-label-text-color: var(--primary);
      --md-text-button-container-shape: 20px;
    }

    .loading-indicator {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: 13px;
      color: var(--secondary);
      margin-right: auto;
    }

    md-circular-progress {
      --md-circular-progress-active-indicator-color: var(--primary);
    }

    .error-banner {
      color: var(--error);
      background-color: color-mix(in srgb, var(--error) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--error) 20%, transparent);
      border-radius: 8px;
      padding: 12px;
      margin-top: 16px;
      font-size: 13px;
    }

    .checkbox-container {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
      color: var(--secondary);
      cursor: pointer;
      margin-top: 16px;
      margin-bottom: 8px;
      user-select: none;
    }

    .open-in-new-icon {
      font-size: 14px;
      margin-left: 2px;
    }
  `;

  private readonly stopProp = (e: Event) => e.stopPropagation();

  connectedCallback() {
    super.connectedCallback();

    this.addEventListener('click', this.stopProp);
    this.addEventListener('pointerdown', this.stopProp);
    this.addEventListener('mousedown', this.stopProp);

    void LoggedIn()
      .then((status: LoginStatus) => {
        this.user = status.email;
      })
      .catch(errorMessage);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.removeEventListener('click', this.stopProp);
    this.removeEventListener('pointerdown', this.stopProp);
    this.removeEventListener('mousedown', this.stopProp);
  }

  async open(mode: PinpointMode, params: PinpointPreloadParams): Promise<void> {
    this.mode = mode;
    this.testPath = params.testPath || '';
    this.bugId = params.bugId || '';
    this.startCommit = params.startCommit || params.baseCommit || '';
    this.endCommit = params.endCommit || '';
    this.story = params.story || '';
    this.anomalyId = params.anomalyId || '';
    this.configuration = params.configuration || '';
    this.benchmark = params.benchmark || '';
    this.patch = '';

    this.useNewPinpoint = false;
    this._bisectRequest = null;
    this._tryRequest = null;
    await this.updateComplete;
    await customElements.whenDefined('md-dialog');
    this._dialog?.show();
  }

  close(): void {
    this._dialog?.close();
  }

  private _validateBisect(req: BisectJobCreateRequest): boolean {
    const rules = [
      { val: req.test_path, err: 'Test path is missing in the request.' },
      { val: req.start_git_hash, err: 'Start commit is required.' },
      { val: req.end_git_hash, err: 'End commit is required.' },
      { val: req.configuration, err: 'Configuration is missing in the request.' },
      { val: req.benchmark, err: 'Benchmark is missing in the request.' },
      { val: req.story, err: 'Story is missing in the request.' },
      { val: req.chart, err: 'Chart is missing in the request.' },
      { val: req.user, err: 'User is not logged in.' },
    ];
    for (const r of rules) {
      if (!r.val) {
        void errorMessage(r.err);
        return false;
      }
    }
    return true;
  }

  private _getProjectAndRepo(): { project: string; repo: string } {
    let project = 'chromium';
    let repo = 'chromium';
    if (window.perf?.git_repo_url?.includes('skia')) {
      project = 'skia';
      repo = 'skia';
    }
    return { project, repo };
  }

  submitBisect(): void {
    const parsedPath = parseTestPath(this.testPath);

    const { project } = this._getProjectAndRepo();

    const req: BisectJobCreateRequest = {
      comparison_mode: 'performance',
      start_git_hash: this.startCommit,
      end_git_hash: this.endCommit,
      configuration: parsedPath.configuration,
      benchmark: parsedPath.benchmark,
      story: this.story || parsedPath.story,
      chart: parsedPath.chart,
      statistic: parsedPath.statistic,
      comparison_magnitude: '',
      pin: this.patch,
      project: project,
      bug_id: this.bugId,
      user: this.user,
      alert_ids: '[' + this.anomalyId + ']',
      test_path: this.testPath,
      extra_test_args: this.traceArgs
        ? JSON.stringify(['--extra-chrome-categories', this.traceArgs])
        : '',
      use_new_pinpoint: this.useNewPinpoint,
    };

    if (this._validateBisect(req)) {
      this._bisectRequest = req;
    }
  }

  submitTry(): void {
    const parsedPath = parseTestPath(this.testPath);
    const config =
      this.configuration || parsedPath.configuration || this.testPath.split('/')[1] || '';
    const bench = this.benchmark || parsedPath.benchmark || this.testPath.split('/')[2] || '';
    const story = this.story || parsedPath.story || '';
    const { project, repo } = this._getProjectAndRepo();

    const req: TryJobCreateRequest = {
      name: `Tracing Debug on ${config}/${bench}/${story}`,
      base_git_hash: this.startCommit,
      end_git_hash: this.endCommit,
      base_patch: '',
      experiment_patch: '',
      configuration: config,
      benchmark: bench,
      story: story,
      repository: repo,
      bug_id: '',
      user: this.user,
      project: project,
      extra_test_args: JSON.stringify(['--extra-chrome-categories', this.traceArgs]),
      use_new_pinpoint: this.useNewPinpoint,
    };

    if (!req.base_git_hash || !req.end_git_hash) {
      void errorMessage('Base and Experiment commits are required.');
      return;
    }
    if (!req.story) {
      void errorMessage('Story is required for try jobs.');
      return;
    }
    this._tryRequest = req;
  }

  private _showNewPinpointBackendCheckbox(): boolean {
    return !!(window as any).perf?.show_new_pinpoint_backend_checkbox;
  }

  render() {
    const isPending =
      this._postBisectTask.status === TaskStatus.PENDING ||
      this._postTryTask.status === TaskStatus.PENDING;

    return html`
      <md-dialog id="dialog" @close=${this.close}>
        <span slot="headline">
          ${this.mode === 'bisect' ? 'Performance Bisection' : 'Debug Traces'}
        </span>

        <div slot="content">
          <md-tabs>
            <md-primary-tab
              ?active=${this.mode === 'bisect'}
              @click=${() => {
                this.mode = 'bisect';
              }}>
              <md-icon slot="icon">insights</md-icon>
              Bisect
            </md-primary-tab>
            <md-primary-tab
              ?active=${this.mode === 'try'}
              @click=${() => {
                this.mode = 'try';
              }}>
              <md-icon slot="icon">history_toggle_off</md-icon>
              Debug
            </md-primary-tab>
          </md-tabs>

          ${this.mode === 'bisect' ? this._renderBisectForm() : this._renderTryForm()}
          ${this._showNewPinpointBackendCheckbox()
            ? html`
                <label class="checkbox-container">
                  <md-checkbox
                    id="use-new-pinpoint"
                    ?checked=${this.useNewPinpoint}
                    @change=${(e: any) => {
                      this.useNewPinpoint = e.target.checked;
                    }}>
                  </md-checkbox>
                  Use new Pinpoint Service (Temporal workflows)
                </label>
              `
            : ''}
          ${this._renderStatus()}
        </div>

        <div slot="actions">
          ${isPending
            ? html`
                <div class="loading-indicator">
                  <md-circular-progress indeterminate></md-circular-progress>
                  Submitting to Pinpoint...
                </div>
              `
            : ''}
          <md-text-button @click=${this.close}>Cancel</md-text-button>
          <md-filled-button
            ?disabled=${isPending || !this.user}
            @click=${this.mode === 'bisect' ? this.submitBisect : this.submitTry}>
            ${this.mode === 'bisect' ? 'Start Bisect' : 'Generate'}
          </md-filled-button>
        </div>
      </md-dialog>
    `;
  }

  private _renderBisectForm() {
    return html`
      <div class="form-container">
        <md-outlined-text-field
          label="Test Path *"
          .value=${this.testPath}
          @input=${(e: any) => {
            this.testPath = e.target.value;
          }}>
        </md-outlined-text-field>

        <div class="form-row">
          <md-outlined-text-field
            label="Bug ID"
            .value=${this.bugId}
            @input=${(e: any) => {
              this.bugId = e.target.value;
            }}>
          </md-outlined-text-field>
          <md-outlined-text-field
            label="Story *"
            .value=${this.story}
            @input=${(e: any) => {
              this.story = e.target.value;
            }}>
          </md-outlined-text-field>
        </div>

        <div class="form-row">
          <md-outlined-text-field
            label="Start Commit *"
            .value=${this.startCommit}
            @input=${(e: any) => {
              this.startCommit = e.target.value;
            }}>
          </md-outlined-text-field>
          <md-outlined-text-field
            label="End Commit *"
            .value=${this.endCommit}
            @input=${(e: any) => {
              this.endCommit = e.target.value;
            }}>
          </md-outlined-text-field>
        </div>

        <md-outlined-text-field
          label="Patch (Optional)"
          placeholder="cl/12345 or git hash"
          .value=${this.patch}
          @input=${(e: any) => {
            this.patch = e.target.value;
          }}>
        </md-outlined-text-field>
      </div>
    `;
  }

  private _renderTryForm() {
    return html`
      <div class="form-container">
        <div class="form-row">
          <md-outlined-text-field
            label="Base Commit *"
            .value=${this.startCommit}
            @input=${(e: any) => {
              this.startCommit = e.target.value;
            }}>
          </md-outlined-text-field>
          <md-outlined-text-field
            label="Experiment Commit *"
            .value=${this.endCommit}
            @input=${(e: any) => {
              this.endCommit = e.target.value;
            }}>
          </md-outlined-text-field>
        </div>

        <md-outlined-text-field
          label="Tracing Arguments *"
          .value=${this.traceArgs}
          @input=${(e: any) => {
            this.traceArgs = e.target.value;
          }}>
        </md-outlined-text-field>
        <div class="help-text">
          Learn more about
          <a
            href="https://source.chromium.org/chromium/chromium/src/+/main:base/trace_event/trace_config.h;l=167"
            target="_blank">
            filter strings
          </a>
        </div>
      </div>
    `;
  }

  private _renderStatus() {
    const activeTask = this.mode === 'bisect' ? this._postBisectTask : this._postTryTask;
    return activeTask.render({
      complete: (json) => {
        if (!json) return html``;
        const url = json.jobUrl || (json as any).job_url;
        if (!url) return html``;
        return html`
          <div class="success-banner">
            <span class="success-title">Pinpoint Job Successfully Created</span>
            <a class="success-link" href="${url}" target="_blank">
              View Pinpoint Job
              <md-icon class="open-in-new-icon">open_in_new</md-icon>
            </a>
          </div>
        `;
      },
      error: (e: any) => {
        return html`
          <div class="error-banner"><strong>Pinpoint API Error:</strong> ${e.message || e}</div>
        `;
      },
    });
  }
}
