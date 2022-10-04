/**
 * @module modules/pivot-query-sk
 * @description <h2><code>pivot-query-sk</code></h2>
 *
 * @evt pivot-changed - Emitted every time the control is changed by the user.
 * See PivotQueryChangedEventDetail.
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { MultiSelectSkSelectionChangedEventDetail } from 'elements-sk/multi-select-sk/multi-select-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParamSet, pivot } from '../json';
import 'elements-sk/multi-select-sk';
import 'elements-sk/select-sk';
import 'elements-sk/styles/select';
import { operationDescriptions, validatePivotRequest } from '../pivotutil';

const sortedOps = Object.keys(operationDescriptions).sort() as pivot.Operation[];

/** CustomEvent details sent when the control is changed by the user. */
export type PivotQueryChangedEventDetail = pivot.Request | null;

/** The name of the event we emit. */
export const PivotQueryChangedEventName = 'pivot-changed';

/**
 * Custom element that allows editing a pivot.Request.
 *
 * A ParamSet is also needed to list the allowable choices
 * for the pivot.Request.group_by.
 */
export class PivotQuerySk extends ElementSk {
  private _paramset: ParamSet = {};

  private _pivotRequest: pivot.Request | null = null;

  constructor() {
    super(PivotQuerySk.template);
  }

  private static template = (ele: PivotQuerySk) => html`
  <label>
    <p>Which keys should traces be grouped by:</p>
    <multi-select-sk id=group_by @selection-changed=${ele.groupByChanged}>
      ${ele.groupByOptions()}
    </multi-select-sk>
  </label>

  <label>
    <p>What operation should be applied:</p>
    <select @change=${ele.operationChanged}>
      ${ele.operationOptions()}
    </select>
  </label>

  <label>
    <p>Optional: Choose summary statistics to calculate for each group:</p>
    <multi-select-sk id=summary @selection-changed=${ele.summaryChanged}>
      ${ele.summaryOptions()}
    </multi-select-sk>
  </label>
`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private createDefaultPivotRequestIfNull(): void {
    if (this._pivotRequest === null) {
      this._pivotRequest = {
        group_by: [],
        operation: 'avg',
        summary: [],
      };
    }
  }

  private allGroupByOptions(): string[] {
    const psKeys = Object.keys(this._paramset);
    const selectedKeys = this._pivotRequest?.group_by || [];
    // An empty ParamSet could wipe out the users selection, which would be
    // unfortunate, so we always display the values in the current pivotRequest
    // so they don't get lost.
    //
    // Do this by concatenating psKeys and selectedKeys and then removing
    // duplicates.
    let last = '';
    return psKeys.concat(selectedKeys).sort().filter((key: string) => {
      if (key === last) {
        return false;
      }
      last = key;
      return true;
    });
  }

  private groupByOptions(): TemplateResult[] {
    const selectedKeys = this._pivotRequest?.group_by || [];
    const allOptions = this.allGroupByOptions();
    return allOptions.map((key: string): TemplateResult => html`<div ?selected=${selectedKeys.includes(key)}>${key}</div>`);
  }

  private operationOptions(): TemplateResult[] {
    return sortedOps.map((key: pivot.Operation): TemplateResult => html`<option value="${key}" .selected=${this._pivotRequest?.operation === key}>${operationDescriptions[key]}</option>`);
  }

  private summaryOptions(): TemplateResult[] {
    const selections = this._pivotRequest?.summary || [];
    return sortedOps.map((key: pivot.Operation): TemplateResult => html`<div ?selected=${selections.includes(key)}>${operationDescriptions[key]}</div>`);
  }

  private groupByChanged(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>): void {
    this.createDefaultPivotRequestIfNull();
    const allOptions = this.allGroupByOptions();
    this._pivotRequest!.group_by = e.detail.selection.map((index: number) => allOptions[index]);
    this.emitChangeEvent();
  }

  private operationChanged(e: Event & { target: HTMLSelectElement }): void {
    this.createDefaultPivotRequestIfNull();
    this._pivotRequest!.operation = sortedOps[e.target.selectedIndex];
    this.emitChangeEvent();
  }

  private summaryChanged(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>): void {
    this.createDefaultPivotRequestIfNull();
    this._pivotRequest!.summary = e.detail.selection.map((index: number) => sortedOps[index]);
    this.emitChangeEvent();
  }

  private emitChangeEvent(): void {
    this.dispatchEvent(new CustomEvent<PivotQueryChangedEventDetail>(PivotQueryChangedEventName, {
      detail: this.pivotRequest,
      bubbles: true,
    }));
  }

  /** Returns null if the pivot.Request isn't valid. */
  public get pivotRequest(): pivot.Request | null {
    if (validatePivotRequest(this._pivotRequest)) {
      return null;
    }
    return this._pivotRequest;
  }

  public set pivotRequest(v: pivot.Request | null) {
    this._pivotRequest = v;
    this._render();
  }

  public get paramset(): ParamSet {
    return this._paramset;
  }

  public set paramset(v: ParamSet) {
    this._paramset = v;
    this._render();
  }
}

define('pivot-query-sk', PivotQuerySk);
