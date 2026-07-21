/**
 * @module modules/test-picker-sk
 * @description <h2><code>test-picker-sk</code></h2>
 *
 * A trace/test picker used to select a valid trace.
 * This element will guide the user by providing the following:
 *  - Specific order in which fields must be filled.
 *  - Fields with dropdown menus to aid in selecting valid values for each param.
 *  - Indicator as to when a test is ready to be plotted.
 *
 * This Element also provides the option to populate all the fields
 * using a given query. e.g.:
 *
 * populateFieldDataFromQuery(
 *    'benchmark=a&bot=b&test=c&subtest1=&subtest2=d',
 *    ['benchmark', 'bot', 'test', 'subtest1', 'subtest2']
 * )
 *
 * In the above case, fields will be filled in order of hierarchy until an
 * empty value is reached. Since subtest1 is empty, it'll stop filling at
 * subtest1, leaving subtest1 and subtest2 fields empty.
 *
 * @evt plot-button-clicked - Triggered when the Plot button is clicked.
 * It will contain the currently populated query in the test picker in
 * event.detail.query.
 *
 */
import { html, LitElement } from 'lit';
import { customElement, query } from 'lit/decorators.js';
import { ParamSet } from '../../../infra-sk/modules/query';
import '../picker-field-sk';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/spinner-sk';

import {
  TestPickerStateController,
  TestPickerStateControllerHost,
} from './test-picker-state-controller';
import { repeat } from 'lit/directives/repeat.js';
import { live } from 'lit/directives/live.js';
import { nothing } from 'lit';
import { MISSING_VALUE_SENTINEL } from '../const/const';

// The maximum number of matches before Plotting is enabled.
const PLOT_MAXIMUM: number = 200;

const MAX_MESSAGE = 'Reduce Traces';

// Data Structure to keep track of field information from controller.
import { FieldInfo } from './test-picker-state-controller';
import { DEFAULT_OPTION_LABEL } from '../common/test-picker';

@customElement('test-picker-sk')
export class TestPickerSk extends LitElement implements TestPickerStateControllerHost {
  ctrl = new TestPickerStateController(this);

  @query('#fieldContainer')
  private _containerDiv!: Element | null;

  @query('#plot-button')
  private _plotButton!: HTMLButtonElement | null;

  private _graphDiv: Element | null = null;

  // Track fields that need to open their overlay after render
  private _fieldPendingOpenOverlay: Set<string> = new Set();

  setFieldPendingOpenOverlay(param: string) {
    this._fieldPendingOpenOverlay.add(param);
  }

  // Track fields that need focus after render
  private _fieldPendingFocus: Set<string> = new Set();

  setFieldPendingFocus(param: string) {
    this._fieldPendingFocus.add(param);
  }

  createRenderRoot() {
    return this; // Render in light DOM preserving global styles
  }

  protected render() {
    return html`
      <div id="testPicker">
        <div id="fieldContainer">
          ${repeat(
            this.ctrl.fieldData,
            (fieldInfo: FieldInfo) => fieldInfo.param,
            (fieldInfo: FieldInfo) => {
              if (fieldInfo.value === null) {
                return nothing;
              }
              return html`
                <picker-field-sk
                  label=${fieldInfo.param}
                  .options=${fieldInfo.options}
                  .selectedItems=${fieldInfo.selectedItems}
                  .index=${fieldInfo.index}
                  .split=${live(fieldInfo.split)}
                  .splitDisabled=${fieldInfo.splitDisabled}
                  ?disabled=${fieldInfo.disabled || this.ctrl.readOnly}
                  @value-changed=${(e: Event) => {
                    void this.handleValueChangeForField(
                      fieldInfo.index,
                      (e as CustomEvent).detail.value,
                      (e as CustomEvent).detail.checkboxSelected,
                      !!(this._graphDiv && this._graphDiv.children.length > 0)
                    );
                  }}
                  @split-by-changed=${(e: Event) =>
                    this.ctrl.setSplitFields(
                      (e as CustomEvent).detail.param,
                      (e as CustomEvent).detail.split
                    )}></picker-field-sk>
              `;
            }
          )}
        </div>
        <div id="queryCount">
          <div class="test-picker-sk-matches-container">
            Traces: ${this.ctrl.requestInProgress ? '' : this.ctrl.count}
            <spinner-sk ?active=${this.ctrl.requestInProgress}></spinner-sk>
          </div>
          <div id="plot-button-container">
            <div ?hidden="${!(this.ctrl.count > 200)}">
              <span id="max-message" style="margin-left:2px"> (Reduce Traces) </span>
            </div>
            <button
              id="plot-button"
              @click=${() => this.onPlotButtonClick()}
              ?disabled=${this._computePlotButtonDisabled()}
              title=${this._computePlotButtonTitle()}>
              Plot
            </button>
          </div>
        </div>
      </div>
    `;
  }

