/**
 * @module modules/machine-app-sk
 * @description <h2><code>machine-app-sk</code></h2>
 *
 * UI wrapper for machineserver readouts which sticks them in tabs and has a
 * shared toolbar
 */
import { html, TemplateResult } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import 'elements-sk/error-toast-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { TabSelectedSkEventDetail, TabsSk } from 'elements-sk/tabs-sk/tabs-sk';
import { TabsPanelSk } from 'elements-sk/tabs-panel-sk/tabs-panel-sk';
import { HintableObject } from 'common-sk/modules/hintable';
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

/** The State of our page that gets reflected into the URL. */
class State {
  /** Which tab is shown. */
  tab: number = 0;

  /** The value of the search input element. */
  search: string = '';
}

export class MachineAppSk extends ElementSk {
  private _inputElement: HTMLInputElement | null = null;

  /** The state to reflect to the URL. */
  private state: State = new State();

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private panel: TabsPanelSk | null = null;

  private tabs: TabsSk | null = null;

  protected _template = (ele: MachineAppSk): TemplateResult => html`
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

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._inputElement = $$<HTMLInputElement>('#filter-input', this)!;
    this.panel = $$<TabsPanelSk>('tabs-panel-sk', this);
    this.tabs = $$<TabsSk>('tabs-sk', this);
    this.update(WaitCursor.SHOW);
    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (hintableState) => {
        const state = hintableState as unknown as State;
        this.state = state;
        this.setSelectedTableIndex(this.state.tab);
        this.update();

        this._inputElement!.value = this.state.search;
        this.propagateFilterChange();
        this._render();
      },
    );
  }

  /** Handle input changed event for the filter element. */
  private filterChanged(): void {
    this.propagateFilterChange();
    this.state.search = this._inputElement!.value;
    this.stateHasChanged();
  }

  /** Tell the active table to filter itself. */
  private propagateFilterChange(): void {
    this.selectedTable().filterChanged(this._inputElement!.value);
  }

  /** Tell the active table to fetch new data and redraw itself. */
  private update(waitCursorPolicy = WaitCursor.DO_NOT_SHOW): void {
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

    this.state.tab = this.getSelectedTableIndex();
    this.stateHasChanged();
  }

  /** Return the table from the tab that is in the foreground. */
  private selectedTable(): AnyLiveTable {
    const n = this.panel!.selected;
    return this.panel!.children[n] as AnyLiveTable;
  }

  /** Returns the index of the tab panel being displayed. */
  private getSelectedTableIndex(): number {
    return this.panel!.selected;
  }

  /** Set the tab panel being displayed based on it's index. */
  private setSelectedTableIndex(index: number) {
    this.tabs!.select(index);
  }
}

define('machine-app-sk', MachineAppSk);
