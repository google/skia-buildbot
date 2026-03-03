/**
 * @module modules/pivot-query-sk
 * @description <h2><code>pivot-query-sk</code></h2>
 *
 * @evt pivot-changed - Emitted every time the control is changed by the user.
 * See PivotQueryChangedEventDetail.
 */
import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { MultiSelectSkSelectionChangedEventDetail } from '../../../elements-sk/modules/multi-select-sk/multi-select-sk';
import { ParamSet, pivot } from '../json';
import '../../../elements-sk/modules/multi-select-sk';
import '../../../elements-sk/modules/select-sk';
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
@customElement('pivot-query-sk')
export class PivotQuerySk extends LitElement {
  private static nextUniqueId = 0;

  private readonly uniqueId = `${PivotQuerySk.nextUniqueId++}`;

  @property({ attribute: false })
  paramset: ParamSet = ParamSet({});

  @state()
  private _pivotRequest: pivot.Request | null = null;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <p id="group_by_label-${this.uniqueId}">Which keys should traces be grouped by:</p>
      <multi-select-sk
        id="group_by-${this.uniqueId}"
        aria-labelledby="group_by_label-${this.uniqueId}"
        @selection-changed=${this.groupByChanged}>
        ${this.groupByOptions()}
      </multi-select-sk>

      <label for="operation-${this.uniqueId}">
        <p>What operation should be applied:</p>
        <select id="operation-${this.uniqueId}" @change=${this.operationChanged}>
          ${this.operationOptions()}
        </select>
      </label>

      <p id="summary_label-${this.uniqueId}">
        Optional: Choose summary statistics to calculate for each group:
      </p>
      <multi-select-sk
        id="summary-${this.uniqueId}"
        aria-labelledby="summary_label-${this.uniqueId}"
        @selection-changed=${this.summaryChanged}>
        ${this.summaryOptions()}
      </multi-select-sk>
    `;
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
    const psKeys = Object.keys(this.paramset);
    const selectedKeys = this._pivotRequest?.group_by || [];
    // An empty ParamSet could wipe out the users selection, which would be
    // unfortunate, so we always display the values in the current pivotRequest
    // so they don't get lost.
    //
    // Do this by concatenating psKeys and selectedKeys and then removing
    // duplicates.
    let last = '';
    return psKeys
      .concat(selectedKeys)
      .sort()
      .filter((key: string) => {
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
    return allOptions.map(
      (key: string): TemplateResult =>
        html`<div ?selected=${selectedKeys.includes(key)}>${key}</div>`
    );
  }

  private operationOptions(): TemplateResult[] {
    return sortedOps.map(
      (key: pivot.Operation): TemplateResult =>
        html`<option value="${key}" .selected=${this._pivotRequest?.operation === key}>
          ${operationDescriptions[key]}
        </option>`
    );
  }

  private summaryOptions(): TemplateResult[] {
    const selections = this._pivotRequest?.summary || [];
    return sortedOps.map(
      (key: pivot.Operation): TemplateResult =>
        html`<div ?selected=${selections.includes(key)}>${operationDescriptions[key]}</div>`
    );
  }

  private groupByChanged(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>): void {
    this.createDefaultPivotRequestIfNull();
    const allOptions = this.allGroupByOptions();
    this._pivotRequest = {
      ...this._pivotRequest!,
      group_by: e.detail.selection.map((index: number) => allOptions[index]),
    };
    this.emitChangeEvent();
  }

  private operationChanged(e: Event & { target: HTMLSelectElement }): void {
    this.createDefaultPivotRequestIfNull();
    this._pivotRequest = {
      ...this._pivotRequest!,
      operation: sortedOps[e.target.selectedIndex],
    };
    this.emitChangeEvent();
  }

  private summaryChanged(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>): void {
    this.createDefaultPivotRequestIfNull();
    this._pivotRequest = {
      ...this._pivotRequest!,
      summary: e.detail.selection.map((index: number) => sortedOps[index]),
    };
    this.emitChangeEvent();
  }

  private emitChangeEvent(): void {
    this.dispatchEvent(
      new CustomEvent<PivotQueryChangedEventDetail>(PivotQueryChangedEventName, {
        detail: this.pivotRequest,
        bubbles: true,
      })
    );
  }

  /** Returns null if the pivot.Request isn't valid. */
  public get pivotRequest(): pivot.Request | null {
    if (validatePivotRequest(this._pivotRequest)) {
      return null;
    }
    return this._pivotRequest;
  }

  @property({ attribute: false })
  public set pivotRequest(v: pivot.Request | null) {
    this._pivotRequest = v;
  }
}
