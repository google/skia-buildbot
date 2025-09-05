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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParamSet, fromParamSet, toParamSet } from '../../../infra-sk/modules/query';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';

import { NextParamListHandlerRequest, NextParamListHandlerResponse } from '../json';
import '../picker-field-sk';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/spinner-sk';

// The maximum number of matches before Plotting is enabled.
const PLOT_MAXIMUM: number = 200;

const MAX_MESSAGE = 'Reduce Traces';

// Data Structure to keep track of field information.
class FieldInfo {
  field: PickerFieldSk | null = null; // The field element itself.

  param: string = ''; // The label of the field. Must match a valid trace key in CDB.

  value: string[] = []; // The currently selected value in a field.

  splitBy: string[] = []; // Split item selected.

  index: number = 0; // Index of the field in the fieldData array.

  onValueChanged: ((e: Event) => void) | null = null;

  onSplitByChanged: ((e: Event) => void) | null = null;
}

export class TestPickerSk extends ElementSk {
  private _fieldData: FieldInfo[] = [];

  private _count: number = -1;

  private _containerDiv: Element | null = null;

  private _plotButton: HTMLButtonElement | null = null;

  private _graphDiv: Element | null = null;

  private _requestInProgress: boolean = false;

  private _currentIndex: number = 0;

  private _defaultParams: { [key: string]: string[] | null } = {};

  private _autoAddTrace: boolean = false;

  private _readOnly: boolean = false;

  constructor() {
    super(TestPickerSk.template);
  }

  private static template = (ele: TestPickerSk) => html`
    <div id="testPicker">
      <div id="fieldContainer"></div>
      <div id="queryCount">
        <div class="test-picker-sk-matches-container">
          Traces: ${ele._requestInProgress ? '' : ele._count}
          <spinner-sk ?active=${ele._requestInProgress}></spinner-sk>
        </div>
        <div id="plot-button-container">
          <div ?hidden="${!(ele._count > PLOT_MAXIMUM)}">
            <span id="max-message" style="margin-left:2px"> (${MAX_MESSAGE}) </span>
          </div>
          <button
            id="plot-button"
            @click=${ele.onPlotButtonClick}
            disabled
            title="Plot a graph on selected values.">
            Plot
          </button>
        </div>
      </div>
    </div>
  `;

