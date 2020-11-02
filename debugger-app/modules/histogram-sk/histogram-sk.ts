/**
 * @module modules/histogram-sk
 *
 * @description A visual representation of the command name counts totalled by
 * commands-sk when a frame is loaded, and a controller for the set of included
 * command names in the filter (also in commands-sk).
 *
 * It's not an indepentent model-view-controller, just a view-controller for a
 * model in commands-sk. The main reason it's a seperate module is that it's
 * visually self-contained in different element elsewhere on the page, and as
 * far as I know commandsSk cannot export two different elements.
 *
 * Due to the functionality depending on the integration with commands-sk,
 * this module is tested in debugger-page-sk_test.ts.
 *
 * @evt toggle-command-inclusion: Emitted when a row is clicked, indicating it's
 * inclusion in the command filter should be toggled.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import {
  CommandsSk, CommandsSkHistogramEventDetail, HistogramEntry,
} from '../commands-sk/commands-sk';

export interface HistogramSkToggleEventDetail {
  // the name of a command to toggle in the filter
  // Clicking rows of the histogram is an alternate way to add or remove command names
  // from the command filter. The filter is managed by commands-sk
  name: string,
}

export class HistogramSk extends ElementSk {
  private static template = (ele: HistogramSk) =>
    html`
<details title="A table of the number of occurrences of each command." open>
  <summary>Histogram</summary>
  <table>
    <tr>
      <td title="Occurrences of command in current frame (or single frame skp file).">frame</td>
      <td title="Occurrences of command within the current range filter.">range</td>
      <td>name</td>
    </tr>
    ${ ele._hist.map((item: HistogramEntry) => HistogramSk.rowTemplate(ele, item)) }
    <tr><td class=countCol>${ele._total()}</td><td><b>Total</b></td></tr>
  </table>
</details>`;

  private static rowTemplate = (ele: HistogramSk, item: HistogramEntry) =>
    html`
<tr @click=${()=>{ele._toggle(item.name)}} id="hist-row-${item.name.toLowerCase()}"
    class="${ele._incl.has(item.name.toLowerCase())? '' : 'pinkBackground'}">
  <td class=countCol>${item.countInFrame}</td>
  <td class=countCol>${item.countInRange}</td>
  <td>${item.name}</td>
</tr>`;

  // counts of command occurances
  private _hist: HistogramEntry[] = [];
  // commands which the filter includes
  private _incl = new Set<string>();

  private _total(): number {
    let total = 0;
    for (const item of this._hist) {
      total += item.countInRange;
    }
    return total;
  }

  constructor() {
    super(HistogramSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    document.addEventListener('histogram-update', (e) => {
      const detail = (e as CustomEvent<CommandsSkHistogramEventDetail>).detail;
      // event may update one or both items
      if (detail.hist) {
        this._hist = detail.hist;
      }
      if (detail.included) {
        this._incl = detail.included;
      }
      this._render();
    });
  }

  // Toggle the given item in or out of the included set in commands-sk
  private _toggle(name: string) {
    const lowerName = name.toLowerCase();
    // short cut to change our own appearance
    if (this._incl.has(lowerName)) {
      this._incl.delete(lowerName);
    } else {
      this._incl.add(lowerName);
    }
    this._render();

    // but make sure to tell the module that actually owns this model
    this.dispatchEvent(
    new CustomEvent<HistogramSkToggleEventDetail>(
      'toggle-command-inclusion', {
        detail: {name: lowerName},
        bubbles: true,
      }));
  }
};

define('histogram-sk', HistogramSk);
