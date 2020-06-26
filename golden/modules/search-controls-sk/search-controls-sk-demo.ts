import './index';
import '../gold-scaffold-sk';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';
import { SearchControlsSk, SearchCriteria } from './search-controls-sk';
import { $$ } from 'common-sk/modules/dom';

const corpora = ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'];

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};

let currentValue: SearchCriteria = {
  corpus: 'gm',
  leftHandTraceFilter: {
    'car make': ['chevrolet', 'dodge', 'ford'],
    'color': ['blue'],
    'year': ['2020', '2019']
  },
  rightHandTraceFilter: {'color': ['blue'], 'used': ['yes']},
  includePositiveDigests: true,
  includeNegativeDigests: true,
  includeUntriagedDigests: true,
  includeDigestsNotAtHead: false,
  includeIgnoredDigests: false,
  minRGBADelta: 100,
  maxRGBADelta: 200,
  mustHaveReferenceImage: true,
  sortOrder: 'descending'
};
updateSearchCriteriaPreview();

const searchControlsSk = new SearchControlsSk();
searchControlsSk.paramSet = paramSet;
searchControlsSk.corpora = corpora;
searchControlsSk.searchCriteria = currentValue;
searchControlsSk.addEventListener('search-controls-sk-change', (e: Event) => {
  currentValue = (e as CustomEvent<SearchCriteria>).detail;
  updateSearchCriteriaPreview();
});
$$('.container')!.appendChild(searchControlsSk);

$$<HTMLButtonElement>('#clear')!.addEventListener('click', () => {
  searchControlsSk.searchCriteria = {
    corpus: 'canvaskit',
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
  currentValue = searchControlsSk.searchCriteria;
  updateSearchCriteriaPreview();
});

// Updates the "Search criteria" section of the demo page.
function updateSearchCriteriaPreview() {
  const set =
    (selector: string, text: string | number | boolean) =>
      $$<HTMLSpanElement>('.preview ' + selector)!.innerText = text.toString();

  set('.corpus', currentValue.corpus);
  set('.left-hand-trace-filter', fromParamSet(currentValue.leftHandTraceFilter));
  set('.right-hand-trace-filter', fromParamSet(currentValue.rightHandTraceFilter));
  set('.include-positive-digests', currentValue.includePositiveDigests);
  set('.include-negative-digests', currentValue.includeNegativeDigests);
  set('.include-untriaged-digests', currentValue.includeUntriagedDigests);
  set('.include-digests-not-at-head', currentValue.includeDigestsNotAtHead);
  set('.include-ignored-digests', currentValue.includeIgnoredDigests);
  set('.min-rgba-delta', currentValue.minRGBADelta);
  set('.max-rgba-delta', currentValue.maxRGBADelta);
  set('.sort-order', currentValue.sortOrder);
  set('.must-have-reference-image', currentValue.mustHaveReferenceImage);
}
