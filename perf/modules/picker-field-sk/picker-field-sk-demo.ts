import './index';
import { PickerFieldSk } from './picker-field-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import {
  DEFAULT_LABEL,
  DEFAULT_OPTIONS,
  DEFAULT_SELECTED_ITEMS,
  NEW_LABEL,
  NEW_OPTIONS,
  NEW_SELECTED_ITEMS,
} from './test_data';

window.customElements.whenDefined('picker-field-sk').then(() => {
  document.querySelectorAll<PickerFieldSk>('picker-field-sk').forEach((ele) => {
    ele.label = DEFAULT_LABEL;
    ele.options = DEFAULT_OPTIONS;
  });
});

$$('#demo-focus1')?.addEventListener('click', () => {
  const ele = document.querySelector('picker-field-sk') as PickerFieldSk;
  ele.focus();
});

$$('#demo-focus2')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.label = DEFAULT_LABEL;
  ele.options = DEFAULT_OPTIONS;
  ele.selectedItems = DEFAULT_SELECTED_ITEMS;
  ele.focus();
});

$$('#demo-fill')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.label = NEW_LABEL;
  ele.options = NEW_OPTIONS;
  ele.selectedItems = NEW_SELECTED_ITEMS;
  ele.setValue('speedometer3');
});

$$('#demo-open')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.openOverlay();
});

$$('#demo-disable')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.disable();
});

$$('#demo-enable')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.enable();
});
