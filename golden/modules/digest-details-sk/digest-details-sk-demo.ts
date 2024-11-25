import './index';
import fetchMock, { MockResponseObject } from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import {
  typicalDetails,
  negativeOnly,
  noRefs,
  noRefsYet,
  noTraces,
  twoHundredCommits,
  fakeNow,
  typicalDetailsDisallowTriaging,
  noRefsDisallowTriaging,
} from './test_data';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { DigestDetailsSk } from './digest-details-sk';
import { SearchResult, TriageResponse } from '../rpc_types';
import { groupingsResponse } from '../search-page-sk/demo_data';

Date.now = () => fakeNow;
testOnlySetSettings({
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

let ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#normal')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetailsDisallowTriaging;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#normal_disallow_triaging')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = negativeOnly;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#negative_only')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noRefs;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#no_refs')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noRefsDisallowTriaging;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#no_refs_disallow_triaging')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noRefsYet;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#no_refs_yet')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
ele.changeListID = '12345';
ele.crs = 'gerrit';
$$('#changelist_id')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
ele.right = typicalDetails.refDiffs!.neg;
$$('#right_overridden')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = noTraces;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#no_traces')!.appendChild(ele);

ele = new DigestDetailsSk();
const noParams = JSON.parse(JSON.stringify(noTraces)) as SearchResult;
noParams.paramset = {};
ele.details = noParams;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
$$('#no_params')!.appendChild(ele);

ele = new DigestDetailsSk();
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.groupings = groupingsResponse;
ele.fullSizeImages = true;
$$('#full_size_images')!.appendChild(ele);

document.addEventListener('triage', (e) => {
  $$('#event')!.textContent = `triage: ${JSON.stringify((e as CustomEvent).detail)}`;
});
document.addEventListener('show-commits', (e) => {
  $$('#event')!.textContent = `show-commits: ${JSON.stringify((e as CustomEvent).detail)}`;
});
document.addEventListener('fetch-error', (e) => {
  $$('#event')!.textContent = `fetch-error: ${JSON.stringify((e as CustomEvent).detail)}`;
});

fetchMock.post(
  '/json/v3/triage',
  delay(() => {
    if ($$<HTMLInputElement>('#simulate-not-logged-in')!.checked) {
      return 403;
    }
    const triageResponse: TriageResponse = { status: 'ok' };
    const mro: MockResponseObject = {
      status: 200,
      body: triageResponse,
    };
    return mro;
  }, 300)
);
