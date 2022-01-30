/**
 * @module module/perf-scaffold-sk
 * @description <h2><code>perf-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the Perf pages. The rest of
 * every Perf page should be a child of this element.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/add-alert-icon-sk';
import 'elements-sk/icon/build-icon-sk';
import 'elements-sk/icon/event-icon-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/sort-icon-sk';
import 'elements-sk/icon/trending-up-icon-sk';
import '../../../infra-sk/modules/alogin-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

// The ID of a top level element under perf-scaffold-sk that will be moved under
// the right hand side nav bar.
const SIDEBAR_HELP_ID = 'sidebar_help';

/**
 * Moves the elements from a list to be the children of the target element.
 *
 * @param from - The list of elements we are moving.
 * @param to - The new parent.
 */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class PerfScaffoldSk extends ElementSk {
  private _main: HTMLElement | null = null;

  private _help: HTMLElement | null = null;

  constructor() {
    super(PerfScaffoldSk.template);
  }

  private static template = (ele: PerfScaffoldSk) => html`
  <app-sk>
    <header id=topbar>
      <h1 class=name>Perf</h1>
      <div class=spacer></div>
      <alogin-sk url=/_/login/status></alogin-sk>
      <theme-chooser-sk></theme-chooser-sk>
    </header>
    <aside id=sidebar>
      <div id=links>
        <a href="/e/" tab-index=0 ><home-icon-sk></home-icon-sk><span>Home</span></a>
        <a href="/t/" tab-index=0 ><trending-up-icon-sk></trending-up-icon-sk><span>Triage</span></a>
        <a href="/a/" tab-index=0 ><add-alert-icon-sk></add-alert-icon-sk><span>Alerts</span></a>
        <a href="/d/" tab-index=0 ><build-icon-sk></build-icon-sk><span>Dry Run</span></a>
        <a href="/c/" tab-index=0 ><sort-icon-sk></sort-icon-sk><span>Clustering<span></a>
        <a href="http://go/perf-user-doc" tab-index=0 ><help-icon-sk></help-icon-sk><span>Help</span></a>
      </div>
      <div id=help>
      </div>
    </aside>
    <main>
    </main>
    <footer>
      <error-toast-sk></error-toast-sk>
    </footer>
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
    this._help = this.querySelector('#help');

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

  // Place these newly added nodes in the right place under the perf-scaffold-sk
  // element.
  private redistributeAddedNodes(from: NodeList) {
    Array.prototype.slice.call(from).forEach((node: Node) => {
      if ((node as Element).id === SIDEBAR_HELP_ID) {
        this._help!.appendChild(node);
      } else {
        this._main!.appendChild(node);
      }
    });
  }
}

define('perf-scaffold-sk', PerfScaffoldSk);
