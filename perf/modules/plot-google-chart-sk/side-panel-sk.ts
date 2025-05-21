/**
 * @module modules/side-panel-sk
 * @description <h2><code>side-panel-sk</code></h2>
 *
 * Element for showing the legend next to plot-google-chart-sk.
 * The side panel comes with a left bar that will close or open the
 * side panel.
 *
 * When there is only one trace in the dataframe, the legend will be empty.
 * When the legend is empty, the side panel will have no content.
 * It is recommended to hide this module if there is no content to show.
 *
 * This side panel can be adapted to also show the tooltip rather
 * than have the tooltip hover over the data point.
 */
import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { consume } from '@lit/context';

import { dataTableContext, DataTable } from '../dataframe/dataframe_context';
import { legendFormatter, getLegend, getLegendKeysTitle } from '../dataframe/traceset';
import { defaultColors } from '../common/plot-builder';

import '@material/web/button/outlined-button.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/icon/icon.js';

const chevronLeft = html`<svg
  xmlns="http://www.w3.org/2000/svg"
  viewBox="0 0 24 24"
  fill="currentColor">
  <path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z" />
</svg>`;

const chevronRight = html`<svg
  xmlns="http://www.w3.org/2000/svg"
  viewBox="0 0 24 24"
  fill="currentColor">
  <path d="M8.59 7.41L10 6l6 6-6 6-1.41-1.41L13.17 12z" />
</svg>`;

export interface SidePanelToggleEventDetails {
  open: boolean;
}

export interface SidePanelCheckboxClickDetails {
  readonly selected: boolean;
  readonly labels: string[];
}

@customElement('side-panel-sk')
export class SidePanelSk extends LitElement {
  static styles = css`
    :host {
      display: flex;
      height: 100%;
      border-radius: 8px;
      overflow: scroll; /* legend entries can be very long */
      box-shadow:
        0px 2px 1px -1px rgba(0, 0, 0, 0.2),
        0px 1px 1px 0px rgba(0, 0, 0, 0.14),
        0px 1px 3px 0px rgba(0, 0, 0, 0.12); /* Elevation shadow */
    }
    .show-hide-bar {
      width: 20px;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .show-hide-bar:hover {
      background-color: gray;
    }
    .label-key-title {
      color: #274878; /* Hex blue, which aligns with the graph title's color */
      padding-top: 10px;
      padding-left: 5px;
      padding-bottom: 5px;
      position: relative;
    }
    .delta-range {
      color: #274878; /* Hex blue, which aligns with the graph title's color */
      padding-top: 10px;
      padding-left: 5px;
      padding-bottom: 5px;
      position: relative;
    }
    .info.closed {
      display: none;
    }
    .select-all-checkbox {
      color: #274878; /* Hex blue, which aligns with the graph title's color */
      display: flex;
      padding-left: 5px;
    }
    ul {
      list-style: none; /* Remove default bullet points */
      padding-left: 5px;
      font-size: 15px;
      margin-block-start: 3px;
    }
    .default {
      border-style: none;
    }
    .highlight {
      border-style: solid;
    }
  `;

  // Manages the state of the side panel as collapsed or open.
  @property({ reflect: true, type: Boolean })
  opened = true;

  // Track state if the legend has been populated initially.
  @property({ reflect: true, type: Boolean })
  private legendLoaded = false;

  @property({ reflect: true, type: Set })
  private checkedColList = new Set<string>();

  @consume({ context: dataTableContext, subscribe: true })
  @property({ attribute: false })
  private data?: DataTable;

  @property({ reflect: true })
  deltaRaw: number | null = null;

  @property({ reflect: true })
  deltaPercentage: number | null = null;

  // Display delta-range if the value is true, vice versa.
  // Default is false, which means hide delta-range information
  @property({ reflect: true })
  showDelta = false;

  /**
   * A map that maps legend to label.
   * The legend is the legend of the trace,
   * the label is the label of the column in the dataframe.
   */
  @property({ attribute: false, reflect: true })
  private legendToLabelMap: { [key: string]: string } = {};

  /**
   * A map that maps legend to label.
   * The legend is the legend of the trace,
   * The label is the label of the column in the dataframe.
   */
  @property({ attribute: false, reflect: true })
  private labelToLegendMap: { [key: string]: string } = {};

  @property({ attribute: false, reflect: true })
  private legendKeysFormat = '';

  @property({ attribute: false, reflect: true })
  private highlightedTraceIndices: number[] = [];

  @property({ reflect: true })
  legendListCache: string[] = [];

  constructor() {
    super();
  }