  /**
   * Called when the element is added to the DOM.
   * Initializes references to DOM elements and renders the component.
   */
  connectedCallback(): void {
    super.connectedCallback();

    // The fields themselves are now rendered via Lit. Wait for component to update.
    void this.updateComplete.then(() => {
      this._graphDiv = document.querySelector('#graphContainer');
    });

    window.addEventListener('data-loaded', () => {
      this.ctrl.dataLoading = false;
      this.ctrl.setReadOnly(false);
      this.requestUpdate();
    });
  }

  protected updated() {
    // Handle pending operations after render
    for (const param of this._fieldPendingOpenOverlay) {
      const field = this._containerDiv?.querySelector(
        `picker-field-sk[label="${param}"]`
      ) as PickerFieldSk;
      if (field) {
        field.updateComplete.then(() => {
          field.openOverlay();
        });
      }
    }
    this._fieldPendingOpenOverlay.clear();

    for (const param of this._fieldPendingFocus) {
      const field = this._containerDiv?.querySelector(
        `picker-field-sk[label="${param}"]`
      ) as PickerFieldSk;
      if (field) {
        field.updateComplete.then(() => {
          field.focus();
        });
      }
    }
    this._fieldPendingFocus.clear();
  }

  private _computePlotButtonDisabled(): boolean {
    return this.ctrl.count > PLOT_MAXIMUM || this.ctrl.count <= 0;
  }

  private _computePlotButtonTitle(): string {
    if (this.ctrl.count === -1) {
      return 'Loading...';
    }
    if (this.ctrl.count > PLOT_MAXIMUM) {
      return 'Too many traces.';
    }
    if (this.ctrl.count === 0) {
      return 'No traces.';
    }
    if (this.ctrl.autoAddTrace) {
      return 'Traces will be added to the chart automatically.';
    }

    return 'Plot';
  }

  private onPlotButtonClick() {
    this.dispatchEvent(
      new CustomEvent('plot-button-clicked', {
        detail: {
          query: this.ctrl.createQueryFromFieldData(),
        },
        bubbles: true,
      })
    );
  }

  async populateFieldDataFromQuery(
    queryString: string,
    params: string[],
    paramSet: ParamSet,
    hasGraphLoaded: boolean = true
  ) {
    await this.ctrl.populateFieldDataFromQuery(queryString, params, paramSet, hasGraphLoaded);
  }

  /**
   * Populates the field data from a given ParamSet.
   * This function initializes the field data based on the unique keys from
   * both paramSets and paramSet, and then populates the fields with the
   * corresponding values and options.
   * If no parameters are provided in `paramSet`, `autoAddTrace` is set to true.
   * @param paramSets - A ParamSet object containing the initial selected
   * values for the fields.
   * @param paramSet - A ParamSet object containing available options for each
   * parameter.
   * @param splitByKeys - An array of parameter keys that should be split.
   * @param hasGraphLoaded - Whether the graph is loaded or being loaded.
   */
  async populateFieldDataFromParamSet(
    paramSets: ParamSet,
    paramSet: ParamSet,
    splitByKeys: string[] = [],
    hasGraphLoaded: boolean = true
  ): Promise<void> {
    await this.ctrl.populateFieldDataFromParamSet(paramSets, paramSet, splitByKeys, hasGraphLoaded);
  }

