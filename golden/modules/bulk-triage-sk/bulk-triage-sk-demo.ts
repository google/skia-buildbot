import './index';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { bulkTriageDeltaInfos } from './test_data';
import { BulkTriageSk } from './bulk-triage-sk';
import { TriageResponse } from '../rpc_types';

const handleTriaged = () => {
  const log = $$<HTMLPreElement>('#event_log')!;
  log.textContent += 'Did triage.\n';
};

const handleCancelled = () => {
  const log = $$<HTMLPreElement>('#event_log')!;
  log.textContent += 'Cancelled.\n';
};

const ele = new BulkTriageSk();
ele.bulkTriageDeltaInfos = deepCopy(bulkTriageDeltaInfos);
ele.addEventListener('bulk_triage_invoked', handleTriaged);
ele.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#default')!.appendChild(ele);

const eleCL = new BulkTriageSk();
eleCL.bulkTriageDeltaInfos = deepCopy(bulkTriageDeltaInfos);
eleCL.changeListID = '1234567';
eleCL.crs = 'github';
eleCL.addEventListener('bulk_triage_invoked', handleTriaged);
eleCL.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#changelist')!.appendChild(eleCL);

const response: TriageResponse = { status: 'ok' };
fetchMock.post({ url: '/json/v3/triage' }, {
  status: 200,
  body: response,
});
