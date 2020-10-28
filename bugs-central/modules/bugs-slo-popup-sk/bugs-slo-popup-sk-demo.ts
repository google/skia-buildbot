import './index';
import { $$ } from 'common-sk/modules/dom';
import { BugsSLOPopupSk, Issue } from './bugs-slo-popup-sk';

const priToSLOIssues: Record<string, Issue[]> = {
  Priority1: [
    {
      id: '123', priority: 'P1', link: 'www.test-link.com', slo_violation_reason: 'exceeded creation time by 1year',
    },
    {
      id: '120', priority: 'P1', link: 'www.test-link.com', slo_violation_reason: 'exceeded modified time by 2months',
    },
  ],
  Priority2: [
    {
      id: '34', priority: 'P2', link: 'www.test-link.com', slo_violation_reason: 'exceeded creation time by 2 days',
    },
  ],
};

const bugsSLOPopupSk = new BugsSLOPopupSk();
$$('body')!.appendChild(bugsSLOPopupSk);

$$<HTMLButtonElement>('#show-dialog')!.addEventListener('click', () => {
  bugsSLOPopupSk.open(priToSLOIssues);
});
