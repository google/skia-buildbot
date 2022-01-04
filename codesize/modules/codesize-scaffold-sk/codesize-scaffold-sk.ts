/** Root element for all codesize.skia.org pages. */
import { html, TemplateResult } from 'lit-html';

import { define } from 'elements-sk/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

/** Moves the elements in a NodeList or HTMLCollection as children of another element. */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class CodesizeScaffoldSk extends ElementSk {
  private static template = (): TemplateResult => html`
    <app-sk>
      <header>
        <h1>Skia Code Size</h1>
        <div class="spacer"></div>
        <theme-chooser-sk></theme-chooser-sk>
      </header>

      <aside>
        <ul>
          <li><a href="/">Home</a></li>
        </ul>
      </aside>

      <main></main>
    </app-sk>
  `;

  private main: HTMLElement | null = null;

  constructor() {
    super(CodesizeScaffoldSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    // Don't call more than once.
    if (this.main) {
      return;
    }

    // We aren't using shadow dom so we need to manually move the children of codesize-scaffold-sk
    // to be children of 'main'. We have to do this for the existing elements and for all future
    // mutations.

    // Create a temporary holding spot for the elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render the template.
    this._render();

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
}

define('codesize-scaffold-sk', CodesizeScaffoldSk);
