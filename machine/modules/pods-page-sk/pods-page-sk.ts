/**
 * @module modules/pods-page-sk
 * @description <h2><code>pods-page-sk</code></h2>
 *
 * A readout of currently extant switch-pods
 *
 * @attr waiting - If present then display the waiting cursor.
 */
import { html, TemplateResult } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk/index';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { Pod } from '../json';
import { FilterArray } from '../filter-array';

const rows = (ele: PodsPageSk): TemplateResult[] => ele.filteredPods().map(
  (pod) => html`
      <tr>
        <td>${pod.Name}</td>
        <td>${pod.LastUpdated}</td>
      </tr>
    `
);

const template = (ele: PodsPageSk): TemplateResult => html`
  <header>
    <auto-refresh-sk @refresh-page=${ele.update}></auto-refresh-sk>
    <span id=header-rhs>
      <input id=filter-input type="text" placeholder="Filter">
      <theme-chooser-sk title="Toggle between light and dark mode."></theme-chooser-sk>
    </span>
  </header>
  <main>
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Last Updated</th>
        </tr>
      </thead>
      <tbody>
        ${rows(ele)}
      </tbody>
    </table>
  </main>
  <note-editor-sk></note-editor-sk>
  <error-toast-sk></error-toast-sk>
`;

export class PodsPageSk extends ElementSk {
  pods: Pod[] = [];

  private filterArray: FilterArray | null = null;

  constructor() {
    super(template);
  }

  // TODO: Genericize and move to FilterArray.
  filteredPods(): Pod[] {
    if (this.filterArray === null) {
      return this.pods;
    }
    return this.filterArray.matchingIndices().map((index) => this.pods[index]);
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    const filterInput = $$<HTMLInputElement>('#filter-input', this)!;
    this.filterArray = new FilterArray(filterInput, () => this._render());
    await this.update();
  }

  async update(changeCursor = false): Promise<void> {
    if (changeCursor) {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch('/_/pods');
      const json = await jsonOrThrow(resp);
      if (changeCursor) {
        this.removeAttribute('waiting');
      }
      this.pods = json;
      this.filterArray!.updateArray(json);
      this._render();
    } catch (error) {
      this.onError(error);
    }
  }

  private onError(msg: { message: string; }) {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }
};

define('pods-page-sk', PodsPageSk);
