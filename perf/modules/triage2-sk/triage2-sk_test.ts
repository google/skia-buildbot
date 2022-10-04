import './index';
import { assert } from 'chai';
import { Status } from '../json/all';
import { TriageSk } from './triage2-sk';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('triage2-sk', () => {
  describe('event', () => {
    it('fires when button is clicked', () => window.customElements.whenDefined('triage2-sk').then(() => {
      container.innerHTML = '<triage2-sk value=untriaged></triage2-sk>';
      let value = 'unfired';
      const tr = container.firstElementChild! as TriageSk;
      tr.addEventListener('change', (e) => {
        value = (e as CustomEvent<Status>).detail;
      });
        tr.querySelector<HTMLButtonElement>('.positive')!.click();
        assert.equal('positive', tr.value, 'Element is changed.');
        assert.equal('positive', value, 'Event was sent.');
    }));
  });
});
