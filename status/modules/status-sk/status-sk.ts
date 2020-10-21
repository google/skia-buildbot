/**
 * @module modules/status-sk
 * @description <h2><code>status-sk</code></h2>
 *
 * The majority of the Status page.
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../autoroller-status-sk';
import '../commits-table-sk';
import '../gold-status-sk';
import '../navigation-sk';
import '../perf-status-sk';
import '../tree-status-sk';
import 'elements-sk/collapse-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/expand-less-icon-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import { defaultRepo } from '../settings';
import { TreeStatus } from '../tree-status-sk/tree-status-sk';

export class StatusSk extends ElementSk {
  private repo: string = defaultRepo();
  private autorollersOpen: boolean = true;
  private perfOpen: boolean = true;
  private goldOpen: boolean = true;
  private static template = (el: StatusSk) =>
    html`
      <app-sk>
        <header>
          <h1>Status: ${el.repo}</h1>
          <div class="spacer">
            <tree-status-sk
              @tree-status-update=${(e: CustomEvent<TreeStatus>) => el.updateTreeStatus(e.detail)}
            ></tree-status-sk>
          </div>
          <login-sk></login-sk>
          <theme-chooser-sk></theme-chooser-sk>
        </header>
        <aside>
          <div><navigation-sk></navigation-sk></div>
          <div>
            <button
              class="collapser"
              @click=${(e: Event) => {
                el.autorollersOpen = !el.autorollersOpen;
                el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
              }}
            >
              ${el.autorollersOpen
                ? html`<expand-less-icon-sk></expand-less-icon-sk>`
                : html`<expand-more-icon-sk></expand-more-icon-sk>`}
              AutoRollers
            </button>
            <collapse-sk>
              <autoroller-status-sk></autoroller-status-sk>
            </collapse-sk>
          </div>
          <div>
            <button
              class="collapser"
              @click=${(e: Event) => {
                el.perfOpen = !el.perfOpen;
                el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
              }}
            >
              ${el.perfOpen
                ? html`<expand-less-icon-sk></expand-less-icon-sk>`
                : html`<expand-more-icon-sk></expand-more-icon-sk>`}
              Perf
            </button>
            <collapse-sk>
              <perf-status-sk></perf-status-sk>
            </collapse-sk>
          </div>
          <div>
            <button
              class="collapser"
              @click=${(e: Event) => {
                el.goldOpen = !el.goldOpen;
                el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
              }}
            >
              ${el.goldOpen
                ? html`<expand-less-icon-sk></expand-less-icon-sk>`
                : html`<expand-more-icon-sk></expand-more-icon-sk>`}
              Gold
            </button>
            <collapse-sk>
              <gold-status-sk></gold-status-sk>
            </collapse-sk>
          </div>
        </aside>

        <main>
          <commits-table-sk
            @repo-changed=${(e: CustomEvent) => el.updateRepo(e.detail)}
          ></commits-table-sk>
          <error-toast-sk></error-toast-sk>
        </main>

        <footer><error-toast-sk></error-toast-sk></footer>
      </app-sk>
    `;

  constructor() {
    super(StatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this.updateRepo(defaultRepo());
  }

  private toggle(collapseSk: any) {
    collapseSk.closed = !collapseSk.closed;
    this._render();
  }

  private updateRepo(r: string) {
    this.repo = r.charAt(0).toUpperCase() + r.slice(1);
    this._render();
  }

  private updateTreeStatus(r: TreeStatus) {
    // We use css to style the header color based on state.
    this.setAttribute('state', r.status.general_state || '');
  }
}

define('status-sk', StatusSk);
