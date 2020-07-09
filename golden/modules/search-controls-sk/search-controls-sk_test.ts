import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { SearchControlsSk, SearchCriteria,
  SearchCriteriaFromHintableObject, SearchCriteriaToHintableObject } from './search-controls-sk';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { fromObject, toObject } from 'common-sk/modules/query';
import { testOnlySetSettings } from '../settings';
import {HintableObject} from "common-sk/modules/hintable";

const expect = chai.expect;

describe('search-controls-sk', () => {
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
  };

  const newInstance = setUpElementUnderTest<SearchControlsSk>('search-controls-sk');

  let searchControlsSk: SearchControlsSk;

  beforeEach(() => {
    searchControlsSk = newInstance();
    searchControlsSk.corpora = corpora;
    searchControlsSk.paramSet = paramSet;
    searchControlsSk.searchCriteria = makeSearchCriteria();
  });

  it('shows the initial value', () => {
    expect(getSearchCriteriaFromUI()).to.deep.equal(makeSearchCriteria());
  });

  describe('corpus', () => {
    const searchCriteria = makeSearchCriteria({corpus: 'image'});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getCorpus()).to.equal('image');
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      clickCorpus('image');
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('include positive digests', () => {
    const searchCriteria = makeSearchCriteria({includePositiveDigests: true});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getIncludePositiveDigestsCheckBox().checked).to.be.true;
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      getIncludePositiveDigestsCheckBox().click();
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('include negative digests', () => {
    const searchCriteria = makeSearchCriteria({includeNegativeDigests: true});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getIncludeNegativeDigestsCheckBox().checked).to.be.true;
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      getIncludeNegativeDigestsCheckBox().click();
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('include untriaged digests', () => {
    const searchCriteria = makeSearchCriteria({includeUntriagedDigests: true});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getIncludeUntriagedDigestsCheckBox().checked).to.be.true;
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      getIncludeUntriagedDigestsCheckBox().click();
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('include digests not at HEAD', () => {
    const searchCriteria = makeSearchCriteria({includeDigestsNotAtHead: true});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getIncludeDigestsNotAtHeadCheckBox().checked).to.be.true;
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      getIncludeDigestsNotAtHeadCheckBox().click();
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('include ignored digests', () => {
    const searchCriteria = makeSearchCriteria({includeIgnoredDigests: true});

    it('can change programmatically', () => {
      searchControlsSk.searchCriteria = searchCriteria;
      expect(getIncludeIgnoredDigestsCheckBox().checked).to.be.true;
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      getIncludeIgnoredDigestsCheckBox().click();
      expect((await event).detail).to.deep.equal(searchCriteria);
      expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
    });
  });

  describe('left-hand trace filter', () => {
    it('can change programmatically', () => {
      searchControlsSk.searchCriteria =
        makeSearchCriteria({leftHandTraceFilter: {'car make': ['ford']}});
      expect(getLeftHandTraceFilterValue()).to.deep.equal({'car make': ['ford']});
    });

    it('can change via the UI', async () => {
      const event = changeEventPromise();
      const traceFilter = changeLeftHandTraceFilter();
      expect((await event).detail)
        .to.deep.equal(makeSearchCriteria({leftHandTraceFilter: traceFilter}));
      expect(searchControlsSk.searchCriteria)
        .to.deep.equal(makeSearchCriteria({leftHandTraceFilter: traceFilter}));
    });
  });

  describe('more filters', () => {
    describe('right-hand trace filter', () => {
      it('can change programmatically', () => {
        searchControlsSk.searchCriteria =
          makeSearchCriteria({rightHandTraceFilter: {'car make': ['ford']}});
        clickMoreFiltersBtn();
        expect(getRightHandTraceFilterValue()).to.deep.equal({'car make': ['ford']});
      });

      it('can change via the UI', async () => {
        const event = changeEventPromise();
        clickMoreFiltersBtn();
        const traceFilter = changeRightHandTraceFilter();
        clickFilterDialogSubmitBtn();
        expect((await event).detail)
          .to.deep.equal(makeSearchCriteria({rightHandTraceFilter: traceFilter}));
        expect(searchControlsSk.searchCriteria)
          .to.deep.equal(makeSearchCriteria({rightHandTraceFilter: traceFilter}));
      });
    });

    describe('min RGBA delta', () => {
      // Arbitrary number in the middle of the allowed range (0 to 255).
      const searchCriteria = makeSearchCriteria({minRGBADelta: 100});

      it('can change programmatically', () => {
        searchControlsSk.searchCriteria = searchCriteria;
        clickMoreFiltersBtn();
        expect(getMinRGBADeltaInput().value).to.equal('100');
      });

      it('can change via the UI', async () => {
        const event = changeEventPromise();
        clickMoreFiltersBtn();
        getMinRGBADeltaInput().value = '100';
        getMinRGBADeltaInput().dispatchEvent(new CustomEvent('input')); // Simulate UI interaction.
        clickFilterDialogSubmitBtn();
        expect((await event).detail).to.deep.equal(searchCriteria);
        expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
      });
    });

    describe('max RGBA delta', () => {
      // Arbitrary number in the middle of the allowed range (0 to 255).
      const searchCriteria = makeSearchCriteria({maxRGBADelta: 100});

      it('can change programmatically', () => {
        searchControlsSk.searchCriteria = searchCriteria;
        clickMoreFiltersBtn();
        expect(getMaxRGBADeltaInput().value).to.equal('100');
      });

      it('can change via the UI', async () => {
        const event = changeEventPromise();
        clickMoreFiltersBtn();
        getMaxRGBADeltaInput().value = '100';
        getMaxRGBADeltaInput().dispatchEvent(new CustomEvent('input')); // Simulate UI interaction.
        clickFilterDialogSubmitBtn();
        expect((await event).detail).to.deep.equal(searchCriteria);
        expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
      });
    });

    describe('sort order', () => {
      const searchCriteria = makeSearchCriteria({sortOrder: 'descending'});

      it('can change programmatically', () => {
        searchControlsSk.searchCriteria = searchCriteria;
        clickMoreFiltersBtn();
        expect(getSortOrderSelect().value).to.equal('descending');
      });

      it('can change via the UI', async () => {
        const event = changeEventPromise();
        clickMoreFiltersBtn();
        getSortOrderSelect().value = 'descending';
        getSortOrderSelect().dispatchEvent(new CustomEvent('change')); // Simulate UI interaction.
        clickFilterDialogSubmitBtn();
        expect((await event).detail).to.deep.equal(searchCriteria);
        expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
      });
    });

    describe('must have reference image', () => {
      const searchCriteria = makeSearchCriteria({mustHaveReferenceImage: true});

      it('can change programmatically', () => {
        searchControlsSk.searchCriteria = searchCriteria;
        clickMoreFiltersBtn();
        expect(getMustHaveReferenceImageCheckBox().checked).to.be.true;
      });

      it('can change via the UI', async () => {
        const event = changeEventPromise();
        clickMoreFiltersBtn();
        getMustHaveReferenceImageCheckBox().click();
        clickFilterDialogSubmitBtn();
        expect((await event).detail).to.deep.equal(searchCriteria);
        expect(searchControlsSk.searchCriteria).to.deep.equal(searchCriteria);
      });
    });
  });

  const changeEventPromise =
    () => eventPromise<CustomEvent<SearchCriteria>>('search-controls-sk-change');

  // Returns the SearchCriteria displayed by the search-controls-sk element by inspecting the DOM.
  const getSearchCriteriaFromUI = (): SearchCriteria => {
    const searchCriteria: Partial<SearchCriteria> = {}
    searchCriteria.corpus = getCorpus();
    searchCriteria.leftHandTraceFilter = getLeftHandTraceFilterValue();
    searchCriteria.includePositiveDigests = getIncludePositiveDigestsCheckBox().checked;
    searchCriteria.includeNegativeDigests = getIncludeNegativeDigestsCheckBox().checked;
    searchCriteria.includeUntriagedDigests = getIncludeUntriagedDigestsCheckBox().checked;
    searchCriteria.includeDigestsNotAtHead = getIncludeDigestsNotAtHeadCheckBox().checked;
    searchCriteria.includeIgnoredDigests = getIncludeIgnoredDigestsCheckBox().checked;

    // More filters dialog.
    clickMoreFiltersBtn();
    searchCriteria.rightHandTraceFilter = getTraceFilterValue('filter-dialog-sk trace-filter-sk');
    searchCriteria.minRGBADelta = parseInt(getMinRGBADeltaInput().value);
    searchCriteria.maxRGBADelta = parseInt(getMaxRGBADeltaInput().value);
    searchCriteria.mustHaveReferenceImage = getMustHaveReferenceImageCheckBox().checked;
    searchCriteria.sortOrder = getSortOrderSelect().value as 'ascending' | 'descending';
    clickFilterDialogCancelBtn();

    return searchCriteria as SearchCriteria;
  };

  // Corpus.

  const getCorpus =
    () => $$<HTMLLIElement>('corpus-selector-sk li.selected', searchControlsSk)!.innerText;

  const clickCorpus =
    (corpus: string) =>
      $<HTMLLIElement>('corpus-selector-sk li', searchControlsSk)
        .find(li => li.innerText === corpus)!
        .click();

  // Checkboxes.

  const getIncludePositiveDigestsCheckBox =
    () => $$<CheckOrRadio>('.include-positive-digests', searchControlsSk)!;

  const getIncludeNegativeDigestsCheckBox =
    () => $$<CheckOrRadio>('.include-negative-digests', searchControlsSk)!;

  const getIncludeUntriagedDigestsCheckBox =
    () => $$<CheckOrRadio>('.include-untriaged-digests', searchControlsSk)!;

  const getIncludeDigestsNotAtHeadCheckBox =
    () => $$<CheckOrRadio>('.include-digests-not-at-head', searchControlsSk)!;

  const getIncludeIgnoredDigestsCheckBox =
    () => $$<CheckOrRadio>('.include-ignored-digests', searchControlsSk)!;

  // Trace filters.

  const getLeftHandTraceFilterValue = () => getTraceFilterValue('.traces trace-filter-sk');

  const getRightHandTraceFilterValue =
    () => getTraceFilterValue('filter-dialog-sk trace-filter-sk');

  const getTraceFilterValue = (selector: string): ParamSet => {
    const paramSet: ParamSet = {};
    $(`${selector} paramset-sk tr`, searchControlsSk).forEach((tr, i) => {
      if (i === 0) return; // Skip the first row, which is empty as there are no titles.
      const key = $$('th', tr)!.textContent!;
      const values = $('div', tr).map(div => div.textContent!);
      paramSet[key] = values;
    })
    return paramSet;
  };

  const changeLeftHandTraceFilter = () => {
    const selector = '.traces trace-filter-sk query-dialog-sk';
    $$<HTMLButtonElement>('.traces .edit-query')!.click();
    const newFilter = changeQueryDialogSelection(selector);
    clickQueryDialogSubmitBtn(selector);
    return newFilter;
  };

  const changeRightHandTraceFilter = () => {
    const selector = 'filter-dialog-sk query-dialog-sk';
    $$<HTMLButtonElement>('filter-dialog-sk .edit-query')!.click();
    const newFilter = changeQueryDialogSelection(selector);
    clickQueryDialogSubmitBtn(selector);
    return newFilter;
  };

  const changeQueryDialogSelection = (queryDialogSelector: string): ParamSet => {
    const queryDialogSk = $$(queryDialogSelector, searchControlsSk)!;
    $$<HTMLButtonElement>('.clear_selections', queryDialogSk)!.click();
    $$<HTMLDivElement>('select-sk div:nth-child(2)', queryDialogSk)!.click(); // Color.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // Green.
    $$<HTMLDivElement>('select-sk div:nth-child(3)', queryDialogSk)!.click(); // Used.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(1)', queryDialogSk)!.click(); // Yes.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // No.
    return {'color': ['green'], 'used': ['yes', 'no']};
  };

  const clickQueryDialogSubmitBtn =
    (queryDialogSelector: string) =>
      $$<HTMLButtonElement>(`${queryDialogSelector} .show-matches`, searchControlsSk)!.click();

  // More filters.

  const clickMoreFiltersBtn =
    () => $$<HTMLButtonElement>('.more-filters', searchControlsSk)!.click();

  const clickFilterDialogSubmitBtn =
    () =>
      $$<HTMLButtonElement>(
        'filter-dialog-sk .filter-dialog > .buttons .filter', searchControlsSk)!.click();

  const clickFilterDialogCancelBtn =
    () =>
      $$<HTMLButtonElement>(
        'filter-dialog-sk .filter-dialog > .buttons .cancel', searchControlsSk)!.click();

  const getMinRGBADeltaInput =
    () => $$<HTMLInputElement>('filter-dialog-sk #min-rgba-delta', searchControlsSk)!;

  const getMaxRGBADeltaInput =
    () => $$<HTMLInputElement>('filter-dialog-sk #max-rgba-delta', searchControlsSk)!;

  const getSortOrderSelect =
    () => $$<HTMLSelectElement>('filter-dialog-sk #sort-order', searchControlsSk)!;

  const getMustHaveReferenceImageCheckBox =
    () => $$<CheckOrRadio>('filter-dialog-sk #must-have-reference-image')!;
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
