/**
 * @module module/compile-scaffold-sk
 * @description <h2><code>compile-scaffold-sk</code></h2>
 *
 * <p>
 *   Contains the title bar and error-toast for all the compile server pages.
 *   The rest of pages should be a child of this element.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';

import '../../../infra-sk/modules/login-sk';

const template = (ele) => html`
  <nav>
    <nav-button-sk></nav-button-sk>
    <nav-links-sk>
      <a href="/" tab-index=0 ><home-icon-sk></home-icon-sk><span>Compile tasks</span></a>
      <a href="https://goto.google.com/skia-android-framework-compile-bot" tab-index=0 ><help-icon-sk></help-icon-sk><span>Docs</span></a>
      <a href="https://github.com/google/skia-buildbot/tree/master/android_compile" tab-index=0 ><folder-icon-sk></folder-icon-sk><span>Code</span></a>
    </nav-links-sk>
    <h1 class=name>${ele.appTitle}</h1>
    <login-sk login_host="skia-android-compile.corp.goog"></login-sk>
  </nav>
  <main>
  </main>
  <error-toast-sk></error-toast-sk>
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

define('compile-scaffold-sk', class extends ElementSk {
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
    // compile-scaffold-sk to be children of 'main'. We have to do this for the
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

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});
