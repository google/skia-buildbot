import './index';

import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { FilterDialogSk, Filters } from './filter-dialog-sk';
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

  const filters: Filters = {
    diffConfig: {
      'car make': ['chevrolet', 'dodge', 'ford'],
      'color': ['blue'],
      'year': ['2020', '2019']
    },
    minRGBADelta: 0,
    maxRGBADelta: 255,
    sortOrder: 'descending',
    mustHaveReferenceImage: false
  };

  const differentFilters: Filters = {
    diffConfig: {
      'color': ['green'],
      'used': ['yes', 'no'],
    },
    minRGBADelta: 50,
    maxRGBADelta: 100,
    sortOrder: 'ascending',
    mustHaveReferenceImage: true
  };

  it('should open the dialog with the given filters', () => {
    expect(isDialogOpen()).to.be.false;
    filterDialogSk.open(paramSet, filters);
    expect(isDialogOpen()).to.be.true;
    expect(getSelectedFilters()).to.deep.equal(filters);
  });

  it('should be possible to reopen the dialog with different filters', () => {
    filterDialogSk.open(paramSet, filters);
    clickCancelBtn();

    filterDialogSk.open(paramSet, differentFilters);
    expect(getSelectedFilters()).to.deep.equal(differentFilters);
  });

  describe('filter button', () => {
    it('closes the dialog', () => {
      filterDialogSk.open(paramSet, filters);
      clickFilterBtn();
      expect(isDialogOpen()).to.be.false;
    })

    it('returns unmodified filters via the "edit" event if the user made no changes', async () => {
      filterDialogSk.open(paramSet, filters);

      const editEvent = eventPromise<CustomEvent<Filters>>('edit');
      clickFilterBtn();
      const newFilters = (await editEvent).detail;

      expect(newFilters).to.deep.equal(filters);
    });

    it('returns the new filters via the "edit" event if the user made some changes', async () => {
      filterDialogSk.open(paramSet, filters);
      setDifferentFiltersViaUI();

      const editEvent = eventPromise<CustomEvent<Filters>>('edit');
      clickFilterBtn();
      const newFilters = (await editEvent).detail;

      expect(newFilters).to.deep.equal(differentFilters);
    });
  });

  describe('cancel button', () => {
    it('closes the dialog without emitting the "edit" event', async () => {
      filterDialogSk.open(paramSet, filters);

      const noEvent = noEventPromise('edit');
      clickCancelBtn();
      await noEvent;

      expect(isDialogOpen()).to.be.false;
    });

    it('discards any changes when reopened with same filters', async () => {
      filterDialogSk.open(paramSet, filters);
      setDifferentFiltersViaUI();
      clickCancelBtn();

      filterDialogSk.open(paramSet, filters);
      expect(getSelectedFilters()).to.deep.equal(filters);
    });
  });

  // Clicks the "Filter" button. This closes the dialog.
  const clickFilterBtn =
    () => $$<HTMLButtonElement>('.filter-dialog > .buttons .filter', filterDialogSk)!.click();

  // Clicks the "Cancel" button. This closes the dialog.
  const clickCancelBtn =
    () => $$<HTMLButtonElement>('.filter-dialog > .buttons .cancel', filterDialogSk)!.click();

  const isDialogOpen = () => $$<HTMLDialogElement>('dialog.filter-dialog', filterDialogSk)!.open;

  // Extracts the Filter currently displayed by looking at the component's UI.
  const getSelectedFilters = (): Filters => ({
    diffConfig: getRightHandQuery(),
    minRGBADelta: parseFloat($$<HTMLInputElement>('#min-rgba-delta', filterDialogSk)!.value),
    maxRGBADelta: parseFloat($$<HTMLInputElement>('#max-rgba-delta', filterDialogSk)!.value),
    sortOrder: $$<HTMLSelectElement>('#sort-order', filterDialogSk)!.value as
               'ascending' | 'descending',
    mustHaveReferenceImage: $$<CheckOrRadio>('#must-have-reference-image')!.checked
  });

  // Returns the ParamSet displayed by the paramset-sk component.
  const getRightHandQuery = (): ParamSet => {
    const paramSet: ParamSet = {};
    $('trace-filter-sk .selection paramset-sk tr', filterDialogSk).forEach((tr, i) => {
      if (i === 0) return; // Skip the first row, which usually displays titles (empty in our case).
      const key = $$('th', tr)!.textContent!;
      const values = $('div', tr).map(div => div.textContent!);
      paramSet[key] = values;
    })
    return paramSet;
  };

  // Sets dialog's filters to match differentFilters via simulated UI interactions.
  const setDifferentFiltersViaUI = () => {
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

    // These input/select elements update the dialog's Filters object on input/change events.
    // Setting these elements' values programmatically does not trigger said events, so we need to
    // dispatch them explicitly.
    setValueAndDispatchEvent('#min-rgba-delta', '50', 'input');
    setValueAndDispatchEvent('#max-rgba-delta', '100', 'input');
    setValueAndDispatchEvent('#sort-order', 'ascending', 'change');

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