  /**
   * Updates the graph based on the selected values.
   * If `autoAddTrace` is false, no update occurs.
   * If the trace count exceeds `PLOT_MAXIMUM`, an error message is displayed.
   * Dispatches 'remove-trace' or 'add-to-graph' events based on changes.
   *
   * @param value The current selected values for the field.
   * @param fieldInfo The FieldInfo object for the current field.
   * @param removedValue The values that were removed from the selection.
   */
  private updateGraph(
    value: string[],
    fieldInfo: FieldInfo,
    removedValue: string[],
    hasGraphLoaded: boolean = false
  ) {
    if (this.ctrl.forceManualPlot) {
      return;
    }

    const mappedValue = value.map((v) => (v === DEFAULT_OPTION_LABEL ? MISSING_VALUE_SENTINEL : v));
    const mappedRemovedValue = removedValue.map((v) =>
      v === DEFAULT_OPTION_LABEL ? MISSING_VALUE_SENTINEL : v
    );

    const isOverLimit = this.ctrl.count > PLOT_MAXIMUM;

    if (fieldInfo.index === 0 && value.length === 0 && isOverLimit) {
      const detail = {
        query: this.ctrl.createQueryFromFieldData(),
        param: fieldInfo.param,
        value: removedValue.length > 0 ? mappedRemovedValue : mappedValue,
        isSplit: fieldInfo.split,
      };
      this.dispatchEvent(
        new CustomEvent('remove-trace', { detail, bubbles: true, composed: true })
      );
      return;
    }

    if (hasGraphLoaded) {
      if (!this.ctrl.autoAddTrace && removedValue.length === 0) {
        return;
      }

      this.ctrl.setReadOnly(true);
      const detail = {
        query: this.ctrl.createQueryFromFieldData(),
        param: fieldInfo.param,
        value: removedValue.length > 0 ? mappedRemovedValue : mappedValue,
        isSplit: fieldInfo.split,
      };

      const isImplicitAll = value.length === 0;
      const willDispatchAdd =
        !isOverLimit && (removedValue.length === 0 || isImplicitAll) && this.ctrl.autoAddTrace;

      if (!willDispatchAdd && removedValue.length === 0) {
        this.ctrl.setReadOnly(false);
        if (isOverLimit) {
          void errorMessage(MAX_MESSAGE);
        }
        return;
      }

      if (removedValue.length > 0 && !willDispatchAdd) {
        this.dispatchEvent(
          new CustomEvent('remove-trace', { detail, bubbles: true, composed: true })
        );
      }
      if (fieldInfo.split && value.length < 2) {
        this.ctrl.setSplitFields(fieldInfo.param, false);
        this.dispatchEvent(
          new CustomEvent('split-by-changed', {
            detail: { param: fieldInfo.param, split: false },
            bubbles: true,
            composed: true,
          })
        );
        return;
      }
      if (willDispatchAdd) {
        this.dispatchEvent(new CustomEvent('add-to-graph', { detail, bubbles: true }));
      }
    }
  }

  async initializeTestPicker(
    params: string[],
    defaultParams: { [key: string]: string[] | null },
    readOnly: boolean,
    forceManualPlot: boolean = false
  ): Promise<void> {
    await this.ctrl.initializeTestPicker(params, defaultParams, readOnly, forceManualPlot);
  }

