import './index';
import { setUpElementUnderTest, eventPromise } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { SearchPageSk } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';

const expect = chai.expect;

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;

  beforeEach(() => {
    searchPageSk = newInstance();
    searchPageSkPO = new SearchPageSkPO(searchPageSk);
  });
});
