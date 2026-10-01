/**
 * @module modules/status-experimental-sk
 * @description <h2><code>status-experimental-sk</code></h2>
 *
 * The majority of the Status page.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { $$ } from '../../../infra-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/alogin-sk';
import '../autoroller-status-sk';
import '../bugs-status-sk';
import '../commits-table-experimental-sk';
import '../failure-classes-sk';
import '../gold-status-sk';
import '../navigation-sk';
import '../perf-status-sk';
import '../rotations-sk';
import '../tree-status-sk';
import '../../../elements-sk/modules/collapse-sk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/expand-less-icon-sk';
import '../../../elements-sk/modules/icons/expand-more-icon-sk';
import { defaultRepo, repoUrl, treeStatusBaseUrl } from '../settings';
import { TreeStatus } from '../tree-status-sk/tree-status-sk';
import { RotationsSk } from '../rotations-sk/rotations-sk';
import { AutorollerStatus } from '../rpc';
import { BranchesSk } from '../branches-sk/branches-sk';
import { CommitsTableExperimentalSk } from '../commits-table-experimental-sk/commits-table-experimental-sk';
import { DetailsDialogSk } from '../details-dialog-sk/details-dialog-sk';
import { ActiveFailureClass, FailureClassesSk } from '../failure-classes-sk/failure-classes-sk';

export class StatusExperimentalSk extends ElementSk {
  private repo: string = defaultRepo();

  private autorollersOpen: boolean = true;

  private bugsOpen: boolean = true;

  private perfOpen: boolean = true;

  private goldOpen: boolean = true;

  private navOpen: boolean = true;

  private rotationsOpen: boolean = true;

  private failureClassesOpen: boolean = false;

  constructor() {
    super(StatusExperimentalSk.template);
  }

  private static template = (el: StatusExperimentalSk) => html`
    <app-sk>
      <header>
        <h1><a href="/">Status</a>: ${el.repo}</h1>
        <div class="spacer">
          <tree-status-sk
            .baseURL=${treeStatusBaseUrl()}
            .repo=${el.repo}
            @tree-status-update=${(e: CustomEvent<TreeStatus>) =>
              el.updateTreeStatus(e.detail)}></tree-status-sk>
        </div>
        <alogin-sk></alogin-sk>
        <theme-chooser-sk></theme-chooser-sk>
      </header>
      <aside>
        <div>
          <button
            class="collapser"
            @click=${(e: Event) => {
              el.navOpen = !el.navOpen;
              el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
            }}>
            ${el.navOpen
              ? html`<expand-less-icon-sk></expand-less-icon-sk>`
              : html`<expand-more-icon-sk></expand-more-icon-sk>`}
            Navigation
          </button>
          <collapse-sk>
            <navigation-sk></navigation-sk>
          </collapse-sk>
        </div>
        <div>
          <button
            class="collapser"
            @click=${(e: Event) => {
              el.autorollersOpen = !el.autorollersOpen;
              el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
            }}>
            ${el.autorollersOpen
              ? html`<expand-less-icon-sk></expand-less-icon-sk>`
              : html`<expand-more-icon-sk></expand-more-icon-sk>`}
            AutoRollers
          </button>
          <collapse-sk>
            <autoroller-status-sk
              @rollers-update=${(e: CustomEvent) =>
                el.updateRollerLabels(e.detail)}></autoroller-status-sk>
          </collapse-sk>
        </div>
        <div>
          <button
            class="collapser"
            @click=${(e: Event) => {
              el.perfOpen = !el.perfOpen;
              el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
            }}>
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
            }}>
            ${el.goldOpen
              ? html`<expand-less-icon-sk></expand-less-icon-sk>`
              : html`<expand-more-icon-sk></expand-more-icon-sk>`}
            Gold
          </button>
          <collapse-sk>
            <gold-status-sk .repo=${el.repo}></gold-status-sk>
          </collapse-sk>
        </div>
        <div>
          <button
            class="collapser"
            @click=${(e: Event) => {
              el.bugsOpen = !el.bugsOpen;
              el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
            }}>
            ${el.bugsOpen
              ? html`<expand-less-icon-sk></expand-less-icon-sk>`
              : html`<expand-more-icon-sk></expand-more-icon-sk>`}
            Untriaged Bugs
          </button>
          <collapse-sk>
            <bugs-status-sk></bugs-status-sk>
          </collapse-sk>
        </div>
        <div>
          <button
            class="collapser"
            @click=${(e: Event) => {
              el.rotationsOpen = !el.rotationsOpen;
              el.toggle((<HTMLButtonElement>e.target).nextElementSibling);
            }}>
            ${el.rotationsOpen
              ? html`<expand-less-icon-sk></expand-less-icon-sk>`
              : html`<expand-more-icon-sk></expand-more-icon-sk>`}
            Gardeners
          </button>
          <collapse-sk>
            <rotations-sk></rotations-sk>
          </collapse-sk>
        </div>
      </aside>

      <main>
        <div class="main-layout">
          <div class="commits-panel">
            <commits-table-experimental-sk
              @repo-changed=${(e: CustomEvent) => el.updateRepo(e.detail)}
              @failed-tasks-changed=${(e: CustomEvent) => el.onFailedTasksChanged(e.detail.taskIds)}
              @task-hover=${(e: CustomEvent) =>
                el.onTaskHover(e.detail.taskId, e.detail.hovered)}></commits-table-experimental-sk>
          </div>
          <aside class="right-sidebar ${el.failureClassesOpen ? 'open' : 'closed'}">
            <div>
              <button
                class="collapser"
                @click=${() => {
                  el.failureClassesOpen = !el.failureClassesOpen;
                  el._render();
                }}>
                ${el.failureClassesOpen
                  ? html`<expand-less-icon-sk></expand-less-icon-sk>`
                  : html`<expand-more-icon-sk></expand-more-icon-sk>`}
                Known Failures
              </button>
              <collapse-sk ?closed=${!el.failureClassesOpen}>
                <failure-classes-sk
                  @highlight-tasks=${(e: CustomEvent) => el.onHighlightTasks(e.detail.taskIds)}
                  @select-failure-class=${(e: CustomEvent<ActiveFailureClass>) =>
                    el.onSelectFailureClass(e.detail)}></failure-classes-sk>
              </collapse-sk>
            </div>
          </aside>
        </div>
        <error-toast-sk></error-toast-sk>
      </main>

      <footer><error-toast-sk></error-toast-sk></footer>
    </app-sk>
  `;

  connectedCallback(): void {
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

  private updateRollerLabels(rolls: Array<AutorollerStatus>) {
    const branchSk = $$<BranchesSk>('branches-sk', this)!;
    branchSk.rolls = rolls;
    branchSk.repoUrl = repoUrl(this.repo.toLowerCase());
  }

  private onFailedTasksChanged(taskIds: Array<string>) {
    const fcSk = $$<FailureClassesSk>('failure-classes-sk', this);
    if (fcSk) {
      fcSk.taskIds = taskIds;
    }
  }

  private onTaskHover(taskId: string, hovered: boolean) {
    const fcSk = $$<FailureClassesSk>('failure-classes-sk', this);
    if (fcSk) {
      fcSk.setHoveredTask(taskId, hovered);
    }
  }

  private onHighlightTasks(taskIds: Array<string>) {
    const tableSk = $$<CommitsTableExperimentalSk>('commits-table-experimental-sk', this);
    if (tableSk) {
      tableSk.highlightTasks(taskIds);
    }
  }

  private onSelectFailureClass(fc: ActiveFailureClass) {
    const dialog = $$<DetailsDialogSk>('details-dialog-sk', this);
    if (dialog) {
      dialog.displayFailureClass(fc.failureClass, fc.taskIds.length);
    }
  }

  private updateTreeStatus(r: TreeStatus) {
    // We use css to style the header color based on state.
    this.setAttribute('state', r.status.general_state || '');
    $$<RotationsSk>('rotations-sk', this)!.rotations = r.rotations;
    // Set the favicon.
    const link = document.createElement('link');
    link.id = 'dynamicFavicon';
    link.rel = 'shortcut icon';
    switch (r.status.general_state) {
      case 'caution':
        link.href = '/dist/img/favicon-caution.ico';
        break;
      case 'closed':
        link.href = '/dist/img/favicon-closed.ico';
        break;
      default:
        link.href = '/dist/img/favicon-open.ico';
        break;
    }
    const head = document.getElementsByTagName('head')[0];
    const oldIcon = document.getElementById(link.id);
    if (oldIcon) {
      head.removeChild(oldIcon);
    }
    head.appendChild(link);
  }
}

define('status-experimental-sk', StatusExperimentalSk);