  /**
   * Called when the element is added to the DOM.
   * Initializes references to DOM elements and renders the component.
   */
  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this._containerDiv = this.querySelector('#fieldContainer');
    this._plotButton = this.querySelector('#plot-button');
    this._graphDiv = document.querySelector('#graphContainer');
  }

  /**
   * Adds a new PickerFieldSk element to the field container.
   * This function fetches options for the new field from the backend,
   * initializes and populates the field, focuses it, and sets up event
   * listeners.
   */
  private addChildField(readOnly: boolean) {
    const currentIndex = this._currentIndex;
    const currentFieldInfo = this._fieldData[currentIndex];
    const param = currentFieldInfo.param;

    const handler = (json: NextParamListHandlerResponse) => {
      this.updateCount(json.count);

      if (param in json.paramset && json.paramset[param] !== null) {
        const options = json.paramset[param].filter((option: string) => !option.includes('.'));
        options.sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }));
        const field: PickerFieldSk = new PickerFieldSk(param);
        currentFieldInfo.field = field;
        this._containerDiv!.appendChild(field);
        this.setReadOnly(readOnly);

        field!.label = param;
        field!.options = options;
        field.index = this._currentIndex;
        const extraTests = json.paramset[param].filter((option: string) => option.includes('.'));
        if (extraTests.length > 0) {
          field!.options = options.concat(extraTests);
        }
        this._currentIndex += 1;
        field!.focus();
        if (currentIndex !== 0) {
          field!.openOverlay();
        }

        this.addValueUpdatedEventToField(currentIndex);
      }
      this._render();
    };
    this.callNextParamList(handler, currentIndex);
  }

  /**
   * Removes child fields from the field container starting from a given index.
   * This function iterates through the `_fieldData` array from the current
   * index down to the specified index, removing the corresponding
   * `PickerFieldSk` elements from the DOM and resetting their values.
   * It also dispatches a 'split-by-changed' event if a split field is removed.
   * @param index The index from which to start removing child fields.
   */
  private removeChildFields(index: number) {
    while (this._currentIndex > index) {
      const fieldInfo = this._fieldData[this._currentIndex];
      // Remove split if it was previously enabled.
      if (fieldInfo.splitBy.length > 0) {
        fieldInfo.field!.split = false;
        this.dispatchEvent(
          new CustomEvent('split-by-changed', {
            detail: {
              param: fieldInfo.param,
              split: false,
            },
            bubbles: true,
            composed: true,
          })
        );
      }
      fieldInfo.value = [];
      if (fieldInfo.field !== null && this._containerDiv?.contains(fieldInfo.field!)) {
        this._containerDiv!.removeChild(fieldInfo.field!);
      }
      fieldInfo.field = null;
      this._currentIndex -= 1;
    }
    this._render();
  }

  /**
   * Sets the readonly property for all rendered fields.
   * When `readonly` is true, all fields and the plot button are disabled.
   * When `readonly` is false, all fields are enabled, and the plot button is
   * enabled unless `autoAddTrace` is true.
   * @param readonly - A boolean indicating whether the fields should be
   * read-only.
   */
  setReadOnly(readonly: boolean) {
    this._readOnly = readonly;
    this._fieldData.forEach((field) => {
      if (readonly) {
        field.field?.disable();
        this._plotButton!.disabled = true;
      } else {
        field.field?.enable();
        if (!this.autoAddTrace) {
          this._plotButton!.disabled = false;
        }
      }
    });
  }

  get readOnly() {
    return this._readOnly;
  }

  /**
   * Makes a POST request to the /_/nextParamList/ endpoint to fetch parameter
   * lists.
   * Updates the count and sets `_requestInProgress` to true during the request.
   * Disables fields if multiple selections are not allowed.
   * Re-enables fields and calls the handler function on success.
   * On failure, removes child fields and displays an error message.
   * @param handler - A callback function to handle the JSON response.
   * @param index - The index of the current field.
   */
  private callNextParamList(handler: (json: NextParamListHandlerResponse) => void, index: number) {
    this.updateCount(-1);
    this._requestInProgress = true;
    // Allow multiple selections to continue.
    if (!(this._fieldData[index].value.length > 1)) {
      this.setReadOnly(true);
    }
    this._render();

    const fieldData = this.createQueryFromFieldData();
    const body: NextParamListHandlerRequest = {
      q: fieldData,
    };

    fetch('/_/nextParamList/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._requestInProgress = false;
        // Only re-enable when autoadd is false, and we have results or it is the initial
        handler(json);
      })
      .catch((msg: any) => {
        this._requestInProgress = false;
        this.setReadOnly(false);
        // If the request fails, we remove child fields to reset.
        this.removeChildFields(0);
        errorMessage(msg);
      });
  }

  /**
   * Fetches the available options for a given field from the backend.
   * After fetching, it populates the field's dropdown, updates the match count,
   * focuses the field, and opens its overlay if it's not the first field.
   * @param index - The index of the field in the `_fieldData` array.
   */
  private fetchOptions(index: number) {
    const fieldInfo = this._fieldData[index];
    const field = fieldInfo.field;
    const param = fieldInfo.param;

    const handler = (json: NextParamListHandlerResponse) => {
      if (param in json.paramset && json.paramset[param] !== null) {
        const options = json.paramset[param].filter((option: string) => !option.includes('.'));
        field!.options = options;
        this.updateCount(json.count);
        field!.focus();
        if (index !== 0) {
          field!.openOverlay();
        }
        this._render();
      }
    };
    this.callNextParamList(handler, index);
  }

  /**
   * Handles the click event on the Plot button.
   * Dispatches a 'plot-button-clicked' custom event with the current query.
   */
  private onPlotButtonClick() {
    const detail = {
      query: this.createQueryFromFieldData(),
    };
    this.dispatchEvent(
      new CustomEvent('plot-button-clicked', {
        detail: detail,
        bubbles: true,
      })
    );
    this._render();
  }

  /**
   * Resets the test picker and populates the fields with an input query.
   * This function parses the input query, initializes the field data based on
   * the provided parameters, and then populates the fields with the
   * corresponding selected values and available options.
   * Note that calling this function will overwrite any current selections in
   * the test picker.
   * @param query - The query string to populate the fields from
   * (e.g., 'benchmark=a&bot=b&test=c').
   * @param params - An array of parameter names defining the hierarchy of the
   * fields.
   * @param paramSet - A ParamSet object containing available options for each
   * parameter.
   */
  populateFieldDataFromQuery(query: string, params: string[], paramSet: ParamSet) {
    const selectedParams: ParamSet = toParamSet(query);
    if (paramSet) {
      const paramKeys: string[] = Object.keys(paramSet).filter((key) => key in selectedParams);
      this.initializeFieldData(paramKeys);
    } else {
      this.initializeFieldData(params);
    }
    for (let i = 0; i < this._fieldData.length; i++) {
      const fieldInfo = this._fieldData[i];
      const param = fieldInfo.param;
      const field: PickerFieldSk = new PickerFieldSk(param);
      fieldInfo.field = field;
      fieldInfo.field.index = i;
      this._containerDiv!.appendChild(field);

      // Set selected items from the query
      const selectedValue = selectedParams[fieldInfo.param] || [];
      field.selectedItems = selectedValue;
      fieldInfo.value = selectedValue;

      // If there are available options provided, use them.
      if (paramSet && paramSet[param]) {
        field.options = paramSet[param];
      }
      field.index = i;

      // Add event listener for value changes
      this.addValueUpdatedEventToField(i);
      this.fetchExtraOptions(i);

      field.focus();
      this._render();
    }
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
   */
  populateFieldDataFromParamSet(paramSets: ParamSet, paramSet: ParamSet) {
    const uniqueParamKeys = [...new Set([...Object.keys(paramSets), ...Object.keys(paramSet)])];
    this.initializeFieldData(uniqueParamKeys);
    this._currentIndex = 0; // Reset current index for proper field initialization
    // If no params are provided, then chart is loaded with all traces.
    if (Object.keys(paramSet).length === 0) {
      this.autoAddTrace = true;
    }

    for (let i = 0; i < this._fieldData.length; i++) {
      const fieldInfo = this._fieldData[i];
      const param = fieldInfo.param;
      fieldInfo.field = new PickerFieldSk(param);
      // Combine options from both paramSets and paramSet for the current param.
      const allOptions = [
        ...new Set([...(paramSets[param] || []), ...(paramSet[param] || [])]),
      ].sort();
      const value = paramSets[param] || [];
      if (value.length === 0) {
        break; // Stop after the first field without a value
      }
      if (allOptions.length > 0) {
        fieldInfo.field.options = allOptions;
        fieldInfo.field.selectedItems = value;
        fieldInfo.field.index = i;
        fieldInfo.value = value;
      }
      this.fetchExtraOptions(i);
      this._containerDiv!.appendChild(fieldInfo.field);
    }
  }

  /**
   * Handles the toggle event of the auto-add trace checkbox.
   * Sets the `autoAddTrace` property based on the checkbox's checked state.
   * @param e The event object from the checkbox toggle.
   */
  private onToggleCheckboxClick(e: Event): void {
    this.autoAddTrace = (e.target as CheckOrRadio).checked;
    // This prevents a double event from happening.
    e.preventDefault();
    this._render();
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
  private updateGraph(value: string[], fieldInfo: FieldInfo, removedValue: string[]) {
    // No valid data, so remove entire graph.
    if (fieldInfo.index === 0 && value.length === 0) {
      const detail = {
        query: this.createQueryFromFieldData(),
        param: fieldInfo.param,
        value: value.length > 0 ? removedValue : value,
      };
      this.dispatchEvent(
        new CustomEvent('remove-trace', {
          detail: detail,
          bubbles: true,
          composed: true,
        })
      );
      return;
    }
    // Only update when autoAdd is ready and chart is active.
    if (!this.autoAddTrace) {
      return;
    }
    if (this._count > PLOT_MAXIMUM) {
      // Show error message if there are too many traces.
      errorMessage(MAX_MESSAGE);
      return;
    }
    this.setReadOnly(true);
    if (this._graphDiv !== null && this._graphDiv.children.length > 0) {
      const detail = {
        query: this.createQueryFromFieldData(),
        param: fieldInfo.param,
        value: value.length > 0 ? removedValue : value,
      };
      if (removedValue.length > 0) {
        // Remove item from chart, no need to requery.
        this.dispatchEvent(
          new CustomEvent('remove-trace', {
            detail: detail,
            bubbles: true,
            composed: true,
          })
        );
        return;
      }
      // Field was split, but not enough values so remove split.
      if (fieldInfo.field!.split && value.length < 2) {
        this.setSplitFields(fieldInfo.param, false);
        this.dispatchEvent(
          new CustomEvent('split-by-changed', {
            detail: {
              param: fieldInfo.param,
              split: false,
            },
            bubbles: true,
            composed: true,
          })
        );
        return;
      }
      this.dispatchEvent(
        new CustomEvent('add-to-graph', {
          detail: detail,
          bubbles: true,
        })
      );
    }
  }

  private addValueUpdatedEventToField(index: number) {
    const fieldInfo = this._fieldData[index];
    if (fieldInfo.field === null) {
      return;
    }
    // Remove existing listeners if they exist.
    if (fieldInfo.onValueChanged) {
      fieldInfo.field.removeEventListener('value-changed', fieldInfo.onValueChanged);
    }
    if (fieldInfo.onSplitByChanged) {
      fieldInfo.field.removeEventListener('split-by-changed', fieldInfo.onSplitByChanged);
    }

    // Create and store the new listeners.
    fieldInfo.onValueChanged = (e: Event) => {
      const value = (e as CustomEvent).detail.value as string[];
      if (value.length === 0) {
        this.removeChildFields(index);
      }
      if (value === fieldInfo.field!.selectedItems) {
        // Updated already, ignore.
        return;
      }

      const newValues = new Set(value);
      const oldValues = new Set(fieldInfo.value);
      const removed = [...oldValues].filter((x) => !newValues.has(x));

      // Don't update graph if the first field is changed as it can overload
      // the graph.
      if (
        this._fieldData[0].param === fieldInfo.param &&
        this._fieldData[0].field!.selectedItems.length > 0 &&
        removed.length === 0
      ) {
        fieldInfo.field!.selectedItems = this._fieldData[0].field!.selectedItems;
        errorMessage('Unable to add more items to the first field.');
        return;
      }

      if (fieldInfo.value !== value) {
        fieldInfo.value = value;
      }
      if (value.length === 0) {
        // Chart needs to be reset, so disable autoAddTrace.
        this.autoAddTrace = false;
      }
      if (value.length !== fieldInfo.field!.selectedItems.length) {
        // Selected Item Needs to be updated.
        fieldInfo.field!.selectedItems = value;
      }
      this.updateGraph(value, fieldInfo, removed);
      this.fetchExtraOptions(index);
    };

    fieldInfo.onSplitByChanged = (e: Event) => {
      const param = (e as CustomEvent).detail.param;
      const split = (e as CustomEvent).detail.split;
      this.setSplitFields(param, split);
    };

    // Add the new listeners.
    fieldInfo.field!.addEventListener('value-changed', fieldInfo.onValueChanged);
    fieldInfo.field.addEventListener('split-by-changed', fieldInfo.onSplitByChanged);
  }

  /**
   * Sets the split property for a given parameter and enables/disables split
   * options for other fields.
   * @param param The parameter to set the split property for.
   * @param split A boolean indicating whether to split or not.
   */
  private setSplitFields(param: string, split: boolean) {
    for (let i = 0; i < this._fieldData.length; i++) {
      if (this._fieldData[i].param === param) {
        (this._fieldData[i].field as PickerFieldSk).split = split;
        // Set split values and disable all other params
        if (split) {
          this._fieldData[i].splitBy = [param];
        } else {
          this._fieldData[i].splitBy = [];
        }
      } else {
        // Enable or disable the rest of the Split options to avoid multiple
        // splits from being attempted.
        if (split) {
          this._fieldData[i].field?.disableSplit();
        } else {
          this._fieldData[i].field?.enableSplit();
        }
      }
    }
  }

  /**
   * Fetches the values for a given field.
   *
   * When creating a new field, we need to talk to the backend to
   * figure out which options the field can provide as valid options in
   * its dropdown menu.
   *
   * Once options are fetched, the field will be populated. Its dropdown
   * menu will be automatically opened, unless it is the first field.
   * The match count is also updated.
   *
   * @param index
   */
  private fetchExtraOptions(index: number) {
    const handler = (json: NextParamListHandlerResponse) => {
      const param = Object.keys(json.paramset)[0];
      const count: number = json.count || -1;
      if (param !== undefined) {
        for (let i = 0; i < this._fieldData.length; i++) {
          const fieldInfo = this._fieldData[i];
          if (fieldInfo.param === param) {
            if (fieldInfo.field === null) {
              const field: PickerFieldSk = new PickerFieldSk(param);
              fieldInfo.field = field;
              this._containerDiv!.appendChild(field);
            }
            fieldInfo.field!.options = json.paramset[param].filter(
              (option: string) => !option.includes('.')
            );
            const extraTests = json.paramset[param].filter((option: string) =>
              option.includes('.')
            );
            if (extraTests.length > 0) {
              fieldInfo.field!.options = fieldInfo.field!.options.concat(extraTests);
            }
            fieldInfo.field.index = i;
            fieldInfo.field!.focus();
            this.addValueUpdatedEventToField(i);
            // Track the furthest index queried
            if (this._currentIndex <= i) {
              this._currentIndex = i;
            }
            this.updateCount(count);
            break;
          }
        }
      } else {
        // No parameter, so last item. Update count.
        this._currentIndex = this._fieldData.length - 1;
        this.updateCount(count);
      }
      this._render();
    };
    this.callNextParamList(handler, index);
  }

  /**
   * Generates a query string from the currently selected field values.
   * Includes default parameter values if they are not already specified in the
   * selected fields.
   * @returns A query string representing the selected field values.
   */
  createQueryFromFieldData(): string {
    const paramSet: ParamSet = {};
    if (this._fieldData[0].value === null) {
      return '';
    }

    this._fieldData.forEach((fieldInfo) => {
      if (fieldInfo.value !== null) {
        paramSet[fieldInfo.param] = fieldInfo.value;
      }
    });

    // If all fields are empty, don't add any defaults, which can potentially
    // make the query slow. An empty query should be a fast retrieval.
    if (Object.keys(paramSet).length === 0) {
      return '';
    }
    // If values are set in child values, but missing initial value, then exit.
    if (this._fieldData[0].value.length === 0) {
      return '';
    }
    // Apply default values.
    for (const defaultParamKey in this._defaultParams) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = this._defaultParams![defaultParamKey]!;
      }
    }

    return fromParamSet(paramSet);
  }

  /**
   * Generates a query string from the selected field values up to a specified
   * index.
   * Includes default parameter values if they are not already specified in the
   * selected fields.
   * @param index - The maximum index of the field to include in the query
   * string.
   * @returns A query string representing the selected field values up to the
   * specified index.
   */
  createQueryFromIndex(index: number): string {
    const paramSet: ParamSet = {};

    for (let i = 0; i <= index; i++) {
      const fieldInfo = this._fieldData[i];
      if (fieldInfo.value !== null) {
        paramSet[fieldInfo.param] = fieldInfo.value;
      }
    }

    // If all fields are empty, don't add any defaults, which can potentially
    // make the query slow. An empty query should be a fast retrieval.
    if (Object.keys(paramSet).length === 0) {
      return '';
    }

    // Apply default values.
    for (const defaultParamKey in this._defaultParams) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = this._defaultParams![defaultParamKey]!;
      }
    }

    return fromParamSet(paramSet);
  }

  /**
   * Updates the count of matching traces and controls the state of the Plot
   * button.
   * If `count` is -1, it indicates loading, disabling the plot button and
   * setting its title to 'Loading...'.
   * If `count` is greater than `PLOT_MAXIMUM` or less than or equal to 0, the
   * plot button is disabled.
   * Otherwise, the plot button is enabled, and `autoAddTrace` is set based on
   * whether a graph is already loaded.
   * @param count - The number of matching traces.
   */
  private updateCount(count: number) {
    this._plotButton!.disabled = true;
    if (count === -1) {
      // Loading new data, so disable plotting.
      this._plotButton!.title = 'Loading...';
      // Still loading so
      if (this._currentIndex > 0) {
        this.setReadOnly(true);
      } else {
        this.setReadOnly(false);
      }
      this._count = 0;
      return;
    }

    this.setReadOnly(false);
    this._count = count;
    if (count > PLOT_MAXIMUM || count <= 0) {
      // Disable plotting if there are too many or no traces.
      this.autoAddTrace = false;
      this._plotButton!.title = this._count > PLOT_MAXIMUM ? 'Too many traces.' : 'No traces.';
      this._plotButton!.disabled = true;
      return;
    }
    if (this._graphDiv && this._graphDiv.children.length > 0) {
      // Graph is already loaded, so allow new changes automatically.
      this.autoAddTrace = true;
    } else {
      // No graph loaded yet, so allow plotting.
      this._plotButton!.title = 'Plot graph';
      this.autoAddTrace = false;
      this._plotButton!.disabled = false;
    }
  }

  /**
   * Sets whether traces should be added automatically to the graph.
   * If `autoAdd` is true, the plot button will be disabled and its title will
   * indicate automatic addition.
   * Otherwise, the plot button will be enabled and its title will indicate
   * manual plotting.
   * @param autoAdd - A boolean indicating whether to automatically add traces.
   */
  set autoAddTrace(autoAdd: boolean) {
    this._autoAddTrace = autoAdd;
    if (this._plotButton !== null) {
      if (this._count > 0) {
        this._plotButton.disabled = autoAdd;
        this._plotButton!.title = autoAdd ? 'Traces are added automatically' : 'Plot a graph';
      }
    }
  }

  /**
   * Returns whether traces are automatically added to the graph.
   * @returns A boolean indicating whether traces are automatically added.
   */
  get autoAddTrace(): boolean {
    return this._autoAddTrace;
  }

  /**
   * Returns true if the first field is loaded.
   *
   * This is used to determine if the test picker is ready to be used.
   * If the first field is not loaded, then we are not ready.
   *
   * @returns true if the first field is loaded, false otherwise.
   */
  isLoaded(): boolean {
    // If the first field is not loaded, then we are not ready.
    return (
      this._fieldData.length > 0 &&
      this._fieldData[0].field !== null &&
      this._fieldData[0].field.selectedItems.length > 0
    );
  }

  /**
   * Initializes Test Picker from scratch.
   *
   * Initializes the fieldData structure based on params given, and
   * renders the first field for the user.
   *
   * @param params - A list of params that'll be used to populate
   * the field labels and query the DB. The order of the list establishes
   * a hierarchy in which each field can be populated.
   *
   * @param defaultParams - A map of default param values to apply to test
   * selections. For example, if defaultParams is { 'bot': ['linux-perf'] },
   * queries will automatically get "bot=linux-perf" appended. The exception
   * is if bot is already specified in the query, then no defaults are applied.
   */
  initializeTestPicker(
    params: string[],
    defaultParams: { [key: string]: string[] | null },
    readOnly: boolean
  ) {
    this._defaultParams = defaultParams;
    this.initializeFieldData(params);
    this.addChildField(readOnly);
    this._render();
  }

  /**
   * Resets data structures from scratch.
   * Clears the field container DOM, resets the index,
   * resets the fieldData structure, and initializes fieldData
   * based on given params.
   * @param params - An array of parameter names to initialize the field data
   * with.
   */
  private initializeFieldData(params: string[]) {
    this._containerDiv!.replaceChildren();
    this._currentIndex = 0;
    if (this._fieldData.length > 0) {
      this._fieldData = this._fieldData.filter((fieldInfo) => {
        if (params.includes(fieldInfo.param)) {
          fieldInfo.field = null;
          fieldInfo.value = [];
          return true;
        }
        return false;
      });
    } else {
      this._fieldData = [];
      params.forEach((param, i) => {
        this._fieldData.push({
          field: null,
          param: param,
          value: [],
          splitBy: [],
          index: i,
          onValueChanged: null,
          onSplitByChanged: null,
        });
      });
    }
    if (this._graphDiv && this._graphDiv.children.length > 0) {
      this.autoAddTrace = true;
    }
  }

  /**
   * Removes an item from the chart.
   * @param param The parameter of the item to remove.
   * @param value The value of the item to remove.
   */
  removeItemFromChart(param: string, value: string[]) {
    // Find the field info for the given param.
    const fieldInfo = this._fieldData.find((field) => field.param === param);
    if (fieldInfo) {
      const newValue = fieldInfo.value.filter((v) => !value.includes(v));
      // Update the selected items in the field.
      if (fieldInfo.field) {
        fieldInfo.field.selectedItems = newValue;
      }
    }
  }
}

define('test-picker-sk', TestPickerSk);
