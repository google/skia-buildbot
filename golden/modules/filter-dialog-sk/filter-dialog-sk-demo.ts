import './index';
import { $$ } from 'common-sk/modules/dom';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';
import { deepCopy } from 'common-sk/modules/object';
import { FilterDialogSk, Filters } from './filter-dialog-sk';

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  color: ['blue', 'green', 'red'],
  used: ['yes', 'no'],
  year: ['2020', '2019', '2018', '2017', '2016', '2015'],
};

// We keep a copy of the initial value so that the "Reset values" button works.
const defaultValue: Filters = {
  diffConfig: {
    'car make': ['chevrolet', 'dodge', 'ford'],
    color: ['blue'],
    year: ['2020', '2019'],
  },
  minRGBADelta: 0,
  maxRGBADelta: 255,
  sortOrder: 'descending',
  mustHaveReferenceImage: false,
};

// This will be updated by the dialog, or by the "Reset values" button.
let currentValue = deepCopy(defaultValue);

// Updates the "Values" section of the demo page.
function updateValues() {
  $$<HTMLSpanElement>('.diff-config')!.innerText = fromParamSet(currentValue.diffConfig);
  $$<HTMLSpanElement>('.min-rgba-delta')!.innerText = currentValue.minRGBADelta.toString();
  $$<HTMLSpanElement>('.max-rgba-delta')!.innerText = currentValue.maxRGBADelta.toString();
  $$<HTMLSpanElement>('.sort-order')!.innerText = currentValue.sortOrder.toString();
  $$<HTMLSpanElement>('.must-have-ref-img')!.innerText = currentValue.mustHaveReferenceImage.toString();
}

const filterDialogSk = new FilterDialogSk();
$$('body')?.appendChild(filterDialogSk);

filterDialogSk.addEventListener('edit', (e: Event) => {
  const value: Filters = (e as CustomEvent<Filters>).detail;
  currentValue = value;
  updateValues();
});

$$<HTMLButtonElement>('#show-dialog')!.addEventListener('click', () => {
  filterDialogSk.open(paramSet, currentValue);
});

$$<HTMLButtonElement>('#reset-values')!.addEventListener('click', () => {
  currentValue = deepCopy(defaultValue);
  updateValues();
});
