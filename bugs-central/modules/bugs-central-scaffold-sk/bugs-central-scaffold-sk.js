/**
 * @module module/bugs-central-scaffold-sk
 * @description <h2><code>bugs-central-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the bugs central pages. The
 * rest of pages should be a child of this element.
 *
 * Has a spinner-sk that can be activated when it hears "begin-fetch" events and keeps
 * spinner until it hears an equal number of "end-fetch" events..
 *
 * The error-toast element responds to fetcherror events and normal error-sk events..
 *
 * @attr {string} app_title - The title to show in the page banner.
 *
 * @attr {boolean} testing_offline - If we should operate entirely in offline mode.

 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { errorMessage } from 'elements-sk/errorMessage';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/android-icon-sk';
import 'elements-sk/icon/bug-report-icon-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/person-pin-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import 'elements-sk/spinner-sk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

const template = (ele) => html`
<app-sk>
  <header class=primary-container-themes-sk>
    <h1>${ele.appTitle}</h1>
    <div class=spinner-spacer>
      <spinner-sk></spinner-sk>
    </div>
    <div class=spacer></div>
    <login-sk ?testing_offline=${ele.testingOffline}></login-sk>
    <theme-chooser-sk></theme-chooser-sk>
  </header>

  <aside>
    <nav class=surface-themes-sk>
      <a href="/" tab-index=0 >
        <home-icon-sk></home-icon-sk><span>Bugs Central</span>
      </a>
      <a href="/?client=Android" tab-index=0 >
        <person-pin-icon-sk></person-pin-icon-sk><span>Android Client</span>
      </a>
      <a href="/?client=Chromium" tab-index=0 >
        <person-pin-icon-sk></person-pin-icon-sk><span>Chromium Client</span>
      </a>
      <a href="/?client=Flutter-native" tab-index=0 >
        <person-pin-icon-sk></person-pin-icon-sk><span>Flutter-native Client</span>
      </a>
      <a href="/?client=Flutter-on-web" tab-index=0 >
        <person-pin-icon-sk></person-pin-icon-sk><span>Flutter-on-web Client</span>
      </a>
      <a href="/?client=Skia" tab-index=0 >
        <person-pin-icon-sk></person-pin-icon-sk><span>Skia Client</span>
      </a>
      <a href="http://go/skia-bugs-central" tab-index=0>
        <help-icon-sk></help-icon-sk><span>Help</span>
      </a>
      <a href="https://github.com/google/skia-buildbot/tree/master/bugs-central" tab-index=0>
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

define('bugs-central-scaffold-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._main = null;
  }

  connectedCallback() {
    super.connectedCallback();
    // Don't call more than once.
    if (this._main) {
      return;
    }
    // We aren't using shadow dom so we need to manually move the children of
    // bugs-central-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

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

  /** @prop appTitle {string} Reflects the app_title attribute for ease of use. */
  get appTitle() { return this.getAttribute('app_title'); }

  set appTitle(val) { this.setAttribute('app_title', val); }

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

  disconnectedCallback() {
    super.disconnectedCallback();
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
      errorMessage(`Unexpected error loading ${loadingWhat}: ${e.message}`,
        5000);
    }
    this._finishedTask();
  }
});
