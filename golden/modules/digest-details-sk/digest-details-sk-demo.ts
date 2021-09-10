import './index';
import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';
import {
  typicalDetails, negativeOnly, noRefs, noRefsYet, noTraces, twoHundredCommits, fakeNow,
} from './test_data';
import { setImageEndpointsForDemos } from '../common';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { DigestDetailsSk } from './digest-details-sk';
import { SearchResult } from '../rpc_types';

Date.now = () => fakeNow;
testOnlySetSettings({
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

setImageEndpointsForDemos();
let ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
$$('#normal')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = negativeOnly;
ele.commits = twoHundredCommits;
$$('#negative_only')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noRefs;
ele.commits = twoHundredCommits;
$$('#no_refs')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noRefsYet;
ele.commits = twoHundredCommits;
$$('#no_refs_yet')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.changeListID = '12345';
ele.crs = 'gerrit';
$$('#changelist_id')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.right = typicalDetails.refDiffs!.neg;
$$('#right_overridden')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noTraces;
ele.commits = twoHundredCommits;
$$('#no_traces')!.appendChild(ele);

ele = new DigestDetailsSk();
const noParams = JSON.parse(JSON.stringify(noTraces)) as SearchResult;
noParams.paramset = {};
ele.details = noParams;
ele.commits = twoHundredCommits;
$$('#no_params')!.appendChild(ele);

document.addEventListener('triage', (e) => {
  $$('#event')!.textContent = `triage: ${JSON.stringify((e as CustomEvent).detail)}`;
});
document.addEventListener('show-commits', (e) => {
  $$('#event')!.textContent = `show-commits: ${JSON.stringify((e as CustomEvent).detail)}`;
});
document.addEventListener('fetch-error', (e) => {
  $$('#event')!.textContent = `fetch-error: ${JSON.stringify((e as CustomEvent).detail)}`;
});

fetchMock.post('/json/v2/triage', delay(() => {
  if ($$<HTMLInputElement>('#simulate-not-logged-in')!.checked) {
    return 403;
  }
  return 200;
}, 300));
