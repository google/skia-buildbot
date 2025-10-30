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
 * request. If succesful, user will see the alert dialog popup at the bottom left of the page
 * and close the dialog. If unsuccesful, an error message toast will appear.
 *
 */

import '../../../elements-sk/modules/select-sk';
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { CreateBisectRequest } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/toast-sk';
import '../../../elements-sk/modules/spinner-sk';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';

const STATISTIC_VALUES = ['avg', 'count', 'max', 'min', 'std', 'sum'];

export interface BisectPreloadParams {
  testPath?: string | '';
  startCommit?: string | '';
  endCommit?: string | '';
  bugId?: string | '';
  story?: string | '';
  anomalyId: string | '';
}

export class BisectDialogSk extends ElementSk {
  bugId: string = '';

  startCommit: string = '';

  endCommit: string = '';

  story: string = '';

  user: string = '';

  testPath: string = '';

  anomalyId: string = '';

  private _dialog: HTMLDialogElement | null = null;

  private spinner: SpinnerSk | null = null;

  private _form: HTMLFormElement | null = null;

  private bisectButton: HTMLButtonElement | null = null;

  private jobId: string = '';

  private _opened: boolean = false;

  private jobUrl: string = '';

  private closeBisectToastButton: HTMLButtonElement | null = null;

  private bisectJobToast: ToastSk | null = null;

  private static template = (ele: BisectDialogSk) => html`
    <dialog id='bisect-dialog'>
      <h2>Bisect</h2>
      <button id="bisectCloseIcon" @click=${ele.closeBisectDialog}>
        <close-icon-sk></close-icon-sk>
      </button>
      <form id="bisect-form">
      <h3>Test Path</h3>
      <input id="testpath" type="text" value=${ele.testPath}></input>
      <h3>Bug ID</h3>
      <input id="bug-id" type="text" value=${ele.bugId}></input>
      <h3>Start Commit</h3>
      <input id="start-commit" type="text" value=${ele.startCommit}></input>
      <h3>End Commit</h3>
      <input id="end-commit" type="text" value=${ele.endCommit}></input>
      <h3>Story</h3>
      <input id="story" type="text" value=${ele.story}></input>
      <h3>Patch to apply to the entire job(optional)</h3>
      <input id="patch" type="text"></input>
      <div class=footer>
        <spinner-sk id="dialog-spinner"></spinner-sk>
        <button id="submit-button" type="submit" @click=${ele.postBisect}>Bisect</button>
        <button id="close-btn" @click=${ele.closeBisectDialog} type="button">Close</button>
      </div>
      </form>
    </dialog>
    <toast-sk id="bisect_toast" duration="5000">
      <div id="bisect-url">
        <a href=${ele.jobUrl} target="_blank">Bisect job created: ${ele.jobId}</a>
      </div>
      <div class="close-bisect">
        <button id="hide-bisect-toast" class="action" type="button">Close</button>
      </div>
    </toast-sk>
    `;

  constructor() {
    super(BisectDialogSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'preloadInputParameters');
    upgradeProperty(this, 'testPath');
    upgradeProperty(this, 'startCommit');
    upgradeProperty(this, 'endCommit');
    upgradeProperty(this, 'bugId');
    upgradeProperty(this, 'story');
    this._render();

    this._dialog = this.querySelector('#bisect-dialog');
    this.spinner = this.querySelector('#dialog-spinner');
    this.bisectButton = this.querySelector('#submit-button');
    this._form = this.querySelector('#bisect-form');
    this.closeBisectToastButton = this.querySelector('#hide-bisect-toast');
    this.bisectJobToast = this.querySelector('#bisect_toast');
    this.closeBisectToastButton!.addEventListener('click', () => this.bisectJobToast?.hide());

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

    this._form!.reset();
    this._render();
  }

  open(): void {
    this._opened = true;
    this._dialog!.showModal();
    this.bisectButton!.disabled = false;
  }

  private closeBisectDialog(): void {
    this._opened = false;
    this._dialog!.close();
  }

  get opened() {
    return this._dialog!.open;
  }

  postBisect(): void {
    this.spinner!.active = true;
    this.bisectButton!.disabled = true;
    if (this.testPath === '') {
      this._render();
      return;
    }
    const parameters = this.testPath.split('/');

    const test = parameters!.at(3);

    const parts: string[] = test!.split(':');
    // Pop up the last element from the array if exists
    const tail = parts.pop();

    let chart: string = test!;
    let statistic: string = '';
    if (tail !== undefined) {
      chart = STATISTIC_VALUES.includes(tail) ? parts!.join('_') : test!;
      statistic = STATISTIC_VALUES.includes(tail) ? tail : '';
    }
    const bugId = document.getElementById('bug-id')! as HTMLInputElement;
    const startCommit = document.getElementById('start-commit')! as HTMLInputElement;
    const endCommit = document.getElementById('end-commit')! as HTMLInputElement;
    this.story = parameters.pop()!;
    const patch = document.getElementById('patch')! as HTMLInputElement;
    const req: CreateBisectRequest = {
      comparison_mode: 'performance',
      start_git_hash: startCommit.value === '' ? this.startCommit : startCommit.value,
      end_git_hash: endCommit.value === '' ? this.endCommit : endCommit.value,
      configuration: this.testPath.split('/')[1],
      benchmark: this.testPath.split('/')[2],
      story: this.story,
      chart: chart,
      statistic: statistic,
      comparison_magnitude: '',
      pin: patch.value,
      project: 'chromium',
      bug_id: bugId.value,
      user: this.user,
      alert_ids: '[' + this.anomalyId + ']',
    };

    const validations = [
      { value: req.start_git_hash, message: 'Start commit is required.' },
      { value: req.end_git_hash, message: 'End commit is required.' },
      { value: req.configuration, message: 'Configuration is missing in the request.' },
      { value: req.benchmark, message: 'Benchmark is missing in the request.' },
      { value: req.story, message: 'Story is required.' },
      { value: req.chart, message: 'Chart is missing in the request.' },
      { value: req.user, message: 'User is not logged in.' },
    ];

    for (const rule of validations) {
      if (!rule.value) {
        errorMessage(rule.message);
        this.spinner!.active = false;
        this.bisectButton!.disabled = false;
        return;
      }
    }

    fetch('/_/bisect/create', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(async (json) => {
        this.bisectButton!.disabled = false;
        this.spinner!.active = false;
        this.jobId = json.jobId;
        this.jobUrl = json.jobUrl;
        this.closeBisectDialog();
        this.bisectJobToast?.show();
        this._render();
      })
      .catch((msg: any) => {
        errorMessage(msg);
        this.spinner!.active = false;
        this._render();
      });
  }

  /** Clear Bisect Dialog fields */
  reset(): void {
    this.startCommit = '';
    this.endCommit = '';
    this.story = '';
    this.user = '';
    this.testPath = '';
    this.anomalyId = '';
    this._render();
  }
}
define('bisect-dialog-sk', BisectDialogSk);
