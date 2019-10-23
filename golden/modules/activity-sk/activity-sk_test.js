import './index.js'

import { $$ } from 'common-sk/modules/dom'

describe('activity-sk', () => {
  // Component under test.
  let activitySk;

  // Create a fresh instance before each test case.
  beforeEach((done) => {
    window.customElements.whenDefined('activity-sk').then(() => {
      activitySk = document.createElement('activity-sk');
      document.body.appendChild(activitySk);
    }).then(done)
  });

  afterEach(() => {
    document.body.removeChild(activitySk);
  });

  describe('not spinning', () => {
    it('is empty initially', () => {
      expect(activitySk.childElementCount).to.equal(0);
    });

    it('returns the right isSpinning value', () => {
      expect(activitySk.isSpinning).to.be.false;
    });
  });

  describe('spinning', () => {
    beforeEach(() => {
      activitySk.startSpinner("Hello, world!");
    });

    it('spins and shows the right text', () => {
      expect($$("spinner-sk")).not.to.be.null;
      expect($$("span", activitySk).innerText).to.equal("Hello, world!")
    });

    it('stops spinning when requested', () => {
      activitySk.stopSpinner();
      expect(activitySk.childElementCount).to.equal(0);
    });

    it('can spin again after having been stopped', () => {
      activitySk.stopSpinner();
      activitySk.startSpinner("foo");
      expect($$("span", activitySk).innerText).to.equal("foo")
    });

    it('returns the right isSpinning value', () => {
      expect(activitySk.isSpinning).to.be.true;
    });
  });
});