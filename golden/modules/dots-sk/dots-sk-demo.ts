import './index';
import { $$ } from 'common-sk/modules/dom';
import { isPuppeteerTest } from '../demo_util';
import { traces, commits } from './demo_data';
import { DotsSk } from './dots-sk';

const logEventDetail = (e: Event) => {
  const detail = (e as CustomEvent).detail;
  const log = $$<HTMLTextAreaElement>('#event-log')!;
  const entry = `Timestamp:    ${new Date().toISOString()}\n`
      + `Event type:   ${e.type}\n`
      + `Event detail: ${JSON.stringify(detail)}\n\n`;
  log.value = entry + log.value;
};

const dotsSk = new DotsSk();
dotsSk.value = traces;
dotsSk.commits = commits;
dotsSk.addEventListener('showblamelist', logEventDetail);
dotsSk.addEventListener('hover', logEventDetail);
$$('#container')!.appendChild(dotsSk);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  $$<HTMLDivElement>('#event-log-container')!.style.display = 'none';
}
