import './index';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { SearchControlsSk, SearchCriteria } from './search-controls-sk';
import { SearchControlsSkPO } from './search-controls-sk_po';

const expect = chai.expect;

const corpora = ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'];

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};

// Takes a partial SearchCriteria and returns a full SearchCriteria by filling any missing fields
// with either zero values or sensible defaults.
const makeSearchCriteria = (partial: Partial<SearchCriteria> = {}) => {
  const defaults: SearchCriteria = {
    corpus: 'gm',
    leftHandTraceFilter: {},
    rightHandTraceFilter: {},
    includePositiveDigests: false,
    includeNegativeDigests: false,
    includeUntriagedDigests: false,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: false,
    minRGBADelta: 0,
    maxRGBADelta: 0,
    mustHaveReferenceImage: false,
    sortOrder: 'ascending'
  };
  return {...defaults, ...partial};
}

describe('search-controls-sk', () => {
  const newInstance = setUpElementUnderTest<SearchControlsSk>('search-controls-sk');

  let searchControlsSk: SearchControlsSk;
  let searchControlsSkPO: SearchControlsSkPO;

  beforeEach(() => {
    searchControlsSk = newInstance();
    searchControlsSk.corpora = corpora;
    searchControlsSk.paramSet = paramSet;
    searchControlsSk.searchCriteria = makeSearchCriteria();

    searchControlsSkPO = new SearchControlsSkPO(searchControlsSk);
  });

  it('shows the initial value', async () => {
    expect(await searchControlsSkPO.getSearchCriteria()).to.deep.equal(makeSearchCriteria());
  });

  const describeField = (name: string, partialSearchCriteria: Partial<SearchCriteria>) => {
    const searchCriteria = makeSearchCriteria(partialSearchCriteria);

    describe(name, () => {
      it('can change programmatically', async () => {
        searchControlsSk.searchCriteria = searchCriteria;
        expect(await searchControlsSkPO.getSearchCriteria()).to.deep.equal(searchCriteria);
      });

      it('can change via the UI', async () => {
        await searchControlsSkPO.setSearchCriteria(searchCriteria);
        expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
      });

      it('emits event "search-controls-sk-change" when it changes via the UI', async () => {
        const event = eventPromise<CustomEvent<SearchCriteria>>('search-controls-sk-change');
        await searchControlsSkPO.setSearchCriteria(searchCriteria);
        expect((await event).detail).to.deep.equal(searchCriteria);
      });
    });
  };

  describeField('corpus', {corpus: 'image'});

  describeField('include positive digests', {includePositiveDigests: true});

  describeField('include negative digests', {includeNegativeDigests: true});

  describeField('include untriaged digests', {includeUntriagedDigests: true});

  describeField('include digests not at head', {includeDigestsNotAtHead: true});

  describeField('include ignored digests', {includeIgnoredDigests: true});

  describeField('left-hand trace filter', {leftHandTraceFilter: {'car make': ['ford']}});

  describeField('right-hand trace filter', {rightHandTraceFilter: {'car make': ['ford']}});

  describeField('min RGBA delta', {minRGBADelta: 100}); // Arbitrary; valid range is (0 to 255).

  describeField('max RGBA delta', {maxRGBADelta: 100}); // Arbitrary; valid range is (0 to 255).

  describeField('sort order', {sortOrder: 'descending'});

  describeField('must have a reference image', {mustHaveReferenceImage: true});
});
