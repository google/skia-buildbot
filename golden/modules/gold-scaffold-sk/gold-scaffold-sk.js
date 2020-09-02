/**
 * @module modules/gold-scaffold-sk
 * @description <h2><code>gold-scaffold-sk</code></h2>
 *
 * Contains the title bar, side bar, and error-toast for all the Gold pages. The rest of
 * every Gold page should be a child of this element.
 *
 * Has a spinner-sk that can be activated when it hears "begin-task" events and keeps
 * spinning until it hears an equal number of "end-task" events.
 *
 * The error-toast element responds to fetch-error events and normal error-sk events.
 *
 * @attr {boolean} testing_offline - If we should operate entirely in offline mode.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { title } from '../settings';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';

import '../last-commit-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/find-in-page-icon-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/label-icon-sk';
import 'elements-sk/icon/laptop-chromebook-icon-sk';
import 'elements-sk/icon/list-icon-sk';
import 'elements-sk/icon/search-icon-sk';
import 'elements-sk/icon/sync-problem-icon-sk';
import 'elements-sk/icon/view-day-icon-sk';
import 'elements-sk/spinner-sk';

const template = (ele) => html`
<app-sk>
  <header>
    <h1>${title()}</h1>
    <div class=spinner-spacer>
      <spinner-sk></spinner-sk>
    </div>
    <div class=spacer></div>
    <last-commit-sk></last-commit-sk>
    <login-sk ?testing_offline=${ele.testingOffline} .loginHost=${window.location.host}></login-sk>
  </header>

  <aside>
    <nav>
      <a href="/" tab-index=0>
        <home-icon-sk></home-icon-sk><span>Home</span>
      </a>
      <a href="/" tab-index=0>
        <view-day-icon-sk></view-day-icon-sk><span>By Blame<span>
      </a>
      <a href="/list" tab-index=0>
        <list-icon-sk></list-icon-sk><span>By Test</span>
      </a>
      <a href="/changelists" tab-index=0>
        <laptop-chromebook-icon-sk></laptop-chromebook-icon-sk><span>By ChangeList</span>
      </a>
      <a href="/search" tab-index=0>
        <search-icon-sk></search-icon-sk><span>Search</span>
      </a>
      <a href="/ignores" tab-index=0>
        <label-icon-sk></label-icon-sk><span>Ignores</span>
      </a>
      <a href="/triagelog" tab-index=0>
        <find-in-page-icon-sk></find-in-page-icon-sk><span>Triage Log</span>
      </a>
      <a href="/help" tab-index=0>
        <help-icon-sk></help-icon-sk><span>Help</span>
      </a>
      <a href="https://github.com/google/skia-buildbot/tree/master/golden" tab-index=0>
        <folder-icon-sk></folder-icon-sk><span>Code</span>
      </a>
    </nav>
  </aside>

  <main></main>

  <footer><error-toast-sk></error-toast-sk></footer>
</app-sk>
`;

/**
 * Moves the elements from one NodeList to another NodeList.
 *
 * @param {NodeList} from - The list we are moving from.
 * @param {NodeList} to - The list we are moving to.
 */
function move(from, to) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

define('gold-scaffold-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._main = null;
    this._busyTaskCount = 0;
    this._spinner = null;
  }

  connectedCallback() {
    super.connectedCallback();
    // Don't call more than once.
    if (this._main) {
      return;
    }
    this.addEventListener('begin-task', this._addBusyTask);
    this.addEventListener('end-task', this._finishedTask);
    this.addEventListener('fetch-error', this._fetchError);

    // We aren't using shadow dom so we need to manually move the children of
    // gold-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

    this._spinner = this.querySelector('header spinner-sk');

    // Move the old children back under main.
    this._main = this.querySelector('main');
    move(div.children, this._main);

    // Move all future children under main also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        move(mut.addedNodes, this._main);
      });
    });
    observer.observe(this, { childList: true });
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.removeEventListener('begin-task', this._addBusyTask);
    this.removeEventListener('end-task', this._finishedTask);
    this.removeEventListener('fetch-error', this._fetchError);
  }

  /** @prop {boolean} busy Indicates if there any on-going tasks (e.g. RPCs).
   *                  This also mirrors the status of the embedded spinner-sk.
   *                  Read-only. */
  get busy() { return !!this._busyTaskCount; }

  /** @prop testingOffline {boolean} Reflects the testing_offline attribute for ease of use.
   */
  get testingOffline() { return this.hasAttribute('testing_offline'); }

  set testingOffline(val) {
    if (val) {
      this.setAttribute('testing_offline', '');
    } else {
      this.removeAttribute('testing_offline');
    }
  }

  /**
   * Indicate there are some number of tasks (e.g. RPCs) the app is waiting on
   * and should be in the "busy" state, if it isn't already.
   *
   */
  _addBusyTask() {
    this._busyTaskCount++;
    if (this._spinner && this._busyTaskCount > 0) {
      this._spinner.active = true;
    }
  }

  /**
   * Removes one task from the busy count. If there are no more tasks to wait
   * for, the app will leave the "busy" state and emit the "busy-end" event.
   *
   */
  _finishedTask() {
    this._busyTaskCount--;
    if (this._busyTaskCount <= 0) {
      this._busyTaskCount = 0;
      if (this._spinner) {
        this._spinner.active = false;
      }
      this.dispatchEvent(new CustomEvent('busy-end', { bubbles: true }));
    }
  }

  /** Handles a fetch error. Expects the detail of error to contain:
   *  error: the error given to fetch.
   *  loading: A string explaining what was being fetched.
   */
  _fetchError(e) {
    const loadingWhat = e.detail.loading;
    e = e.detail.error;
    if (e.name !== 'AbortError') {
      // We can ignore AbortError since they fire anytime an AbortController was canceled.
      // Chrome and Firefox report a DOMException in this case:
      // https://developer.mozilla.org/en-US/docs/Web/API/DOMException
      console.error(e);
      errorMessage(`Unexpected error loading ${loadingWhat}: ${e.message}`,
        5000);
    }
    this._finishedTask();
  }
});
