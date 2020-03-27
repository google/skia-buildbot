import './index';
import { digestDetails, negativeOnly, noRefs, twoHundredCommits } from './test_data';
import { $$ } from '../../../common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';

setImageEndpointsForDemos();
let ele = document.createElement('digest-details-sk');
ele.details = digestDetails;
ele.commits = twoHundredCommits;
$$('#normal').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = negativeOnly;
ele.commits = twoHundredCommits;
$$('#negative_only').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = noRefs;
ele.commits = twoHundredCommits;
$$('#no_refs').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = digestDetails;
ele.commits = twoHundredCommits;
ele.issue = '12345';
$$('#changelist_id').appendChild(ele);

document.addEventListener('triage', (e) => {
  $$('#event').textContent = `triage: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('show-commits', (e) => {
  $$('#event').textContent = `show-commits: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('zoom-clicked', (e) => {
  $$('#event').textContent = `zoom-clicked: ${JSON.stringify(e.detail)}`;
});
