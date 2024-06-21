import './index';
import { PickerFieldSk } from './picker-field-sk';
import { $$ } from '../../../infra-sk/modules/dom';

window.customElements.whenDefined('picker-field-sk').then(() => {
  document.querySelectorAll<PickerFieldSk>('picker-field-sk').forEach((ele) => {
    ele.label = 'Benchmark';
    ele.options = Array.from(
      { length: 3000 },
      (_, index) => `speedometer${index + 1}`
    );
  });
});

$$('#demo-focus1')?.addEventListener('click', () => {
  const ele = document.querySelector('picker-field-sk') as PickerFieldSk;
  ele.focus();
});

$$('#demo-focus2')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.focus();
});

$$('#demo-fill')?.addEventListener('click', () => {
  const ele = document.querySelector('#focus-and-fill') as PickerFieldSk;
  ele.setValue('speedometer223');
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
