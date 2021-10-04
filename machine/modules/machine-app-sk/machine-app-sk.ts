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
import { MachinesTableSk } from '../machines-table-sk';
import { PodsTableSk } from '../pods-table-sk';
import { MeetingPointsTableSk } from '../meeting-points-table-sk';
import { WaitCursor } from '../live-table-sk';
import '../machines-table-sk';
import '../pods-table-sk';
import '../meeting-points-table-sk';

/**
 * Any of the readouts that are on our tabs. This could turn into an interface
 * if we diversify beyond LiveTableSk subclasses.
 */
type AnyLiveTable = MachinesTableSk | MeetingPointsTableSk | PodsTableSk;

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
        <machines-table-sk></machines-table-sk>
        <pods-table-sk></pods-table-sk>
        <meeting-points-table-sk></meeting-points-table-sk>
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
