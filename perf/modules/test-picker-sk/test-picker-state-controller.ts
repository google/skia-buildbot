import { ReactiveController, ReactiveControllerHost } from 'lit';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ParamSet, fromParamSet, toParamSet } from '../../../infra-sk/modules/query';
import { NextParamListHandlerRequest, NextParamListHandlerResponse } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';

import { MISSING_VALUE_SENTINEL } from '../const/const';
import { DEFAULT_OPTION_LABEL } from '../common/test-picker';
export const PLOT_MAXIMUM: number = 200;
export const MAX_MESSAGE = 'Reduce Traces';

export interface FieldInfo {
  param: string;
  value: string[] | null;
  options: string[];
  splitBy: string[];
  index: number;
  split: boolean;
  splitDisabled: boolean;
  disabled: boolean;
  selectedItems: string[];
}

export interface TestPickerStateControllerHost extends ReactiveControllerHost, HTMLElement {
  dispatchEvent(event: Event): boolean;
  setFieldPendingFocus(param: string): void;
  setFieldPendingOpenOverlay(param: string): void;
  requestUpdate(): void;
  handleValueChangeForField(
    index: number,
    value: string[],
    checkboxSelected: boolean,
    hasGraphLoaded?: boolean
  ): Promise<void>;
}

export class TestPickerStateController implements ReactiveController {
  private host: TestPickerStateControllerHost;

  fieldData: FieldInfo[] = [];

  count: number = -1;

  requestInProgress: boolean = false;

  currentIndex: number = 0;

  defaultParams: { [key: string]: string[] | null } = {};

  autoAddTrace: boolean = false;

  readOnly: boolean = false;

  dataLoading: boolean = false;

  forceManualPlot: boolean = false;

  initialParams: string[] = [];

  constructor(host: TestPickerStateControllerHost) {
    (this.host = host).addController(this);
  }

  hostConnected() {}

  /**
   * Initializes Test Picker from scratch.
   */
  async initializeTestPicker(
    params: string[],
    defaultParams: { [key: string]: string[] | null },
    readOnly: boolean,
    forceManualPlot: boolean = false
  ): Promise<void> {
    this.initialParams = params;
    this.forceManualPlot = forceManualPlot;
    this.defaultParams = defaultParams;
    this.initializeFieldData(params);
    await this.addChildField(readOnly);
  }

  /**
   * Resets data structures from scratch.
   */
  private initializeFieldData(params: string[]) {
    this.currentIndex = 0;
    this.fieldData = params.map((param, i) => ({
      param: param,
      value: i === 0 ? [] : null,
      options: [],
      selectedItems: [],
      split: false,
      splitDisabled: false,
      disabled: false,
      splitBy: [],
      index: i,
    }));
    this.host.requestUpdate();
  }

  /**
   * Generates a query string from the currently selected field values.
   */
  createQueryFromFieldData(): string {
    return fromParamSet(this.createParamSetFromFieldData());
  }

