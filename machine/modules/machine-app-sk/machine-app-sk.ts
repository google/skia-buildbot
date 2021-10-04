/**
 * @module modules/machine-app-sk
 * @description <h2><code>machine-app-sk</code></h2>
 *
 * UI wrapper for machineserver readouts which sticks them in tabs and has a
 * shared toolbar
 */
import { html } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import 'elements-sk/error-toast-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';
import { TabSelectedSkEventDetail } from 'elements-sk/tabs-sk/tabs-sk';
import { TabsPanelSk } from 'elements-sk/tabs-panel-sk/tabs-panel-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { MachineServerSk } from '../machine-server-sk';
import { PodsPageSk } from '../pods-page-sk';
import { MeetingPointsPageSk } from '../meeting-points-page-sk';
import { WaitCursor } from '../list-page-sk';
import '../machine-server-sk';
import '../pods-page-sk';
import '../meeting-points-page-sk';

/**
 * Any of the readouts that are on our tabs. This could turn into an interface
 * if we diversify beyond LiveTableSk subclasses.
 */
type AnyLiveTable = MachineServerSk | MeetingPointsPageSk | PodsPageSk;

export class MachineAppSk extends ElementSk {
  private _inputElement: HTMLInputElement | null = null;

  private panel: TabsPanelSk | null = null;

  protected _template = (ele: MachineAppSk) => html`
    <header>
      <auto-refresh-sk @refresh-page=${ele.update}></auto-refresh-sk>
      <span id=header-rhs>
        <input id="filter-input" @input=${ele.filterChanged} type="text" placeholder="Filter">
        <theme-chooser-sk title="Toggle between light and dark mode."></theme-chooser-sk>
      </span>
    </header>
    <main>
      <tabs-sk @tab-selected-sk=${ele.tabSwitched}>
        <button class="selected">Machines</button>
        <button>Pods</button>
        <button>Meeting Points</button>
      </tabs-sk>
      <tabs-panel-sk>
        <machine-server-sk></machine-server-sk>
        <pods-page-sk></pods-page-sk>
        <meeting-points-page-sk></meeting-points-page-sk>
      </tabs-panel-sk>
    </main>
    <error-toast-sk></error-toast-sk>
  `;

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    this._inputElement = $$<HTMLInputElement>('#filter-input', this)!;
    this.panel = $$<TabsPanelSk>('tabs-panel-sk', this);
    this.update(WaitCursor.SHOW);
  }

  /** Tell the active table to filter itself. */
  filterChanged() {
    this.selectedTable().filterChanged(this._inputElement!.value);
  }

  /** Tell the active table to fetch new data and redraw itself. */
  update(waitCursorPolicy = WaitCursor.DO_NOT_SHOW) {
    this.selectedTable().update(waitCursorPolicy);
  }

  /** Keep the UI stable as we change tabs. */
  private tabSwitched(_: CustomEvent<TabSelectedSkEventDetail>) {
    // Bring the filtration on the tab up to date, in case it changed while the
    // tab was hidden:
    this.filterChanged();

    // I feel like updating is more useful than maintaining stale data, but
    // experience will tell. Show wait cursor so the user knows we're updating.
    this.update(WaitCursor.SHOW);
  }

  /** Return the table from the tab that is in the foreground. */
  private selectedTable(): AnyLiveTable {
    const n = this.panel!.selected;
    return this.panel!.children[n] as AnyLiveTable;
  }
}

define('machine-app-sk', MachineAppSk);