  render() {
    if (this.legendListCache.length === 0) {
      this.legendListCache = this.getLegend();
    }
    return html`
      <div
        class="show-hide-bar"
        @click=${this.toggleSidePanel}
        title=${this.opened ? 'Close panel' : 'Open panel'}>
        <md-icon>${this.opened ? chevronRight : chevronLeft}</md-icon>
      </div>
      <div class="info ${classMap({ closed: !this.opened })}">
        <div class="delta-range" ?hidden=${!this.showDelta}> Delta: ${this.deltaRaw}<br/>
        Percentage: ${this.deltaPercentage}</div>
        <div class="label-key-title">
          <span>${this.legendKeysFormat}</span>
        </div>
        <div class="select-all-checkbox">
          <label>
            <input type="checkbox" id="header-checkbox"
            .defaultChecked=${true} .checked=${true}
            @click=${this.toggleAllCheckboxes}> Select all</input>
          </label>
        </div>
        <div id="rows">
          <ul>
          ${this.legendListCache.map((item, index) => {
            if (this.legendLoaded) {
              const checkbox = this.renderRoot.querySelector(`#id-${index}`) as HTMLInputElement;
              checkbox.checked = this.checkedColList.has(item);
              if (this.checkedColList.size === 1 && this.checkedColList.has(item)) {
                checkbox.disabled = true;
              } else {
                checkbox.disabled = false;
              }
            } else {
              if (!this.checkedColList.has(item)) {
                this.checkedColList.add(item);
              }
            }
            const handleCheck = (e: MouseEvent) => {
              const checkEvent = e.target! as HTMLInputElement;
              if (checkEvent) {
                this.setCheckbox(checkEvent.checked, index);
              }
              // Set the header box as checked based on all boxes being checked.
              const headerCheckbox = this.renderRoot.querySelector(
                `#header-checkbox`
              ) as HTMLInputElement;
              if (this.legendListCache.length === this.checkedColList.size) {
                headerCheckbox.checked = true;
              } else {
                headerCheckbox.checked = false;
              }
            };
            let labelClass = 'default';
            if (this.highlightedTraceIndices.includes(index)) {
              labelClass = 'highlight';
            }
            return html`
              <li style="color: ${defaultColors[index % defaultColors.length]}">
                <label class="${labelClass}">
                  <input
                    type="checkbox"
                    id="id-${index}"
                    @click=${handleCheck}
                    ?checked=${this.checkedColList.has(item)}
                    title="Select/Unselect this value from the graph" />
                  ${item}
                </label>
              </li>
            `;
          })}
          </ul>
        </div>
      </div>
    `;
  }

  /**
   * Sets the checkbox state for the given trace id.
   * @param checked Whether the boxed is checked or not.
   * @param traceId The trace id.
   */
  public SetCheckboxForTrace(checked: boolean, traceId: string) {
    const items = this.legendListCache;
    // The legend values are in the form of a/b/c/d while traceId is
    // ,k1=a,k2=b,k3=c,k4=d. The labelToLegendMap contains this
    // mapping so we first get the value on the legend to be updated.
    const legendVal = this.labelToLegendMap[traceId];
    let index = -1;
    // Now let's find the index of the value to be updated.
    for (let i = 0; i < items.length; i++) {
      if (items[i] === legendVal) {
        index = i;
        break;
      }
    }
    // Set the checkbox state on the index if item exists.
    if (index > -1) {
      this.setCheckbox(checked, index);
    }
  }

  /**
   * Sets all the checkboxes in the panel to the given checked state.
   * @param checked The state of checkboxes.
   */
  public SetAllBoxes(checked: boolean) {
    for (let index = 0; index < this.legendListCache.length; index++) {
      this.setCheckbox(checked, index);
    }
  }

  /**
   * Highlight the traces on the given indices on the panel.
   * @param traceIndices Indices of the traces to highlight.
   */
  public HighlightTraces(traceIndices: number[]) {
    this.highlightedTraceIndices = traceIndices;
    this.render();
  }

  /**
   * Sets the checkbox and tracks checked items in a list.
   */
  private setCheckbox(checked: boolean, index: number) {
    const item = this.legendListCache[index];
    if (checked) {
      if (!this.checkedColList.has(item)) {
        this.checkedColList.add(item);
        this.checkboxDispatchHandler(checked, [item]);
      }
    } else {
      if (this.checkedColList.has(item) && this.checkedColList.size > 1) {
        this.checkedColList.delete(item);
        this.checkboxDispatchHandler(checked, [item]);
      }
    }
    this.render();
  }

  private toggleAllCheckboxes() {
    const headerCheckbox = this.renderRoot.querySelector(`#header-checkbox`) as HTMLInputElement;
    for (let index = 0; index < this.legendListCache.length; index++) {
      this.setCheckbox(headerCheckbox.checked, index);
    }
  }

  private toggleSidePanel() {
    this.opened = !this.opened;
    this.dispatchEvent(
      new CustomEvent<SidePanelToggleEventDetails>('side-panel-toggle', {
        bubbles: true,
        composed: true,
        detail: {
          open: this.opened,
        },
      })
    );
  }

  private getLegend() {
    if (this.data) {
      const getLegendData = getLegend(this.data);
      if (getLegendData.length === 0) {
        return [];
      }
      // The legend is a list of strings that are the legend values.
      const legendList = legendFormatter(getLegendData);
      this.legendKeysFormat = getLegendKeysTitle(getLegendData[0]);
      const numCols = this.data!.getNumberOfColumns();
      // The first two columns of the data table for the commit number/ timestamp x axis- options.
      // It converted n-2 labels to legend format and stored in the legend list.
      for (let i = 2; i < numCols; i++) {
        const k = this.data!.getColumnLabel(i);
        this.legendToLabelMap[legendList[i - 2]] = k;
        this.labelToLegendMap[k] = legendList[i - 2];
      }
      return legendList;
    }
    return [];
  }

  checkboxDispatchHandler(isSelected: boolean, legendList: string[]): void {
    const labels: string[] = [];
    this.legendLoaded = true;
    legendList.forEach((legend) => {
      labels.push(this.legendToLabelMap[legend] ? this.legendToLabelMap[legend] : '');
    });
    const detail: SidePanelCheckboxClickDetails = {
      selected: isSelected,
      labels: labels,
    };
    this.dispatchEvent(
      new CustomEvent('side-panel-selected-trace-change', {
        detail,
        bubbles: true,
      })
    );
  }
}

declare global {
  interface GlobalEventHandlersEventMap {
    'side-panel-toggle': CustomEvent<SidePanelToggleEventDetails>;
    'side-panel-selected-trace-change': CustomEvent<SidePanelCheckboxClickDetails>;
  }
}
