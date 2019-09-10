/**
 * @module module/gold-scaffold-sk
 * @description <h2><code>gold-scaffold-sk</code></h2>
 *
 * Contains the title bar, side bar, and error-toast for all the Gold pages. The rest of
 * every Gold page should be a child of this element.
 *
 */
import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import { html } from 'lit-html'

import '../../../infra-sk/modules/app-sk'
import '../../../infra-sk/modules/login-sk'

import 'elements-sk/error-toast-sk'
import 'elements-sk/icon/find-in-page-icon-sk'
import 'elements-sk/icon/folder-icon-sk'
import 'elements-sk/icon/help-icon-sk'
import 'elements-sk/icon/home-icon-sk'
import 'elements-sk/icon/label-icon-sk'
import 'elements-sk/icon/laptop-chromebook-icon-sk'
import 'elements-sk/icon/list-icon-sk'
import 'elements-sk/icon/search-icon-sk'
import 'elements-sk/icon/sync-problem-icon-sk'
import 'elements-sk/icon/view-day-icon-sk'
import 'elements-sk/spinner-sk'

const template = (ele) => html`
<app-sk>
  <header>
    <h1>${ele.app_title}</h1>
    <div class=spinner-spacer>
      <spinner-sk></spinner-sk>
    </div>
    <div class=spacer></div>
    <!-- TODO(kjlubick) last commit -->
    <login-sk ?testing_offline=${!!ele.testing_offline}></login-sk>
  </header>

  <aside>
    <nav>
      <a href="/" tab-index=0><home-icon-sk></home-icon-sk><span>Home</span></a>
      <a href="/" tab-index=0><view-day-icon-sk></view-day-icon-sk><span>By Blame<span></a>
      <a href="/list" tab-index=0><list-icon-sk></list-icon-sk><span>By Test</span></a>
      <a href="/changelists" tab-index=0><laptop-chromebook-icon-sk></laptop-chromebook-icon-sk><span>By ChangeList</span></a>
      <a href="/search" tab-index=0><search-icon-sk></search-icon-sk><span>Search</span></a>
      <a href="/ignores" tab-index=0><label-icon-sk></label-icon-sk><span>Ignores</span></a>
      <a href="/triagelog" tab-index=0><find-in-page-icon-sk></find-in-page-icon-sk><span>Triage Log</span></a>
      <a href="/failures" tab-index=0><sync-problem-icon-sk></sync-problem-icon-sk><span>Failures</span></a>
      <a href="/help" tab-index=0><help-icon-sk></help-icon-sk><span>Help</span></a>
      <a href="https://github.com/google/skia-buildbot/tree/master/golden" tab-index=0 ><folder-icon-sk></folder-icon-sk><span>Code</span></a>
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
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele))
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
      return
    }
    this._initPropertyFromAttrOrProperty("app_title");
    this._initPropertyFromAttrOrProperty("testing_offline");
    // We aren't using shadow dom so we need to manually move the children of
    // gold-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div')
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
    observer.observe(this, {childList: true});
  }

  /** @prop {boolean} busy Indicates if there any on-going tasks (e.g. RPCs).
   *                  This also mirrors the status of the embedded spinner-sk.
   *                  Read-only. */
  get busy() { return !!this._busyTaskCount;}

  /**
   * Indicate there are some number of tasks (e.g. RPCs) the app is waiting on
   * and should be in the "busy" state, if it isn't already.
   *
   * @param {Number} count - Number of tasks to wait for. Should be positive.
   */
  addBusyTasks(count) {
    this._busyTaskCount += count;
    if (this._spinner && this._busyTaskCount > 0) {
      this._spinner.active = true;
    }
  }

  /**
   * Removes one task from the busy count. If there are no more tasks to wait
   * for, the app will leave the "busy" state and emit the "busy-end" event.
   *
   */
  finishedTask() {
    this._busyTaskCount--;
    if (this._busyTaskCount <= 0) {
      this._busyTaskCount = 0;
      if (this._spinner) {
        this._spinner.active = false;
      }
      this.dispatchEvent(new CustomEvent('busy-end', {bubbles: true}));
    }
  }

  _initPropertyFromAttrOrProperty(prop, removeAttr=true) {
    this._upgradeProperty(prop);
    if (this[prop] === undefined && this.hasAttribute(prop)) {
      this[prop] = this.getAttribute(prop);
      if (removeAttr) {
        this.removeAttribute(prop);
      }
    }
  }

  /** Handles a fetch error
      @param {Object} e The error given by fetch.
      @param {String} loadingWhat A short string to describe what failed.
                      (e.g. bots/list if the bots/list endpoint was queried)
   */
  fetchError(e, loadingWhat) {
    if (e.name !== 'AbortError') {
      // We can ignore AbortError since they fire anytime we page.
      // Chrome and Firefox report a DOMException in this case:
      // https://developer.mozilla.org/en-US/docs/Web/API/DOMException
      console.error(e);
      errorMessage(`Unexpected error loading ${loadingWhat}: ${e.message}`,
                   5000);
    }
    this.finishedTask();
  }


});
