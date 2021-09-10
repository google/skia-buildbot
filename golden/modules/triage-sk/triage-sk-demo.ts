import './index';
import { isPuppeteerTest } from '../demo_util';
import { TriageSk } from './triage-sk';
import { Label } from '../rpc_types';

const log = (message: string) => {
  const log = document.querySelector<HTMLTextAreaElement>('#event-log')!;
  const entry = `${new Date().toISOString()}\t${message}\n`;
  log.value = entry + log.value;
};

const triageSk = new TriageSk();
triageSk.addEventListener('change', (e: Event) => log((e as CustomEvent<Label>).detail));

document.querySelector<HTMLDivElement>('#container')!.appendChild(triageSk);

// Hide event log if we're within a Puppeteer test. We don't need the event log
// to appear in any screenshots uploaded to Gold.
if (isPuppeteerTest()) {
  document.querySelector<HTMLDivElement>('#event-log-container')!.style.display = 'none';
}