  createParamSetFromFieldData(): ParamSet {
    const paramSet: ParamSet = {};
    if (this.fieldData.length === 0 || this.fieldData[0].value === null) {
      return {};
    }

    this.fieldData.forEach((fieldInfo) => {
      if (fieldInfo.value !== null) {
        paramSet[fieldInfo.param] = fieldInfo.value.map((v) =>
          v === DEFAULT_OPTION_LABEL ? MISSING_VALUE_SENTINEL : v
        );
      }
    });

    if (Object.keys(paramSet).length === 0) {
      return {};
    }
    if ((this.fieldData[0].value || []).length === 0) {
      return {};
    }
    for (const defaultParamKey in this.defaultParams) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = this.defaultParams[defaultParamKey]!;
      }
    }

    return paramSet;
  }

  createQueryFromIndex(index: number): string {
    const paramSet: ParamSet = {};
    for (let i = 0; i <= index; i++) {
      const fieldInfo = this.fieldData[i];
      if (fieldInfo && fieldInfo.value !== null) {
        paramSet[fieldInfo.param] = fieldInfo.value.map((v) =>
          v === DEFAULT_OPTION_LABEL ? MISSING_VALUE_SENTINEL : v
        );
      }
    }
    if (Object.keys(paramSet).length === 0) {
      return '';
    }
    if ((this.fieldData[0]?.value || []).length === 0) {
      return '';
    }
    for (const defaultParamKey in this.defaultParams) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = this.defaultParams[defaultParamKey]!;
      }
    }
    return fromParamSet(paramSet);
  }

  setReadOnly(readonly: boolean, overrideDataLoadingCheck = false) {
    if (this.readOnly === readonly) {
      return;
    }
    const exploreMulti = document.querySelector('explore-multi-sk') as any;
    if (exploreMulti && exploreMulti._dataLoading && !overrideDataLoadingCheck) {
      readonly = true;
    }
    this.dataLoading = exploreMulti?._dataLoading || false;
    this.readOnly = readonly;
    this.fieldData = this.fieldData.map((field) => ({
      ...field,
      disabled: readonly,
    }));

    this.host.requestUpdate();
  }

  updateCount(count: number, hasGraphLoaded: boolean = false) {
    if (count === -1) {
      if (this.currentIndex > 0) {
        this.setReadOnly(true);
      } else {
        this.setReadOnly(false);
      }
      this.count = 0;
      this.host.requestUpdate();
      return;
    }

    this.setReadOnly(false);
    this.count = count;
    if (count > PLOT_MAXIMUM || count <= 0) {
      this.autoAddTrace = false;
      this.host.requestUpdate();
      return;
    }
    if (hasGraphLoaded && !this.forceManualPlot) {
      this.autoAddTrace = true;
    } else {
      this.autoAddTrace = false;
    }
    this.host.requestUpdate();
  }

  async callNextParamList(
    handler: (json: NextParamListHandlerResponse) => void,
    index: number,
    hasGraphLoaded: boolean = false
  ): Promise<void> {
    this.updateCount(-1, hasGraphLoaded);
    this.requestInProgress = true;
    if (!((this.fieldData[index]?.value || []).length > 1)) {
      this.setReadOnly(true);
    }

    const fieldDataQuery = this.createQueryFromIndex(index);
    const body: NextParamListHandlerRequest = { q: fieldDataQuery };

    try {
      const response = await fetch('/_/nextParamList/', {
        method: 'POST',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const json = await jsonOrThrow(response);
      this.requestInProgress = false;
      this.setReadOnly(false);
      await handler(json as NextParamListHandlerResponse);
    } catch (msg: any) {
      this.requestInProgress = false;
      this.setReadOnly(false);
      this.removeChildFields(index);
      errorMessage(msg);
    }
    this.host.requestUpdate();
  }

  async addChildField(readOnly: boolean, hasGraphLoaded: boolean = false): Promise<void> {
    const currentIndex = this.currentIndex;
    const currentFieldInfo = this.fieldData[currentIndex];
    const param = currentFieldInfo.param;

    const handler = async (json: NextParamListHandlerResponse) => {
      this.updateCount(json.count, hasGraphLoaded);

      if (param in json.paramset && json.paramset[param] !== null) {
        let options = json.paramset[param].filter((option: string) => !option.includes('.'));
        options = options.map((o) => (o === '' ? DEFAULT_OPTION_LABEL : o));
        options.sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }));

        const extraTests = json.paramset[param].filter((option: string) => option.includes('.'));
        if (extraTests.length > 0) {
          options = options.concat(extraTests);
        }

        this.fieldData[currentIndex].options = options;
        this.fieldData = [...this.fieldData];

        this.setReadOnly(readOnly);
        this.currentIndex += 1;

        this.host.setFieldPendingFocus(param);
        if (currentIndex !== 0) {
          this.host.setFieldPendingOpenOverlay(param);
        }

        const defaults = (document.querySelector('explore-multi-sk') as any)?.defaults;
        if (
          defaults?.default_trigger_priority &&
          defaults.default_trigger_priority[param] &&
          currentFieldInfo.value &&
          currentFieldInfo.value.length === 0
        ) {
          const priorityList = defaults.default_trigger_priority[param];
          for (const priorityVal of priorityList) {
            if (options.includes(priorityVal)) {
              await this.host.handleValueChangeForField(
                currentIndex,
                [priorityVal],
                false,
                hasGraphLoaded
              );
              break;
            }
          }
        }
      } else {
        this.currentIndex += 1;
      }
      this.host.requestUpdate();
    };
    return await this.callNextParamList(handler, currentIndex, hasGraphLoaded);
  }

  removeChildFields(index: number) {
    while (this.currentIndex > index + 1) {
      this.currentIndex -= 1;
      if (this.currentIndex < this.fieldData.length) {
        const fieldInfo = this.fieldData[this.currentIndex];
        if (fieldInfo.splitBy.length > 0) {
          fieldInfo.split = false;
          this.host.dispatchEvent(
            new CustomEvent('split-by-changed', {
              detail: { param: fieldInfo.param, split: false },
              bubbles: true,
              composed: true,
            })
          );
        }
        fieldInfo.value = null;
        fieldInfo.selectedItems = [];
        fieldInfo.options = [];
      }
    }
    if (this.fieldData[index]) {
      this.fieldData[index].value = [];
      this.fieldData[index].selectedItems = [];
    }
    this.fieldData = [...this.fieldData];
    this.host.requestUpdate();
  }

  setSplitFields(param: string, split: boolean) {
    if (split) {
      const alreadySplit = this.fieldData.find((f) => f.splitBy.length > 0 && f.param !== param);
      if (alreadySplit) {
        const currentField = this.fieldData.find((f) => f.param === param);
        if (currentField) {
          currentField.split = false;
        }
        this.fieldData = [...this.fieldData];
        this.host.requestUpdate();
        return;
      }
    }

    this.setReadOnly(true);
    this.fieldData = this.fieldData.map((field) => {
      if (field.param === param) {
        return {
          ...field,
          split,
          splitBy: split ? [param] : [],
        };
      } else {
        return {
          ...field,
          split: split ? false : field.split,
          splitDisabled: split,
        };
      }
    });
    this.host.requestUpdate();
  }

  async fetchExtraOptions(index: number, hasGraphLoaded: boolean = false): Promise<void> {
    const handler = async (json: NextParamListHandlerResponse) => {
      const param = Object.keys(json.paramset)[0];
      const count: number = json.count || -1;
      if (param !== undefined) {
        for (let i = 0; i < this.fieldData.length; i++) {
          const fieldInfo = this.fieldData[i];
          if (fieldInfo.param === param) {
            let options = json.paramset[param]
              .filter((option: string) => !option.includes('.'))
              .map((o) => (o === '' ? DEFAULT_OPTION_LABEL : o));

            const extraTests = json.paramset[param].filter((option: string) =>
              option.includes('.')
            );
            if (extraTests.length > 0) {
              options = options.concat(extraTests);
            }

            fieldInfo.options = options;
            fieldInfo.index = i;
            let isNewField = false;
            let valueChanged = false;
            if (fieldInfo.value === null) {
              isNewField = true;
              fieldInfo.value = [];
            } else {
              const validValues = fieldInfo.value.filter((v) => options.includes(v));
              if (validValues.length !== fieldInfo.value.length) {
                fieldInfo.value = validValues;
                fieldInfo.selectedItems = validValues;
                valueChanged = true;
              }
            }
            this.fieldData = [...this.fieldData];

            if (valueChanged && fieldInfo.value.length === 0) {
              this.removeChildFields(i);
            }

            this.host.setFieldPendingFocus(param);
            if (isNewField && i > 0) {
              this.host.setFieldPendingOpenOverlay(param);
            }

            const defaults = (document.querySelector('explore-multi-sk') as any)?.defaults;
            if (
              defaults?.default_trigger_priority &&
              defaults.default_trigger_priority[param] &&
              fieldInfo.value.length === 0
            ) {
              const priorityList = defaults.default_trigger_priority[param];
              for (const priorityVal of priorityList) {
                if (options.includes(priorityVal)) {
                  await this.host.handleValueChangeForField(
                    i,
                    [priorityVal],
                    false,
                    hasGraphLoaded
                  );
                  break;
                }
              }
            } else if (fieldInfo.value.length > 0) {
              await this.fetchExtraOptions(i, hasGraphLoaded);
            }

            if (this.currentIndex <= i + 1) {
              this.currentIndex = i + 1;
              this.updateCount(count, hasGraphLoaded);
            }
            break;
          }
        }
      } else {
        if (this.currentIndex <= this.fieldData.length) {
          this.currentIndex = this.fieldData.length;
          this.updateCount(count, hasGraphLoaded);
        }
      }
      this.host.requestUpdate();
    };
    return await this.callNextParamList(handler, index, hasGraphLoaded);
  }

  async populateFieldDataFromQuery(
    query: string,
    params: string[],
    paramSet: ParamSet,
    hasGraphLoaded: boolean = false
  ) {
    const selectedParams: ParamSet = toParamSet(query);
    if (paramSet && Object.keys(paramSet).length > 0) {
      const paramKeys: string[] = Object.keys(paramSet).filter((key) => key in selectedParams);
      this.initializeFieldData(paramKeys);
    } else {
      this.initializeFieldData(params);
    }
    for (let i = 0; i < this.fieldData.length; i++) {
      const fieldInfo = this.fieldData[i];
      const param = fieldInfo.param;

      let selectedValue = selectedParams[fieldInfo.param] || [];
      selectedValue = selectedValue.map((v) =>
        v === MISSING_VALUE_SENTINEL || v === '' ? DEFAULT_OPTION_LABEL : v
      );

      fieldInfo.selectedItems = selectedValue;
      fieldInfo.value = selectedValue;

      if (paramSet && paramSet[param]) {
        fieldInfo.options = paramSet[param];
      }
      fieldInfo.index = i;

      this.fieldData = [...this.fieldData];

      await this.fetchExtraOptions(i, hasGraphLoaded);
      this.host.setFieldPendingFocus(param);
    }
  }

  async populateFieldDataFromParamSet(
    paramSets: ParamSet,
    paramSet: ParamSet,
    splitByKeys: string[] = [],
    hasGraphLoaded: boolean = false
  ): Promise<void> {
    const uniqueParamKeys = [...new Set([...Object.keys(paramSets), ...Object.keys(paramSet)])];
    const filteredKeys = this.initialParams.filter((key) => uniqueParamKeys.includes(key));
    this.initializeFieldData(filteredKeys);
    this.currentIndex = 0;
    if (Object.keys(paramSet).length === 0 && !this.forceManualPlot) {
      this.autoAddTrace = true;
    }

    const promises: Promise<void>[] = [];
    for (let i = 0; i < this.fieldData.length; i++) {
      const fieldInfo = this.fieldData[i];
      const param = fieldInfo.param;

      if (splitByKeys.includes(param)) {
        fieldInfo.split = true;
        fieldInfo.splitBy = [param];
      }

      let value = paramSets[param] || [];
      value = value.map((v) =>
        v === MISSING_VALUE_SENTINEL || v === '' ? DEFAULT_OPTION_LABEL : v
      );
      const allOptions = [...new Set([...value, ...(paramSet[param] || [])])].sort();

      if (value.length === 0) {
        break;
      }
      if (allOptions.length > 0) {
        fieldInfo.options = allOptions;
        fieldInfo.selectedItems = value;
        fieldInfo.index = i;
        fieldInfo.value = value;
      }
      promises.push(this.fetchExtraOptions(i, hasGraphLoaded));
    }
    await Promise.all(promises);
  }

  isLoaded(): boolean {
    return (
      this.fieldData.length > 0 &&
      this.fieldData[0].selectedItems &&
      this.fieldData[0].selectedItems.length > 0
    );
  }
}
