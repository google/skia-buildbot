import './index';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { SearchControlsSk, SearchCriteria, SearchCriteriaHintableObject, SearchCriteriaFromHintableObject, SearchCriteriaToHintableObject } from './search-controls-sk';
import { SearchControlsSkPO } from './search-controls-sk_po';
import { testOnlySetSettings } from '../settings';
import * as query from 'common-sk/modules/query';
import { HintableObject } from "common-sk/modules/hintable";

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

  const fieldCanChangeProgrammaticallyAndViaTheUI =
      (partialSearchCriteria: Partial<SearchCriteria>) => {
    const searchCriteria = makeSearchCriteria(partialSearchCriteria);

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
  }

  describe('corpus', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({corpus: 'image'});
  });

  describe('include positive digests', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({includePositiveDigests: true});
  });

  describe('include negative digests', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({includeNegativeDigests: true});
  });

  describe('include untriaged digests', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({includeUntriagedDigests: true});
  });

  describe('include digests not at head', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({includeDigestsNotAtHead: true});
  });

  describe('include ignored digests', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({includeIgnoredDigests: true});
  });

  describe('left-hand trace filter', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({leftHandTraceFilter: {'car make': ['ford']}});
  });

  describe('right-hand trace filter', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({rightHandTraceFilter: {'car make': ['ford']}});
  });

  describe('min RGBA delta', () => {
    // Arbitrary value within the valid range of (0 to 255).
    fieldCanChangeProgrammaticallyAndViaTheUI({minRGBADelta: 100});
  });

  describe('max RGBA delta', () => {
    // Arbitrary value within the valid range of (0 to 255).
    fieldCanChangeProgrammaticallyAndViaTheUI({maxRGBADelta: 100});
  });

  describe('sort order', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({sortOrder: 'descending'});
  });

  describe('must have a reference image', () => {
    fieldCanChangeProgrammaticallyAndViaTheUI({mustHaveReferenceImage: true});
  });
});

describe('SearchCriteriaHintableObject helpers', () => {
  before(() => {
    testOnlySetSettings({defaultCorpus: 'the_default_corpus'});
  });

  after(() => {
    testOnlySetSettings({});
  });

  describe('SearchCriteriaToHintableObject', () => {
    it('returns a HintableObject with default values given an empty SearchCriteria', () => {
      expect(SearchCriteriaToHintableObject({})).to.deep.equal(hintableObjectWithDefaultValues);
    });

    it('returns a HintableObject with the expected values given a fully populated SearchCriteria',
        () => {
      expect(SearchCriteriaToHintableObject(fullyPopulatedSearchCriteria))
        .to.deep.equal(fullyPopulatedHintableObject);
    });

    describe('SearchCriteria to query string end-to-end tests', () => {
      it('converts an empty SearchCriteria to the expected query string', () => {
        expect(searchCriteriaToQueryString({})).to.equal(queryStringWithDefaultValues);
      });

      it('converts a fully populated SearchCriteria to the expected query string', () => {
        expect(searchCriteriaToQueryString(fullyPopulatedSearchCriteria))
          .to.equal(fullyPopulatedQueryString);
      });

      const searchCriteriaToQueryString = (searchCriteria: Partial<SearchCriteria>) => {
        const hintableObject = SearchCriteriaToHintableObject(searchCriteria);
        return query.fromObject(hintableObject as HintableObject);
      };
    });
  });

  describe('SearchCriteriaFromHintableObject', () => {
    it('returns a SearchCriteria with default values given an empty HintableObject', () => {
      expect(SearchCriteriaFromHintableObject({})).to.deep.equal(searchCriteriaWithDefaultValues);
    });

    it('returns a SearchCriteria with the expected values given a fully populated HintableObject',
        () => {
      expect(SearchCriteriaFromHintableObject(fullyPopulatedHintableObject))
        .to.deep.equal(fullyPopulatedSearchCriteria);
    });

    describe('query string to SearchCriteria end-to-end tests', () => {
      it('converts an empty query string to the expected SearchCriteria', () => {
        expect(queryStringToSearchCriteria('')).to.deep.equal(searchCriteriaWithDefaultValues);
      });

      it('converts a query string with empty values to the expected SearchCriteria', () => {
        expect(queryStringToSearchCriteria(queryStringWithDefaultValues))
          .to.deep.equal(searchCriteriaWithDefaultValues);
      });

      it('converts a fully populated query string to the expected SearchCriteria', () => {
        expect(queryStringToSearchCriteria(fullyPopulatedQueryString))
          .to.deep.equal(fullyPopulatedSearchCriteria);
      });

      const queryStringToSearchCriteria = (queryString: string) => {
        const hintableObject =
          query.toObject(queryString, SearchCriteriaToHintableObject({}) as HintableObject);
        return SearchCriteriaFromHintableObject(hintableObject);
      };
    });
  });

  // Default values.

  const searchCriteriaWithDefaultValues: SearchCriteria = {
    corpus: 'the_default_corpus',
    leftHandTraceFilter: {},
    rightHandTraceFilter: {},
    includePositiveDigests: false,
    includeNegativeDigests: false,
    includeUntriagedDigests: false,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: false,
    minRGBADelta: 0,
    maxRGBADelta: 255,
    mustHaveReferenceImage: false,
    sortOrder: 'descending'
  };

  const hintableObjectWithDefaultValues: SearchCriteriaHintableObject = {
    corpus: '',
    left_filter: '',
    right_filter: '',
    positive: false,
    negative: false,
    untriaged: false,
    not_at_head: false,
    include_ignored: false,
    min_rgba: 0,
    max_rgba: 0,
    reference_image_required: false,
    sort: 'descending'
  };

  const queryStringWithDefaultValues =
    'corpus=&include_ignored=false&left_filter=&max_rgba=0&min_rgba=0&negative=false' +
    '&not_at_head=false&positive=false&reference_image_required=false&right_filter=' +
    '&sort=descending&untriaged=false';

  // Fully populated values.

  const fullyPopulatedSearchCriteria: SearchCriteria = {
    corpus: 'some_corpus',
    leftHandTraceFilter: {'os': ['apple', 'banana'], 'config': ['1234', '5678']},
    rightHandTraceFilter: {'gpu': ['grape']},
    includePositiveDigests: true,
    includeNegativeDigests: false,
    includeUntriagedDigests: true,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: true,
    minRGBADelta: 7,
    maxRGBADelta: 89,
    mustHaveReferenceImage: true,
    sortOrder: 'ascending'
  };

  const fullyPopulatedHintableObject: SearchCriteriaHintableObject = {
    corpus: 'some_corpus',
    left_filter: 'config=1234&config=5678&os=apple&os=banana',
    right_filter: 'gpu=grape',
    positive: true,
    negative: false,
    untriaged: true,
    not_at_head: false,
    include_ignored: true,
    min_rgba: 7,
    max_rgba: 89,
    reference_image_required: true,
    sort: 'ascending'
  };

  const fullyPopulatedQueryString =
    'corpus=some_corpus&include_ignored=true' +
    '&left_filter=config%3D1234%26config%3D5678%26os%3Dapple%26os%3Dbanana&max_rgba=89&min_rgba=7' +
    '&negative=false&not_at_head=false&positive=true&reference_image_required=true' +
    '&right_filter=gpu%3Dgrape&sort=ascending&untriaged=true';
});
