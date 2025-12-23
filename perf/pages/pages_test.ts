import './alerts';
import './clusters2';
import './dryrunalert';
import './favorites';
import './help';
import './multiexplore';
import './newindex';
import './playground';
import './regressions';
import './report';
import './revisions';
import './triage';
import './trybot';
import { assert } from 'chai';

describe('Perf Pages', () => {
  it('should import all page modules and define custom elements', () => {
    const expectedElements = [
      'alerts-page-sk',
      'cluster-page-sk',
      'cluster-lastn-page-sk',
      'favorites-sk',
      'explore-multi-sk',
      'explore-sk',
      'anomaly-playground-sk',
      'regressions-page-sk',
      'report-page-sk',
      'revision-info-sk',
      'triage-page-sk',
      'trybot-page-sk',
    ];
    expectedElements.forEach((el) => {
      assert.isNotNull(customElements.get(el), `Element ${el} should be defined`);
    });
  });

  it('can instantiate explore-sk', () => {
    const explore = document.createElement('explore-sk');
    assert.isNotNull(explore);
    assert.equal(explore.tagName, 'EXPLORE-SK');
  });
});
