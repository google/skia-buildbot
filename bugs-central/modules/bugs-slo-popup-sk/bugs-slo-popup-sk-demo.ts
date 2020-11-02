import './index';
import { $$ } from 'common-sk/modules/dom';
import { BugsSLOPopupSk } from './bugs-slo-popup-sk';
import { Issue } from '../json';

const priToSLOIssues: Record<string, Issue[]> = {
  Priority1: [
    {
      id: '123', priority: 'P1', link: 'www.test-link.com', slo_violation_reason: 'exceeded creation time by 1year', owner: '', slo_violation: true, slo_violation_duration: 10, created: '2m', modified: '4m', title: '', summary: '', state: 'open',
    },
    {
      id: '120', priority: 'P1', link: 'www.test-link.com', slo_violation_reason: 'exceeded modified time by 2months', owner: '', slo_violation: true, slo_violation_duration: 10, created: '2m', modified: '4m', title: '', summary: '', state: 'open',
    },
  ],
  Priority2: [
    {
      id: '34', priority: 'P2', link: 'www.test-link.com', slo_violation_reason: 'exceeded creation time by 2 days', owner: '', slo_violation: true, slo_violation_duration: 10, created: '2m', modified: '4m', title: '', summary: '', state: 'open',
    },
  ],
};

const bugsSLOPopupSk = new BugsSLOPopupSk();
$$('body')!.appendChild(bugsSLOPopupSk);

$$<HTMLButtonElement>('#show-dialog')!.addEventListener('click', () => {
  bugsSLOPopupSk.open(priToSLOIssues);
});
