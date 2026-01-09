/**
 * @module modules/side-panel-sk
 * @description <h2><code>side-panel-sk</code></h2>
 *
 * Element for showing the legend next to plot-google-chart-sk.
 * The side panel comes with a left bar that will close or open the
 * side panel. This component is used by plot-google-chart-sk.
 *
 * When there is only one trace in the dataframe, the legend will be empty.
 * When the legend is empty, the side panel will have no content.
 * It is recommended to hide this module if there is no content to show.
 *
 * This side panel can be adapted to also show the tooltip rather
 * than have the tooltip hover over the data point.
 */
import { LitElement, html, css, PropertyValues } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { classMap } from 'lit/directives/class-map.js';
import { consume } from '@lit/context';

import { dataTableContext, DataTable, traceColorMapContext } from '../dataframe/dataframe_context';
import { getLegend } from '../dataframe/traceset';

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

const unwantedKeys = new Set(['master', 'improvement_direction', 'unit']);

export interface SidePanelToggleEventDetails {
  open: boolean;
}

export interface SidePanelCheckboxClickDetails {
  readonly selected: boolean;
  readonly labels: string[];
}

interface LegendItem {
  displayName: string;
  // The original trace labels from the DataTable.
  labels: string[];
  color: string;
  checked: boolean;
  highlighted: boolean;
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

  @property({ attribute: false })
  private legendItems: LegendItem[] = [];

  @consume({ context: traceColorMapContext, subscribe: true })
  @property({ attribute: false })
  private traceColorMap = new Map<string, string>();

  @property({ attribute: false, reflect: true })
  private legendKeysFormat = '';

  constructor() {
    super();
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (changedProperties.has('traceColorMap') || (changedProperties.has('data') && this.data)) {
      this.updateLegendItems();
    }
  }

