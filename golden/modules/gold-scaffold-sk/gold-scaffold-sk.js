/**
 * @module module/gold-scaffold-sk
 * @description <h2><code>gold-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the Gold pages. The rest of
 * every Gold page should be a child of this element.
 *
 */
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import '../../../infra-sk/modules/app-sk'
import '../../../infra-sk/modules/login-sk'

import 'elements-sk/error-toast-sk'
// import 'elements-sk/icon/add-alert-icon-sk'

import 'elements-sk/icon/folder-icon-sk'
import 'elements-sk/icon/help-icon-sk'
import 'elements-sk/icon/home-icon-sk'
import 'elements-sk/icon/view-day-icon-sk'
import 'elements-sk/icon/label-icon-sk'
import 'elements-sk/icon/search-icon-sk'
import 'elements-sk/icon/laptop-chromebook-icon-sk'
import 'elements-sk/icon/list-icon-sk'
import 'elements-sk/icon/find-in-page-icon-sk'
import 'elements-sk/icon/sync-problem-icon-sk'

const template = (ele) => html`
<app-sk>
  <header>
    <h1>Gold</h1>
    <div class=spacer></div>
    <!-- TODO(kjlubick) last commit -->
    <login-sk></login-sk>
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
  }

  connectedCallback() {
    super.connectedCallback();
    // Don't call more than once.
    if (this._main) {
      return
    }
    // We aren't using shadow dom so we need to manually move the children of
    // gold-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div')
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
    observer.observe(this, {childList: true});
  }


});
