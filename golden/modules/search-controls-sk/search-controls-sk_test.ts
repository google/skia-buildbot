import './index';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { SearchControlsSk, SearchCriteria, SearchCriteriaFromHintableObject, SearchCriteriaToHintableObject } from './search-controls-sk';
import { SearchControlsSkPO } from './search-controls-sk_po';
import { testOnlySetSettings } from '../settings';
import { fromObject, toObject } from 'common-sk/modules/query';
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

describe('SearchCriteriaHintableObject and helpers', () => {
  before(() => {
    testOnlySetSettings({
      defaultCorpus: 'the_default_corpus',
    });
  });

  after(() => {
    testOnlySetSettings({});
  });

  describe('SearchCriteriaToHintableObject', () => {
    it('can be used to produce a URL with all settings', () => {
      const sc = makeFilledSearchCriteria();

      const hintObj = SearchCriteriaToHintableObject(sc);
      const url = fromObject(hintObj as HintableObject);
      expect(url).to.equal('corpus=some_corpus&include_ignored=true'+
          '&left_filter=config%3D1234%26config%3D5678%26os%3Dapple%26os%3Dbanana'+
          '&max_rgba=89&min_rgba=7&negative=false&not_at_head=false'+
          '&positive=true&reference_image_required=true'+
          '&right_filter=gpu%3Dgrape&sort=ascending&untriaged=true');
    });

    it('produces a URL with missing settings', () => {
      const sc: Partial<SearchCriteria> = {};
      const hintObj = SearchCriteriaToHintableObject(sc);
      const url = fromObject(hintObj as HintableObject);
      expect(url).to.equal('corpus=&include_ignored=false&left_filter='+
          '&max_rgba=0&min_rgba=0&negative=false&not_at_head=false'+
          '&positive=false&reference_image_required=false&right_filter='+
          '&sort=descending&untriaged=false');
    });
  }); // describe('SearchCriteriaToHintableObject');

  describe('SearchCriteriaFromHintableObject', () => {
    it('can create a SearchCriteria from a complete url', () => {
      const url = 'corpus=some_corpus&include_ignored=true'+
          '&left_filter=config%3D1234%26config%3D5678%26os%3Dapple%26os%3Dbanana'+
          '&max_rgba=89&min_rgba=7&negative=false&not_at_head=false'+
          '&positive=true&reference_image_required=true'+
          '&right_filter=gpu%3Dgrape&sort=ascending&untriaged=true';
      const urlObj = toObject(url, makeEmptyHint());
      const sc = SearchCriteriaFromHintableObject(urlObj);
      expect(sc).to.deep.equal(makeFilledSearchCriteria());
    });

    it('can create a SearchCriteria from a url with everything blank', () => {
      const url = 'corpus=&include_ignored=false&left_filter='+
          '&max_rgba=0&min_rgba=0&negative=false&not_at_head=false'+
          '&positive=false&reference_image_required=false&right_filter='+
          '&sort=descending&untriaged=false';
      const urlObj = toObject(url, makeEmptyHint());
      const sc = SearchCriteriaFromHintableObject(urlObj);
      expect(sc).to.deep.equal(makeSearchCriteriaWithDefaults());
    });

    it('can create a SearchCriteria from an empty url', () => {
      const url = '';
      const urlObj = toObject(url, makeEmptyHint());
      const sc = SearchCriteriaFromHintableObject(urlObj);
      expect(sc).to.deep.equal(makeSearchCriteriaWithDefaults());
    });
  }); // describe('SearchCriteriaFromHintableObject');

  function makeFilledSearchCriteria() : SearchCriteria {
    return {
      corpus: 'some_corpus',
      leftHandTraceFilter: {'os':['apple', 'banana'], 'config': ['1234', '5678']},
      rightHandTraceFilter: {'gpu':['grape']},
      includePositiveDigests: true,
      includeNegativeDigests: false,
      includeUntriagedDigests: true,
      includeDigestsNotAtHead: false,
      includeIgnoredDigests: true,
      minRGBADelta: 7,
      maxRGBADelta: 89,
      mustHaveReferenceImage: true,
      sortOrder: 'ascending'
    }
  }

  function makeSearchCriteriaWithDefaults() : SearchCriteria {
    return {
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
      sortOrder: 'descending',
    }
  }

  function makeEmptyHint() : HintableObject {
    return SearchCriteriaToHintableObject({} as Partial<SearchCriteria>) as HintableObject;
  }
});
