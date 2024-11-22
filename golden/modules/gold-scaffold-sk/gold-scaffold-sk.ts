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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { title } from '../settings';
import { FetchErrorEventDetail } from '../common';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/alogin-sk';

import '../last-commit-sk';

import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/find-in-page-icon-sk';
import '../../../elements-sk/modules/icons/folder-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import '../../../elements-sk/modules/icons/home-icon-sk';
import '../../../elements-sk/modules/icons/label-icon-sk';
import '../../../elements-sk/modules/icons/laptop-chromebook-icon-sk';
import '../../../elements-sk/modules/icons/list-icon-sk';
import '../../../elements-sk/modules/icons/search-icon-sk';
import '../../../elements-sk/modules/icons/sync-problem-icon-sk';
import '../../../elements-sk/modules/icons/view-day-icon-sk';
import '../../../elements-sk/modules/spinner-sk';

/** Moves the elements in a NodeList or HTMLCollection as children of another element. */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class GoldScaffoldSk extends ElementSk {
  private static template = (ele: GoldScaffoldSk) => html`
    <app-sk>
      <header>
        <h1>${title()}</h1>
        <div class=spinner-spacer>
          <spinner-sk></spinner-sk>
        </div>
        <div class=spacer></div>
        <last-commit-sk></last-commit-sk>
        <alogin-sk ?testing_offline=${ele.testingOffline}></alogin-sk>
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

  private main: HTMLElement | null = null;

  private busyTaskCount = 0;

  private spinner: SpinnerSk | null = null;

  constructor() {
    super(GoldScaffoldSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    // Don't call more than once.
    if (this.main) {
      // We still want to re-render in case the GoldSettings have changed (e.g. the title), which
      // is common in tests.
      this._render();
      return;
    }
    this.addEventListener('begin-task', this.addBusyTask);
    this.addEventListener('end-task', this.finishedTask);
    this.addEventListener('fetch-error', this.fetchError);

    // We aren't using shadow dom so we need to manually move the children of
    // gold-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

    this.spinner = this.querySelector<SpinnerSk>('header spinner-sk');

    // Move the old children back under main.
    this.main = this.querySelector('main');
    move(div.children, this.main!);

    // Move all future children under main also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        move(mut.addedNodes, this.main!);
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

  /**
   * Indicates if there any on-going tasks (e.g. RPCs). This also mirrors the status of the
   * embedded spinner-sk.
   */
  get busy() {
    return !!this.busyTaskCount;
  }

  /** Reflects the testing_offline attribute for ease of use. */
  get testingOffline() {
    return this.hasAttribute('testing_offline');
  }

  set testingOffline(val) {
    if (val) {
      this.setAttribute('testing_offline', '');
    } else {
      this.removeAttribute('testing_offline');
    }
  }

  /**
   * Indicate there are some number of tasks (e.g. RPCs) the app is waiting on and should be in the
   * "busy" state, if it isn't already.
   */
  private addBusyTask() {
    this.busyTaskCount++;
    if (this.spinner && this.busyTaskCount > 0) {
      this.spinner.active = true;
    }
  }

  /**
   * Removes one task from the busy count. If there are no more tasks to wait for, the app will
   * leave the "busy" state and emit the "busy-end" event.
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

  /** Handles a fetch-error event. */
  private fetchError(e: Event) {
    // Method removeEventListener expects an Event, so we're forced to take an Event and cast it as
    // a CustomEvent here.
    const fetchErrorEvent = e as CustomEvent<FetchErrorEventDetail>;
    const error = fetchErrorEvent.detail.error;
    const loadingWhat = fetchErrorEvent.detail.loading;
    if (fetchErrorEvent.detail.error.name !== 'AbortError') {
      // We can ignore AbortError since they fire anytime an AbortController was canceled.
      // Chrome and Firefox report a DOMException in this case:
      // https://developer.mozilla.org/en-US/docs/Web/API/DOMException
      console.error(error);
      errorMessage(
        `Unexpected error loading ${loadingWhat}: ${error.message}`,
        5000
      );
    }
    this.finishedTask();
  }
}

define('gold-scaffold-sk', GoldScaffoldSk);
