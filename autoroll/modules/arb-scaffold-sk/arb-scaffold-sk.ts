/**
 * @module autoroll/modules/arb-scaffold-sk
 * @description <h2><code>arb-scaffold-sk</code></h2>
 *
 * Contains the title bar, side bar, and error-toast for all the Autoroll pages.
 * The rest of every Autoroll page should be a child of this element.
 *
 * Has a spinner-sk that can be activated when it hears "begin-task" events and
 * keeps spinning until it hears an equal number of "end-task" events.
 *
 * The error-toast element responds to fetch-error events and normal error-sk
 * events.
 *
 * @attr {string} app_title - The title to show in the page banner.
 *
 * @attr {boolean} testing_offline - If we should operate entirely in offline
 *     mode.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/spinner-sk';

/**
 * Moves the elements from one NodeList to the given HTMLElement.
 */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class ARBScaffoldSk extends ElementSk {
  private main: HTMLElement | null = null;

  private busyTaskCount: number = 0;

  private spinner: SpinnerSk | null = null;

  private static template = (ele: ARBScaffoldSk) => html`
<app-sk>
  <header class=primary-container-themes-sk>
    <h1>${ele.title}</h1>
    <div class=spinner-spacer>
      <spinner-sk></spinner-sk>
    </div>
    <div class=spacer></div>
    <login-sk ?testing_offline=${ele.testingOffline} login_host="${ele.loginHost}"></login-sk>
    <theme-chooser-sk></theme-chooser-sk>
  </header>

  <aside class=surface-themes-sk>
    <nav>
      <a href="/" tab-index=0>
        <home-icon-sk></home-icon-sk><span>Home</span>
      </a>
      <a href="https://skia.googlesource.com/buildbot/+/main/autoroll/README.md" tab-index=0>
        <help-icon-sk></help-icon-sk><span>Docs</span>
      </a>
    </nav>
  </aside>

  <main></main>

  <footer><error-toast-sk></error-toast-sk></footer>
</app-sk>
`;

  constructor() {
    super(ARBScaffoldSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    // Don't call more than once.
    if (this.main) {
      return;
    }
    this.addEventListener('begin-task', this.addBusyTask);
    this.addEventListener('end-task', this.finishedTask);
    this.addEventListener('fetch-error', this.fetchError);

    // We aren't using shadow dom so we need to manually move the children of
    // arb-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

    this.spinner = this.querySelector('header spinner-sk');

    // Move the old children back under main.
    this.main = this.querySelector('main');
    if (this.main) {
      move(div.children, this.main);
    }

    // Move all future children under main also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        if (this.main) {
          move(mut.addedNodes, this.main);
        }
      });
    });
    observer.observe(this, { childList: true });
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.removeEventListener('begin-task', this.addBusyTask);
    this.removeEventListener('end-task', this.finishedTask);
    this.removeEventListener('fetch-error', this.fetchError);
  }

  /** @prop loginHost Host name used for login. */
  get loginHost() { return window.location.host; }

  /** @prop title Reflects the app_title attribute for ease of use. */
  get title() { return <string> this.getAttribute('title'); }

  set title(val: string) { this.setAttribute('title', val); }

  /** @prop busy Indicates if there any on-going tasks (e.g. RPCs). This also
   * mirrors the status of the embedded spinner-sk. Read-only. */
  get busy() { return !!this.busyTaskCount; }

  /** @prop testingOffline {boolean} Reflects the testing_offline attribute for
   * ease of use. */
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
   */
  private addBusyTask() {
    this.busyTaskCount++;
    if (this.spinner && this.busyTaskCount > 0) {
      this.spinner.active = true;
    }
  }

  /**
   * Removes one task from the busy count. If there are no more tasks to wait
   * for, the app will leave the "busy" state and emit the "busy-end" event.
   */
  private finishedTask() {
    this.busyTaskCount--;
    if (this.busyTaskCount <= 0) {
      this.busyTaskCount = 0;
      if (this.spinner) {
        this.spinner.active = false;
      }
      this.dispatchEvent(new CustomEvent('busy-end', { bubbles: true }));
    }
  }

  /** Handles a fetch error. Expects the detail of error to contain:
   *  error: the error given to fetch.
   *  loading: A string explaining what was being fetched.
   */
  private fetchError(e: any) {
    const loadingWhat = e.detail.loading;
    e = e.detail.error;
    if (e.name !== 'AbortError') {
      // We can ignore AbortError since they fire anytime an AbortController was
      // canceled. Chrome and Firefox report a DOMException in this case:
      // https://developer.mozilla.org/en-US/docs/Web/API/DOMException
      errorMessage(`Unexpected error loading ${loadingWhat}: ${e.message}`,
        5000);
    }
    this.finishedTask();
  }
}

define('arb-scaffold-sk', ARBScaffoldSk);
