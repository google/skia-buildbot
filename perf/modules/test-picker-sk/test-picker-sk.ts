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
import {
  ParamSet,
  fromParamSet,
  toParamSet,
} from '../../../infra-sk/modules/query';

import {
  NextParamListHandlerRequest,
  NextParamListHandlerResponse,
} from '../json';
import '../picker-field-sk';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/spinner-sk';

// The maximum number of matches before Plotting is enabled.
const PLOT_MAXIMUM = 10;

// Data Structure to keep track of field information.
class FieldInfo {
  field: PickerFieldSk | null = null; // The field element itself.

  param: string = ''; // The label of the field. Must match a valid trace key in CDB.

  value: string = ''; // The currently selected value in a field.
}

export class TestPickerSk extends ElementSk {
  private _fieldData: FieldInfo[] = [];

  private _count: string = '';

  private _containerDiv: Element | null = null;

  private _plotButton: HTMLButtonElement | null = null;

  private _requestInProgress: boolean = false;

  private _currentIndex: number = 0;

  private _defaultParams: { [key: string]: string[] | null } = {};

  constructor() {
    super(TestPickerSk.template);
  }

  private static template = (ele: TestPickerSk) => html`
    <div id="testPicker">
      <div id="fieldContainer"></div>
      <div id="queryCount">
        <div>
          Matches: <span>${ele._count}</span
          ><spinner-sk ?active=${ele._requestInProgress}></spinner-sk>
        </div>
        <button
          id="plot-button"
          @click=${ele.onPlotButtonClick}
          title="Plot a graph on selected values. Narrow down selection to ${PLOT_MAXIMUM} matches to be able to plot."
          disabled>
          Add Graph
        </button>
      </div>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this._containerDiv = this.querySelector('#fieldContainer');
    this._plotButton = this.querySelector('#plot-button');
  }

  /**
   * Adds a PickerFieldSk to the fieldContainer div.
   *
   * Order of events:
   * 1. Fetch options for the next child to be added.
   * 2. If we receive options, we initialize new PickerFieldSk object. Otherwise, do
   *    create a new field. Just update the count.
   * 2. Populate the field with fetched dropdown options.
   * 3. Focus the field.
   * 4. Open the dropdown selection menu automatically if it's not the first field.
   * 5. Add eventListener to field specifying how to handle selected value changes.
   */
  private addChildField() {
    const currentIndex = this._currentIndex;
    const currentFieldInfo = this._fieldData[currentIndex];
    const param = currentFieldInfo.param;

    const handler = (json: NextParamListHandlerResponse) => {
      this.updateCount(json.count);

      if (param in json.paramset && json.paramset[param] !== null) {
        const options = json.paramset[param];
        const field: PickerFieldSk = new PickerFieldSk(param);
        currentFieldInfo.field = field;
        this._containerDiv!.appendChild(field);
        this._currentIndex += 1;

        field!.options = options;
        field!.focus();
        if (currentIndex !== 0) {
          field!.openOverlay();
        }
        this.addValueChangedEventToField(currentIndex);
      }

      this._render();
    };
    this.callNextParamList(handler);
  }

  /**
   * Adds event listener to PickerFieldSk element that handles whenever the
   * selected value in a field has "changed".
   *
   * A value can only be considered "changed", when it's set to either
   * empty string or a valid value from the dropdown selection menu.
   *
   * @param index - index of the FieldInfo to add the event listener to.
   */
  private addValueChangedEventToField(index: number) {
    const fieldInfo = this._fieldData[index];

    fieldInfo.field!.comboBox!.addEventListener('value-changed', (e) => {
      const value = (e as CustomEvent).detail.value;
      fieldInfo.value = value;

      // Remove any child fields, as their values are no longer valid.
      // If the value of a parent changes, the child values need to be
      // recalculated.
      this.removeChildFields(index);

      // If the new value is not empty, there's two scenarios:
      // 1. If this is not the last param, add a new child field as
      //    there's still more values to choose from.
      // 2. If this is the last param, we are done selecting values.
      //    Just update the match count to reflect the new selection.
      if (value !== '') {
        if (index !== this._fieldData.length - 1) {
          this.addChildField();
        } else {
          this.fetchCount();
        }
        // If new value is empty, simply re-calculate the field options and
        // update the count.
      } else {
        this.fetchOptions(index);
      }
    });
  }

  /**
   * Removes child fields, given an index.
   *
   * For example, if the params are:
   * ['benchmark', 'bot', 'test', 'subtest_1']
   *
   * 'benchmark' has 'bot' as a child, which has 'test' as a child,
   * and so on.
   *
   * Given index 0 for 'benchmark' param, this function will remove
   * 'bot', 'test', and 'subtest_1' fields if they exist.
   *
   * @param index
   */
  private removeChildFields(index: number) {
    while (this._currentIndex - 1 > index) {
      const fieldInfo = this._fieldData[this._currentIndex - 1];
      fieldInfo.value = '';
      this._containerDiv!.removeChild(fieldInfo.field!);
      fieldInfo.field = null;
      this._currentIndex -= 1;
    }
    this._render();
  }

  /**
   * Sets the readonly property for all rendered fields.
   *
   * @param readonly
   */
  private setReadOnly(readonly: boolean) {
    this._fieldData.forEach((field) => {
      if (readonly) {
        field.field?.disable();
      } else {
        field.field?.enable();
      }
    });
  }

  /**
   * Wrapper for POST Call to backend.
   *
   * @param handler
   */
  private callNextParamList(
    handler: (json: NextParamListHandlerResponse) => void
  ) {
    this.updateCount(-1);
    this._requestInProgress = true;
    this.setReadOnly(true);
    this._render();

    const body: NextParamListHandlerRequest = {
      q: this.createQueryFromFieldData(),
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
        this.setReadOnly(false);
        handler(json);
      })
      .catch((msg: any) => {
        this._requestInProgress = false;
        this.setReadOnly(false);
        this._render();
        errorMessage(msg);
      });
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
  private fetchOptions(index: number) {
    const fieldInfo = this._fieldData[index];
    const field = fieldInfo.field;
    const param = field!.label;

    const handler = (json: NextParamListHandlerResponse) => {
      if (param in json.paramset && json.paramset[param] !== null) {
        const options = json.paramset[field!.label];
        field!.options = options;
        this.updateCount(json.count);
        field!.focus();
        if (index !== 0) {
          field!.openOverlay();
        }
        this._render();
      }
    };
    this.callNextParamList(handler);
  }

  /**
   * Update the matches count.
   *
   * Calls '/_/nextParamList/' to calculate how many matches the current
   * selection has.
   */
  private fetchCount() {
    const body: NextParamListHandlerRequest = {
      q: this.createQueryFromFieldData(),
    };

    const handler = (json: NextParamListHandlerResponse) => {
      this._requestInProgress = false;
      this.updateCount(json.count);
      this._render();
    };

    this.callNextParamList(handler);
  }

  private onPlotButtonClick() {
    const detail = {
      query: this.createQueryFromFieldData(),
    };
    this.dispatchEvent(
      new CustomEvent('plot-button-clicked', { detail: detail, bubbles: true })
    );
  }

  /**
   * Reset test picker and populate the fields with an input query.
   * e.g.:
   * populateFieldDataFromQuery(
   *    'benchmark=a&bot=b&test=c&subtest1=&subtest2=d',
   *    ['benchmark', 'bot', 'test', 'subtest1', 'subtest2']
   * )
   *
   * In the above case, fields will be filled in order of hierarchy until an
   * empty value is reached. Since subtest1 is empty, it'll stop filling at
   * subtest1, leaving subtest1 and subtest2 fields empty.
   *
   * As another example the query 'bot=b&test=c&subtest1=&subtest2=d', is
   * not valid as it's missing the benchmark key.
   *
   * Note that calling this function will overwrite any current selections
   * in the test picker.
   *
   * @param query
   * @param params
   */
  populateFieldDataFromQuery(query: string, params: string[]) {
    const paramSet = toParamSet(query);

    this.initializeFieldData(params);

    for (let i = 0; i < this._fieldData.length; i++) {
      const fieldInfo = this._fieldData[i];
      const param = fieldInfo.param;

      if (!(param in paramSet)) {
        break;
      }

      const field: PickerFieldSk = new PickerFieldSk(param);
      fieldInfo.field = field;
      this._containerDiv!.appendChild(field);
      this._currentIndex += 1;

      if (paramSet[param][0] === '') {
        this.addValueChangedEventToField(i);
        this.fetchOptions(i);
        break;
      }

      fieldInfo.value = paramSet[param][0];
      field.options = [fieldInfo.value];
      field.setValue(fieldInfo.value);

      this.addValueChangedEventToField(i);
    }

    this.fetchCount();
  }

  /**
   * Reads the values currently selected and transforms them to
   * query format. Add default values from _defaultParams.
   *
   * This is necessary to make calls to /_/nextParamList/.
   *
   * @returns value selection in query format.
   */
  createQueryFromFieldData(): string {
    const paramSet: ParamSet = {};
    this._fieldData.forEach((fieldInfo) => {
      if (fieldInfo.value !== '') {
        paramSet[fieldInfo.param] = [fieldInfo.value];
      }
    });

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
   * Updates the count and updates the Plot button based on the count.
   *
   * -1 is a valid value and it sets the count to be empty string.
   * This is useful for when we want to display the spinning wheel
   * instead, when the count is still being calculated.
   *
   * Also, enables plotting based on the count. If the count is
   * PLOT_MAXIMUM or 0, user is not able to plot.
   *
   * @param count
   */
  private updateCount(count: number) {
    if (count === -1) {
      this._count = '';
      this._plotButton!.disabled = true;
      return;
    }

    this._count = `${count}`;
    if (count > PLOT_MAXIMUM || count <= 0) {
      this._plotButton!.disabled = true;
    } else {
      this._plotButton!.disabled = false;
    }
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
    defaultParams: { [key: string]: string[] | null }
  ) {
    this.initializeFieldData(params);
    this._defaultParams = defaultParams;
    this.addChildField();
    this._render();
  }

  /**
   * Resets data structures from scratch.
   *
   * Clears the field container DOM, resets the index,
   * resets the fieldData structure, and initializes fieldData
   * based on given params.
   *
   * @param params
   */
  private initializeFieldData(params: string[]) {
    this._containerDiv!.replaceChildren();
    this._currentIndex = 0;
    this._fieldData = [];

    params.forEach((param) => {
      this._fieldData.push({
        field: null,
        param: param,
        value: '',
      });
    });
  }
}

define('test-picker-sk', TestPickerSk);
