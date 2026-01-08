import './alerts';
import './clusters2';
import './dryrunalert';
import './extralinks';
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
  const expectedElements = [
    'alerts-page-sk',
    'cluster-page-sk',
    'cluster-lastn-page-sk',
    'favorites-sk',
    'explore-multi-sk',
    'explore-sk',
    'extra-links-sk',
    'anomaly-playground-sk',
    'regressions-page-sk',
    'report-page-sk',
    'revision-info-sk',
    'triage-page-sk',
    'trybot-page-sk',
  ];

  beforeEach(() => {
    (window as any).perf = {
      radius: 7,
      interesting: 0.1,
      demo: false,
      key_order: ['config'],
      git_repo_url: 'https://skia.googlesource.com/skia',
    };
  });

  expectedElements.forEach((el) => {
    it(`can instantiate ${el}`, () => {
      assert.isNotNull(customElements.get(el), `Element ${el} should be defined`);
      const instance = document.createElement(el);
      assert.isNotNull(instance);
      assert.equal(instance.tagName, el.toUpperCase());
    });
  });
});