  render() {
    const allItemsChecked = this.legendItems.length > 0 && this.legendItems.every((i) => i.checked);
    return html`
      <div
        class="show-hide-bar"
        @click=${this.toggleSidePanel}
        title=${this.opened ? 'Close panel' : 'Open panel'}>
        <md-icon>${this.opened ? chevronRight : chevronLeft}</md-icon>
      </div>
      <div class="info ${classMap({ closed: !this.opened })}">
        <div class="delta-range" ?hidden=${!this.showDelta}>
          Delta: ${this.deltaRaw}<br />
          Percentage: ${this.deltaPercentage}
        </div>
        <div class="label-key-title">
          <span>${this.legendKeysFormat}</span>
        </div>
        <div class="select-all-checkbox">
          <label>
            <input
              type="checkbox"
              id="header-checkbox"
              .checked=${allItemsChecked}
              @click=${this.toggleAllCheckboxes} />
            Select all
          </label>
        </div>
        <div id="rows">
          <ul>
            ${this.legendItems.map((item, index) => {
              const handleCheck = (e: MouseEvent) => {
                const checkEvent = e.target! as HTMLInputElement;
                this.toggleItemChecked(item, checkEvent.checked);
              };

              const checkedCount = this.legendItems.filter((i) => i.checked).length;
              const isLastChecked = item.checked && checkedCount === 1;

              return html`
                <li style="color: ${item.color}">
                  <label class="${item.highlighted ? 'highlight' : 'default'}">
                    <input
                      type="checkbox"
                      id="id-${index}"
                      @click=${handleCheck}
                      .checked=${item.checked}
                      ?disabled=${isLastChecked}
                      title=${isLastChecked
                        ? 'At least one trace must be selected.'
                        : 'Select/Unselect this value from the graph'} />
                    ${item.displayName}
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
   * Toggles the checked state of a legend item and dispatches an event.
   *
   * @param item - The legend item to toggle.
   * @param isChecked - The new checked state.
   */
  private toggleItemChecked(item: LegendItem, isChecked: boolean) {
    const checkedCount = this.legendItems.filter((i) => i.checked).length;
    if (!isChecked && checkedCount <= 1) {
      // Don't allow unchecking the last item. Re-render to revert checkbox state.
      this.requestUpdate();
      return;
    }

    item.checked = isChecked;
    this.checkboxDispatchHandler(isChecked, item.labels);

    // Update header checkbox
    const headerCheckbox = this.renderRoot.querySelector('#header-checkbox') as HTMLInputElement;
    if (headerCheckbox) {
      headerCheckbox.checked = this.legendItems.every((i) => i.checked);
    }
    this.requestUpdate();
  }

  /**
   * Sets the checkbox state for the given trace id.
   * @param checked Whether the boxed is checked or not.
   * @param traceId The trace id.
   */
  public SetCheckboxForTrace(checked: boolean, traceId: string) {
    const item = this.legendItems.find((i) => i.labels.includes(traceId));
    if (item) {
      this.toggleItemChecked(item, checked);
    }
  }

  /**
   * Sets all the checkboxes in the panel to the given checked state.
   * @param checked The desired state of the checkboxes.
   */
  public SetAllBoxes(checked: boolean) {
    const changedLabels: string[] = [];
    const itemsToChange: LegendItem[] = [];

    if (checked) {
      this.legendItems.forEach((item) => {
        if (!item.checked) {
          itemsToChange.push(item);
        }
      });
    } else {
      // Unchecking: don't uncheck the last item.
      const checkedItems = this.legendItems.filter((item) => item.checked);
      if (checkedItems.length > 1) {
        // Uncheck all but the last one.
        for (let i = 0; i < checkedItems.length - 1; i++) {
          itemsToChange.push(checkedItems[i]);
        }
      }
    }

    itemsToChange.forEach((item) => {
      item.checked = checked;
      changedLabels.push(...item.labels);
    });

    if (changedLabels.length > 0) {
      this.checkboxDispatchHandler(checked, changedLabels);
    }
    this.requestUpdate();
  }

  /**
   * Highlight the traces on the given indices on the panel.
   * @param traceIndices Indices of the traces to highlight.
   */
  public HighlightTraces(traceIndices: number[]) {
    this.legendItems.forEach((item) => (item.highlighted = false));

    if (!this.data) {
      return;
    }

    traceIndices.forEach((traceIndex) => {
      const label = this.data!.getColumnLabel(traceIndex + 2);
      const item = this.legendItems.find((i) => i.labels.includes(label));
      if (item) {
        item.highlighted = true;
      }
    });
    this.requestUpdate();
  }

  private toggleAllCheckboxes() {
    const headerCheckbox = this.renderRoot.querySelector(`#header-checkbox`) as HTMLInputElement;
    const isChecked = headerCheckbox.checked;

    const changedLabels: string[] = [];
    const itemsToChange: LegendItem[] = [];

    if (isChecked) {
      this.legendItems.forEach((item) => {
        if (!item.checked) {
          itemsToChange.push(item);
        }
      });
    } else {
      // Unchecking: don't uncheck the last item.
      const checkedItems = this.legendItems.filter((item) => item.checked);
      if (checkedItems.length <= 1) {
        // Revert the checkbox state since we can't uncheck the last item.
        headerCheckbox.checked = true;
        return;
      }
      // Uncheck all but the last one.
      for (let i = 0; i < checkedItems.length - 1; i++) {
        itemsToChange.push(checkedItems[i]);
      }
    }

    itemsToChange.forEach((item) => {
      item.checked = isChecked;
      changedLabels.push(...item.labels);
    });

    if (changedLabels.length > 0) {
      this.checkboxDispatchHandler(isChecked, changedLabels);
    }
    this.requestUpdate();
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

  private updateLegendItems() {
    if (!this.data) {
      this.legendItems = [];
      return;
    }

    const getLegendData = getLegend(this.data);
    if (getLegendData.length === 0) {
      this.legendItems = [];
      return;
    }

    // Determine the ordered list of keys to display from the first legend item.
    // `getLegend` ensures all items have the same keys in the same sorted order.
    const allKeys = Object.keys(getLegendData[0]);
    const displayKeys: string[] = [];

    // Special handling for 'test' key to ensure it's always first if it exists.
    if (allKeys.includes('test')) {
      displayKeys.push('test');
    }

    // Add remaining keys, excluding unwanted ones and 'test' (if already added).
    allKeys.forEach((key) => {
      if (!unwantedKeys.has(key) && key !== 'test') {
        displayKeys.push(key);
      }
    });

    this.legendKeysFormat = displayKeys.join('/');

    // Generate the display name for each legend item using the determined key order.
    const displayLegendList = getLegendData.map((legendEntryObj) => {
      const legendEntry = legendEntryObj as { [key: string]: string };
      return displayKeys
        .map((key) => legendEntry[key])
        .filter((value) => value)
        .join('/');
    });

    // Group trace labels by their generated display name.
    const legendToLabelsMap = new Map<string, string[]>();
    const numCols = this.data!.getNumberOfColumns();
    for (let i = 2; i < numCols; i++) {
      const label = this.data!.getColumnLabel(i);
      const displayName = displayLegendList[i - 2];
      if (!legendToLabelsMap.has(displayName)) {
        legendToLabelsMap.set(displayName, []);
      }
      legendToLabelsMap.get(displayName)!.push(label);
    }

    // Create the final list of legend items for rendering.
    const newLegendItems: LegendItem[] = [];
    const uniqueDisplayNames = [...legendToLabelsMap.keys()];

    uniqueDisplayNames.forEach((displayName) => {
      const labels = legendToLabelsMap.get(displayName)!;

      // Preserve state from existing items.
      const existingItem = this.legendItems.find((item) => item.displayName === displayName);
      const color = this.traceColorMap.get(labels[0]) || 'black';

      newLegendItems.push({
        displayName: displayName,
        labels: labels,
        color: color,
        checked: existingItem?.checked ?? true,
        highlighted: existingItem?.highlighted ?? false,
      });
    });
    this.legendItems = newLegendItems.sort((a, b) => {
      // Sort by display name, then by first label for consistent ordering.
      return a.displayName.localeCompare(b.displayName) || a.labels[0].localeCompare(b.labels[0]);
    });
  }

  private checkboxDispatchHandler(isSelected: boolean, labels: string[]): void {
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
