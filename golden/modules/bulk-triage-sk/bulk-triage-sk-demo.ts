import './index';
import { $$ } from 'common-sk/modules/dom';
import { examplePageData, exampleAllData } from './test_data';
import { BulkTriageSk } from './bulk-triage-sk';
import fetchMock from 'fetch-mock';

const handleTriaged = () => {
  const log = $$<HTMLPreElement>('#event_log')!;
  log.textContent += 'Did triage.\n';
};

const handleCancelled = () => {
  const log = $$<HTMLPreElement>('#event_log')!;
  log.textContent += 'Cancelled.\n';
};

const ele = new BulkTriageSk();
ele.currentPageDigests = examplePageData;
ele.allDigests = exampleAllData;
ele.addEventListener('bulk_triage_invoked', handleTriaged);
ele.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#default')!.appendChild(ele);

const eleCL = new BulkTriageSk();
eleCL.currentPageDigests = examplePageData;
eleCL.allDigests = exampleAllData;
eleCL.changeListID = '1234567';
eleCL.crs = 'github';
eleCL.addEventListener('bulk_triage_invoked', handleTriaged);
ele.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#changelist')!.appendChild(eleCL);

fetchMock.post('/json/v1/triage', 200);
