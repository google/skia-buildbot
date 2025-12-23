import './index';
import { assert } from 'chai';
import { TriageStatusSk, TriageStatusSkStartTriageEventDetails } from './triage-status-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('triage-status-sk', () => {
  const newInstance = setUpElementUnderTest<TriageStatusSk>('triage-status-sk');

  let element: TriageStatusSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('renders initial status', () => {
    const button = element.querySelector('button')!;
    assert.equal(button.className, 'untriaged');
    assert.equal(button.title, '(none)');
  });

  it('updates when triage property is set', () => {
    element.triage = { status: 'positive', message: 'Confirmed regression' };
    const button = element.querySelector('button')!;
    assert.equal(button.className, 'positive');
    assert.equal(button.title, 'Confirmed regression');

    const tricon = element.querySelector('tricon2-sk')!;
    assert.equal(tricon.getAttribute('value'), 'positive');
  });

  it('emits start-triage event when clicked', async () => {
    const triageData = { status: 'negative' as const, message: 'False positive' };
    const fullSummary: any = { summary: { num: 10 } };
    const alert: any = { id_as_string: '1' };

    element.triage = triageData;
    element.full_summary = fullSummary;
    element.alert = alert;
    element.cluster_type = 'high';

    const eventPromise = new Promise<CustomEvent<TriageStatusSkStartTriageEventDetails>>(
      (resolve) => {
        element.addEventListener(
          'start-triage',
          (e) => {
            resolve(e as CustomEvent<TriageStatusSkStartTriageEventDetails>);
          },
          { once: true }
        );
      }
    );

    element.querySelector('button')!.click();

    const event = await eventPromise;
    assert.deepEqual(event.detail.triage, triageData);
    assert.deepEqual(event.detail.full_summary, fullSummary);
    assert.deepEqual(event.detail.alert, alert);
    assert.equal(event.detail.cluster_type, 'high');
    assert.equal(event.detail.element, element);
  });
});
