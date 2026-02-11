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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { CreateLegacyTryRequest, CreatePinpointResponse } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

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

export class PinpointTryJobDialogSk extends ElementSk {
  private jobUrl: string = '';

  private testPath: string = '';

  private baseCommit: string = '';

  private endCommit: string = '';

  private traceArgs: string = 'toplevel,toplevel.flow,disabled-by-default-toplevel.flow';

  // This link is from legacy perf. It links to some other trace arguments you can use
  // but does not explain the feature too well.
  private traceHelpLink: string =
    'https://source.chromium.org/chromium/chromium/src/+/main:base/trace_event/trace_config.h;' +
    'l=167;drc=04e98bf9e13012fda48001096174a4500ff27866;bpv=1';

  private story: string = '';

  private user: string = '';

  private _dialog: HTMLDialogElement | null = null;

  private _spinner: SpinnerSk | null = null;

  private _form: HTMLFormElement | null = null;

  private submitButton: HTMLButtonElement | null = null;

  private static template = (ele: PinpointTryJobDialogSk) => html`
    <dialog id='pinpoint-try-job-dialog'>
      <h2>Debug Traces</h2>
      <button id="pinpoint-try-job-dialog-close" @click=${ele.closeDialog}>
        <close-icon-sk></close-icon-sk>
      </button>
      <form id="pinpoint-try-job-form">
      <h3>Base Commit</h3>
      <input id="base-commit" type="text" value=${ele.baseCommit}></input>
      <h3>Experiment Commit</h3>
      <input id="exp-commit" type="text" value=${ele.endCommit}></input>
      <h3>Tracing arguments</h3>
      <p>Learn more about
        <a href="${ele.traceHelpLink}" target="_blank">
          filter strings<md-icon id="icon">open_in_new</md-icon>
        </a>
      <p>
      <input id="trace-args" type="text" value=${ele.traceArgs}></input>
      <div class=footer>
        <button id="pinpoint-try-job-dialog-submit" type="Submit">Generate</button>
        <spinner-sk id="dialog-spinner"></spinner-sk>
        ${
          ele.jobUrl
            ? html`<a href="${ele.jobUrl}" target="_blank">
                Pinpoint Job Created<md-icon id="icon">open_in_new</md-icon>
              </a>`
            : ''
        }
      </div>
      </form>
    </dialog>
    `;

  constructor() {
    super(PinpointTryJobDialogSk.template);
  }

  connectedCallback() {
    super.connectedCallback();

    upgradeProperty(this, 'preloadInputParameters');
    upgradeProperty(this, 'testPath');
    upgradeProperty(this, 'startCommit');
    upgradeProperty(this, 'endCommit');
    upgradeProperty(this, 'story');

    this._render();

    this._dialog = this.querySelector('#pinpoint-try-job-dialog');
    this._spinner = this.querySelector('#dialog-spinner');
    this.submitButton = this.querySelector('#pinpoint-try-job-dialog-submit');
    this._form = this.querySelector('#pinpoint-try-job-form');
    this._form!.addEventListener('submit', (e) => {
      e.preventDefault();
      this.postTryJob();
    });

    LoggedIn()
      .then((status: LoginStatus) => {
        this.user = status.email;
      })
      .catch(errorMessage);

    // close the dialog if mouse click outside of the dialog
    this._dialog!.addEventListener('click', (event) => {
      const rect = this._dialog!.getBoundingClientRect();
      if (
        event.clientX < rect.left ||
        event.clientX > rect.right ||
        event.clientY < rect.top ||
        event.clientY > rect.bottom
      ) {
        this.closeDialog();
      }
    });
  }

  setTryJobInputParams(params: TryJobPreloadParams): void {
    this.testPath = params.testPath!;
    this.baseCommit = params.baseCommit!;
    this.endCommit = params.endCommit!;
    this.story = params.story!;
    this.jobUrl = '';

    this._form!.reset();
    this._render();
  }

  open(): void {
    this._render();
    this._dialog!.showModal();
    this.submitButton!.disabled = false;
  }

  private closeDialog(): void {
    this._dialog!.close();
  }

  private postTryJob(): void {
    this._spinner!.active = true;
    this.submitButton!.disabled = true;
    this.jobUrl = ''; // reset the job URL
    this._render();

    const baseCommit = document.getElementById('base-commit')! as HTMLInputElement;
    const endCommit = document.getElementById('exp-commit')! as HTMLInputElement;
    const traceArgs = document.getElementById('trace-args')! as HTMLInputElement;

    const req: CreateLegacyTryRequest = {
      name: 'Tracing Debug',
      base_git_hash: baseCommit.value,
      end_git_hash: endCommit.value,
      base_patch: '',
      experiment_patch: '',
      configuration: this.testPath.split('/')[1],
      benchmark: this.testPath.split('/')[2],
      story: this.story,
      repository: 'chromium',
      bug_id: '',
      user: this.user,
      extra_test_args: `["--extra-chrome-categories","${traceArgs.value}"]`,
    };
    req.name = `${req.name} on ${req.configuration}/${req.benchmark}/${req.story}`;
    console.log('here is the request', req);
    fetch('/_/try/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: CreatePinpointResponse) => {
        this.submitButton!.disabled = false;
        this._spinner!.active = false;
        this.jobUrl = json.jobUrl;
        this._render();
      })
      .catch((msg: any) => {
        errorMessage(msg);
        this._spinner!.active = false;
        this._render();
      });
  }
}

define('pinpoint-try-job-dialog-sk', PinpointTryJobDialogSk);
