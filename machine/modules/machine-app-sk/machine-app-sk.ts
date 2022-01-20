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
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { MachinesTableSk } from '../machines-table-sk';
import { WaitCursor } from '../live-table-sk';
import '../machines-table-sk';
import '../pods-table-sk';
import '../meeting-points-table-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

/** The State of our page that gets reflected into the URL. */
class State {
  /** The value of the search input element. */
  search: string = '';
}

export class MachineAppSk extends ElementSk {
  private _inputElement: HTMLInputElement | null = null;

  /** The state to reflect to the URL. */
  private state: State = new State();

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private machinesTable: MachinesTableSk | null = null;

  protected _template = (ele: MachineAppSk): TemplateResult => html`
    <app-sk>
      <header>
        <span>
          <auto-refresh-sk @refresh-page=${ele.update}></auto-refresh-sk>
          <h1><a href="/">Machines</a></h1>
        </span>
        <span id=header-rhs>
          <input id="filter-input" @input=${ele.filterChanged} type="text" placeholder="Filter">
          <theme-chooser-sk title="Toggle between light and dark mode."></theme-chooser-sk>
        </span>
      </header>
      <main>
        <machines-table-sk></machines-table-sk>
      </main>
      <error-toast-sk></error-toast-sk>
    </app-sk>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._inputElement = $$<HTMLInputElement>('#filter-input', this)!;
    this.machinesTable = $$<MachinesTableSk>('machines-table-sk', this);
    this.update(WaitCursor.SHOW);
    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (hintableState) => {
        const state = hintableState as unknown as State;
        this.state = state;
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
    this.machinesTable?.filterChanged(this._inputElement!.value);
  }

  /** Tell the active table to fetch new data and redraw itself. */
  private update(waitCursorPolicy = WaitCursor.DO_NOT_SHOW): void {
    this.machinesTable?.update(waitCursorPolicy);
  }
}

define('machine-app-sk', MachineAppSk);
