import './index';

import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { FilterDialogSk, FilterDialogSkValue } from './filter-dialog-sk';
import { ParamSet } from 'common-sk/modules/query';
import { $, $$ } from 'common-sk/modules/dom';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

const expect = chai.expect;

describe('filter-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<FilterDialogSk>('filter-dialog-sk');

  let filterDialogSk: FilterDialogSk;
  beforeEach(() => {
    filterDialogSk = newInstance();
  });

  const paramSet: ParamSet = {
    'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
    'color': ['blue', 'green', 'red'],
    'used': ['yes', 'no'],
    'year': ['2020', '2019', '2018', '2017', '2016', '2015']
  };

  const value: FilterDialogSkValue = {
    diffConfig: {
      'car make': ['chevrolet', 'dodge', 'ford'],
      'color': ['blue'],
      'year': ['2020', '2019']
    },
    minRGBADelta: 0,
    maxRGBADelta: 255,
    maxDiff: -1,
    metric: 'combined',
    sortOrder: 'descending',
    mustHaveReferenceImage: false
  };

  const differentValue: FilterDialogSkValue = {
    diffConfig: {
      'color': ['green'],
      'used': ['yes', 'no'],
    },
    minRGBADelta: 50,
    maxRGBADelta: 100,
    maxDiff: 0,
    metric: 'percent',
    sortOrder: 'ascending',
    mustHaveReferenceImage: true
  };

  it('should open the dialog with the given values', () => {
    expect(isDialogOpen()).to.be.false;
    filterDialogSk.open(paramSet, value);
    expect(isDialogOpen()).to.be.true;
    expect(getDisplayedValue()).to.deep.equal(value);
  });

  it('should be possible to reopen the dialog with a different value', () => {
    filterDialogSk.open(paramSet, value);
    clickCancelBtn();

    filterDialogSk.open(paramSet, differentValue);
    expect(getDisplayedValue()).to.deep.equal(differentValue);
  });

  describe('filter button', () => {
    it('closes the dialog', () => {
      filterDialogSk.open(paramSet, value);
      clickFilterBtn();
      expect(isDialogOpen()).to.be.false;
    })

    it('returns unmodified value via the "edit" event when clicked with no changes', async () => {
      filterDialogSk.open(paramSet, value);

      const editEvent = eventPromise<CustomEvent<FilterDialogSkValue>>('edit');
      clickFilterBtn();
      const newValue = (await editEvent).detail;

      expect(newValue).to.deep.equal(value);
    });

    it('returns new value via the "edit" event when clicked with some changes', async () => {
      filterDialogSk.open(paramSet, value);
      setDifferentValue();

      const editEvent = eventPromise<CustomEvent<FilterDialogSkValue>>('edit');
      clickFilterBtn();
      const newValue = (await editEvent).detail;

      expect(newValue).to.deep.equal(differentValue);
    });
  });

  it('closes the dialog without emitting the "edit" event when "cancel" is clicked', async () => {
    filterDialogSk.open(paramSet, value);

    const noEvent = noEventPromise('edit');
    clickCancelBtn();
    await noEvent;

    expect(isDialogOpen()).to.be.false;
  });

  // Clicks the "Filter" button. This closes the dialog.
  const clickFilterBtn =
    () => $$<HTMLButtonElement>('.filter-dialog button.filter', filterDialogSk)!.click();

  // Clicks the "Cancel" button. This closes the dialog.
  const clickCancelBtn =
    () => $$<HTMLButtonElement>('.filter-dialog button.cancel', filterDialogSk)!.click();

  const isDialogOpen = () => $$<HTMLDialogElement>('dialog.filter-dialog', filterDialogSk)!.open;

  // Extracts the FilterDialogSkValue currently displayed by looking at the component's UI.
  const getDisplayedValue = (): FilterDialogSkValue => ({
    diffConfig: getRightHandQuery(),
    minRGBADelta: parseFloat($$<HTMLInputElement>('#min-rgba-delta', filterDialogSk)!.value),
    maxRGBADelta: parseFloat($$<HTMLInputElement>('#max-rgba-delta', filterDialogSk)!.value),
    maxDiff: parseFloat($$<HTMLInputElement>('#max-diff', filterDialogSk)!.value),
    metric: $$<HTMLSelectElement>('#metric', filterDialogSk)!.value as
            'pixel' | 'percent' | 'combined',
    sortOrder: $$<HTMLSelectElement>('#sort-by', filterDialogSk)!.value as
               'ascending' | 'descending',
    mustHaveReferenceImage: $$<CheckOrRadio>('#must-have-reference-image')!.checked
  });

  // Returns the ParamSet displayed by the paramset-sk component.
  const getRightHandQuery = (): ParamSet => {
    const paramSet: ParamSet = {};
    $('paramset-sk tr', filterDialogSk).forEach((tr, i) => {
      if (i === 0) return; // Skip the first row, which usually displays titles (empty in our case).
      const key = $$('th', tr)!.textContent!;
      const values = $('div', tr).map(div => div.textContent!);
      paramSet[key] = values;
    })
    return paramSet;
  };

  // Sets dialog's value to match differentValue via simulated UI interactions.
  const setDifferentValue = () => {
    // Set right hand query.
    $$<HTMLButtonElement>('.edit-query', filterDialogSk)!.click();
    const queryDialogSk = $$('query-dialog-sk', filterDialogSk)!;
    $$<HTMLButtonElement>('.clear_selections', queryDialogSk)!.click();
    $$<HTMLDivElement>('select-sk div:nth-child(2)', queryDialogSk)!.click(); // Color.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // Green.
    $$<HTMLDivElement>('select-sk div:nth-child(3)', queryDialogSk)!.click(); // Used.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(1)', queryDialogSk)!.click(); // Yes.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // No.
    $$<HTMLButtonElement>('.show-matches', queryDialogSk)!.click();

    setValueAndDispatchEvent('#min-rgba-delta', '50', 'input');
    setValueAndDispatchEvent('#max-rgba-delta', '100', 'input');
    setValueAndDispatchEvent('#max-diff', '0', 'input');
    setValueAndDispatchEvent('#metric', 'percent', 'change');
    setValueAndDispatchEvent('#sort-by', 'ascending', 'change');

    const mustHaveReferenceImageCheckBox =
      $$<CheckOrRadio>('#must-have-reference-image', filterDialogSk)!;
    mustHaveReferenceImageCheckBox.checked = true;
    mustHaveReferenceImageCheckBox.dispatchEvent(new Event('change', {bubbles: true}));
  };

  const setValueAndDispatchEvent = (elementId: string, value: string, event: string) => {
    const element = $$<HTMLInputElement | HTMLSelectElement>(elementId, filterDialogSk)!;
    element.value = value;
    element.dispatchEvent(new Event(event, {bubbles: true}));
  }
});
