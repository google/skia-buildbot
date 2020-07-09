import './index';
import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { FilterDialogSk, Filters } from './filter-dialog-sk';
import { FilterDialogSkPO } from './filter-dialog-sk_po';
import { ParamSet } from 'common-sk/modules/query';

const expect = chai.expect;

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

describe('filter-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<FilterDialogSk>('filter-dialog-sk');

  let filterDialogSk: FilterDialogSk;
  let filterDialogSkPO: FilterDialogSkPO;

  beforeEach(() => {
    filterDialogSk = newInstance();
    filterDialogSkPO = new FilterDialogSkPO(filterDialogSk);
  });

  it('should open the dialog with the given filters', async () => {
    expect(await filterDialogSkPO.isDialogOpen()).to.be.false;
    filterDialogSk.open(paramSet, filters);
    expect(await filterDialogSkPO.isDialogOpen()).to.be.true;
    expect(await filterDialogSkPO.getSelectedFilters()).to.deep.equal(filters);
  });

  it('should be possible to reopen the dialog with different filters', async () => {
    filterDialogSk.open(paramSet, filters);
    await filterDialogSkPO.clickCancelBtn();

    filterDialogSk.open(paramSet, differentFilters);
    expect(await filterDialogSkPO.getSelectedFilters()).to.deep.equal(differentFilters);
  });

  describe('filter button', () => {
    it('closes the dialog', async () => {
      filterDialogSk.open(paramSet, filters);
      await filterDialogSkPO.clickFilterBtn();
      expect(await filterDialogSkPO.isDialogOpen()).to.be.false;
    })

    it('returns unmodified filters via the "edit" event if the user made no changes', async () => {
      filterDialogSk.open(paramSet, filters);

      const editEvent = eventPromise<CustomEvent<Filters>>('edit');
      await filterDialogSkPO.clickFilterBtn();
      const newFilters = (await editEvent).detail;

      expect(newFilters).to.deep.equal(filters);
    });

    it('returns the new filters via the "edit" event if the user made some changes', async () => {
      filterDialogSk.open(paramSet, filters);
      await filterDialogSkPO.setSelectedFilters(differentFilters);

      const editEvent = eventPromise<CustomEvent<Filters>>('edit');
      await filterDialogSkPO.clickFilterBtn();
      const newFilters = (await editEvent).detail;

      expect(newFilters).to.deep.equal(differentFilters);
    });
  });

  describe('cancel button', () => {
    it('closes the dialog without emitting the "edit" event', async () => {
      filterDialogSk.open(paramSet, filters);

      const noEvent = noEventPromise('edit');
      await filterDialogSkPO.clickCancelBtn();
      await noEvent;

      expect(await filterDialogSkPO.isDialogOpen()).to.be.false;
    });

    it('discards any changes when reopened with same filters', async () => {
      filterDialogSk.open(paramSet, filters);
      await filterDialogSkPO.setSelectedFilters(differentFilters);
      await filterDialogSkPO.clickCancelBtn();

      filterDialogSk.open(paramSet, filters);
      expect(await filterDialogSkPO.getSelectedFilters()).to.deep.equal(filters);
    });
  });
});