  async applyConditionalDefaults(
    triggerParam: string,
    triggerValues: string[],
    hasGraphLoaded: boolean = false
  ) {
    const defaults = (document.querySelector('explore-multi-sk') as any)?.defaults;
    if (!defaults || !defaults.conditional_defaults) {
      return;
    }

    let madeChanges = false;
    for (const rule of defaults.conditional_defaults) {
      if (
        rule.trigger.param === triggerParam &&
        rule.trigger.values.some((v: string) => triggerValues.includes(v))
      ) {
        for (const applyItem of rule.apply) {
          const targetFieldInfo = this.ctrl.fieldData.find(
            (f: FieldInfo) => f.param === applyItem.param
          );
          if (targetFieldInfo) {
            const availableOptions = new Set(targetFieldInfo.options);
            let newSelectedItems: string[] = [];
            if (applyItem.select_only_first) {
              const firstAvailable = applyItem.values.find((v: string) => availableOptions.has(v));
              if (firstAvailable) {
                newSelectedItems = [firstAvailable];
              }
            } else {
              newSelectedItems = applyItem.values.filter((v: string) => availableOptions.has(v));
            }

            if (newSelectedItems.length > 0) {
              targetFieldInfo.value = newSelectedItems;
              targetFieldInfo.selectedItems = newSelectedItems;
              madeChanges = true;
              await this.ctrl.fetchExtraOptions(targetFieldInfo.index, hasGraphLoaded);
            }
          }
        }
      }
    }

    if (madeChanges) {
      this.ctrl.fieldData = [...this.ctrl.fieldData];
      this.requestUpdate();
    }
  }

  async handleValueChangeForField(
    index: number,
    value: string[],
    checkboxSelected: boolean,
    hasGraphLoaded: boolean = false
  ) {
    const fieldInfo = this.ctrl.fieldData[index];

    if (JSON.stringify(value) === JSON.stringify(fieldInfo.selectedItems) && !checkboxSelected) {
      return;
    }

    if (value.length === 0) {
      this.ctrl.removeChildFields(index);
    }

    const newValues = new Set(value);
    const oldValues = new Set(fieldInfo.value || []);
    const removed = [...oldValues].filter((x) => !newValues.has(x));

    if (
      !this.ctrl.forceManualPlot &&
      fieldInfo.index === 0 &&
      this.ctrl.fieldData[0].selectedItems.length > 0 &&
      removed.length === 0
    ) {
      fieldInfo.selectedItems = this.ctrl.fieldData[0].selectedItems;
      this.ctrl.fieldData = [...this.ctrl.fieldData];
      void errorMessage('Unable to add more items to the first field.');
      this.requestUpdate();
      return;
    }

    if (fieldInfo.value !== value) {
      fieldInfo.value = value;
      fieldInfo.selectedItems = value;
    }

    this.ctrl.fieldData = [...this.ctrl.fieldData];

    if (value.length > 0) {
      this.ctrl.setReadOnly(true);
      const cascadeId = this.ctrl.startNewCascade();
      await this.ctrl.fetchExtraOptions(index, hasGraphLoaded, cascadeId);
    }
    this.updateGraph(value, fieldInfo, removed, hasGraphLoaded);
    if (value.length > 0) {
      await this.applyConditionalDefaults(fieldInfo.param, value, hasGraphLoaded);
    }
    this.requestUpdate();
  }

  removeItemFromChart(param: string, value: string[]) {
    const hasGraphLoaded = !!(this._graphDiv && this._graphDiv.children.length > 0);
    const fieldIndex = this.ctrl.fieldData.findIndex((field: FieldInfo) => field.param === param);
    const fieldInfo = this.ctrl.fieldData[fieldIndex];
    if (fieldInfo) {
      const newValue = (fieldInfo.value || []).filter((v: string) => !value.includes(v));
      void this.handleValueChangeForField(fieldIndex, newValue, false, hasGraphLoaded);
    }
  }

  isLoaded(): boolean {
    return this.ctrl.isLoaded();
  }

  createQueryFromFieldData(): string {
    return this.ctrl.createQueryFromFieldData();
  }

  setReadOnly(readOnly: boolean) {
    this.ctrl.setReadOnly(readOnly);
  }

  createParamSetFromFieldData(): ParamSet {
    return this.ctrl.createParamSetFromFieldData();
  }

  get autoAddTrace(): boolean {
    return this.ctrl.autoAddTrace;
  }

  set autoAddTrace(auto: boolean) {
    this.ctrl.autoAddTrace = auto;
  }

  get readOnly(): boolean {
    return this.ctrl.readOnly;
  }

  set readOnly(val: boolean) {
    this.ctrl.readOnly = val;
  }
}
