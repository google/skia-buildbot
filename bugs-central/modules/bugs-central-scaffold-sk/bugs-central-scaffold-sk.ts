/**
 * @module module/bugs-central-scaffold-sk
 * @description <h2><code>bugs-central-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the bugs central pages. The
 * rest of pages should be a child of this element.
 *
 * @attr {string} app_title - The title to show in the page banner.
 *
 * @attr {boolean} testing_offline - If we should operate entirely in offline mode.

 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/insert-chart-icon-sk';
import 'elements-sk/icon/person-pin-icon-sk';
import 'elements-sk/spinner-sk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

/**
 * Moves the elements from a list to be the children of the target element.
 *
 * @param from - The list of elements we are moving.
 * @param to - The new parent.
 */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class BugsCentralScaffoldSk extends ElementSk {
  private _main: HTMLElement | null = null;

  constructor() {
    super(BugsCentralScaffoldSk.template);
  }

  private static template = (el: BugsCentralScaffoldSk) => html`
  <app-sk>
  <header class=primary-container-themes-sk>
    <h1>${el.appTitle}</h1>
    <div class=spinner-spacer>
      <spinner-sk></spinner-sk>
    </div>
    <div class=spacer></div>
    <login-sk ?testing_offline=${el.testingOffline}></login-sk>
    <theme-chooser-sk></theme-chooser-sk>
  </header>

  <aside>
    <nav class=surface-themes-sk>
      <a href="/" tab-index=0 >
        <home-icon-sk></home-icon-sk><span>All Clients</span>
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
      <a href="https://estado.googleplex.com/dashboard/140965800253573027/buganizer" target="_blank" tab-index=0>
        <insert-chart-icon-sk></insert-chart-icon-sk><span>Estado Dashboard</span>
      </a>
      <a href="http://go/skia-bugs-central" target="_blank" tab-index=0>
        <help-icon-sk></help-icon-sk><span>Help</span>
      </a>
      <a href="https://github.com/google/skia-buildbot/tree/master/bugs-central" target="_blank" tab-index=0>
        <folder-icon-sk></folder-icon-sk><span>Code</span>
      </a>
    </nav>
  </aside>

  <main></main>

  <footer><error-toast-sk></error-toast-sk></footer>
  </app-sk>
`;

  connectedCallback(): void {
    super.connectedCallback();
    // Don't call more than once.
    if (this._main) {
      return;
    }
    // We aren't using shadow dom so we need to manually move the children of
    // perf-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

    this._main = this.querySelector('main');

    // Move the old children back.
    this.redistributeAddedNodes(div.childNodes);

    // Move all future children also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        this.redistributeAddedNodes(mut.addedNodes);
      });
    });
    observer.observe(this, { childList: true });
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  // Place these newly added nodes in the right place under the bugs-central-scaffold-sk
  // element.
  private redistributeAddedNodes(from: NodeList) {
    Array.prototype.slice.call(from).forEach((node: Node) => {
      this._main!.appendChild(node);
    });
  }


  /** @prop appTitle {string} Reflects the app_title attribute for ease of use. */
  get appTitle(): string { return this.getAttribute('app_title')!; }

  set appTitle(val: string) { this.setAttribute('app_title', val); }

  /** @prop testingOffline {boolean} Reflects the testing_offline attribute for ease of use.
   */
  get testingOffline(): boolean { return this.hasAttribute('testing_offline'); }

  set testingOffline(val: boolean) {
    if (val) {
      this.setAttribute('testing_offline', '');
    } else {
      this.removeAttribute('testing_offline');
    }
  }
}

define('bugs-central-scaffold-sk', BugsCentralScaffoldSk);
