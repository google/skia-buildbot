/**
 * @module modules/filter-sk
 * @description A multi-modal filter for the commands within a frame of a SKP file.
 * One can filter commands based on a numerical range, and either by an inclusive
 * or exclusive list of command names. The range filter is always applied first.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class FilterSk extends ElementSk {
  private static template = (ele: FilterSk) =>
    html`
    <div class="horizontal-flex">
      <label title="Filter command names (Single leading ! negates entire filter).
Command types can also be filted by clicking on their names in the histogram">Filter</label>
      <input @change=${ele._textFilter} value="!DrawAnnotation"></input>&nbsp;
      <label>Range</label>
      <input @change=${ele._rangeFilter} class=range-input value="0"></input>
      <b>:</b>
      <input @change=${ele._rangeFilter} class=range-input value="100"></input>
      <button id=clear @click=${ele._clearFilter}>Clear</button>
    </div>`;


  // This module is currently a visual placeholder
  // TODO(nifong): Do all the functional parts once command list is in place
  // Consider merging this module with the command list or making this a well-isolated
  // submodule of it.
  // This is expected to have extensive communication with the histogram module, can those be
  // one class that exports two html tags? Or maybe one tag that has two completely different
  // visual appearances?

  constructor() {
    super(FilterSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private _textFilter(e: Event) {
    console.log(e);
  }

  private _rangeFilter(e: Event) {
    console.log(e);
  }

  private _clearFilter() {
    console.log('clearfilter');
  }
};

define('filter-sk', FilterSk);
