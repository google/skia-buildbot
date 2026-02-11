import './index';
import { assert } from 'chai';
import { NewBugDialogSk } from './new-bug-dialog-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly } from '../json';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

describe('new-bug-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<NewBugDialogSk>('new-bug-dialog-sk');
  fetchMock.config.overwriteRoutes = false;

  let element: NewBugDialogSk;
  beforeEach(async () => {
    element = newInstance();
    await element.updateComplete;
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    assert.isTrue(fetchMock.done());
    fetchMock.restore();
  });

  const dummyAnomaly = (bugId: number): Anomaly => ({
    id: '1',
    test_path: 'test/path/suite/subtest',
    bug_id: bugId,
    start_revision: 1234,
    end_revision: 1239,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Test>Component',
    bug_labels: ['TestLabel1', 'TestLabel2'],
    bug_cc_emails: [],
    bisect_ids: [],
  });

  describe('open and close dialog', () => {
    it('opens and closes the dialog', async () => {
      fetchMock.get('/_/login/status', { email: 'test@example.com' });
      assert.isFalse(element.opened);
      element.open();
      await fetchMock.flush(true);
      assert.isTrue(element.opened);
      element.closeDialog();
      assert.isFalse(element.opened);
    });
  });

  describe('set anomalies', () => {
    it('sets the anomalies and trace names', () => {
      const anomalies = [dummyAnomaly(12345)];
      const traceNames = ['trace1', 'trace2'];
      element.anomalies = anomalies;
      element.traceNames = traceNames;
      assert.deepEqual(element.anomalies, anomalies);
      assert.deepEqual(element.traceNames, traceNames);
    });
  });

  describe('file new bug', () => {
    it('successfully files a new bug', async () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];

      fetchMock.post('/_/triage/file_bug', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.equal(body.title, '33.6% regression in suite at 1234:1239');
        return { status: 200, body: JSON.stringify({ bug_id: '12345' }) };
      });

      await element.fileNewBug();
      await fetchMock.flush(true);
      // successfully open a new buganizer page
      sinon.stub(window, 'confirm').returns(true);
    });

    it('handles error when filing a new bug', async () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];
      fetchMock.post('/_/triage/file_bug', 500);
      const event = eventPromise('error-sk');

      await element.fileNewBug();
      await fetchMock.flush(true);
      const errEvent = await event;
      assert.isDefined(errEvent);
      const errMessage = (errEvent as CustomEvent).detail.message as string;
      assert.strictEqual(
        errMessage,
        'File new bug request failed due to an internal server error. Please try again.'
      );
    });
  });

  describe('get bug title', () => {
    it('generates the correct bug title', () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];
      assert.equal(element.getBugTitle(), '33.6% regression in suite at 1234:1239');
    });
  });

  describe('get suite name for alert', () => {
    it('generates the correct suite name', () => {
      const anomaly = dummyAnomaly(0);
      assert.equal(element.getSuiteNameForAlert(anomaly), 'suite');
    });

    it('generates the correct suite name for rendering.desktop', () => {
      const anomaly = dummyAnomaly(0);
      anomaly.test_path = 'test/path/rendering.desktop/subtest';
      assert.equal(element.getSuiteNameForAlert(anomaly), 'rendering.desktop/subtest');
    });
  });

  describe('get display percent changed', () => {
    it('generates the correct display percent changed', () => {
      const anomaly = dummyAnomaly(0);
      assert.equal(element.getDisplayPercentChanged(anomaly), '33.6%');
    });

    it('generates the correct display percent changed for infinity', () => {
      const anomaly = dummyAnomaly(0);
      anomaly.median_before_anomaly = 0;
      assert.equal(element.getDisplayPercentChanged(anomaly), 'zero-to-nonzero');
    });
  });

  describe('get percent change for anomaly', () => {
    it('generates the correct percent change', () => {
      const anomaly = dummyAnomaly(0);
      assert.closeTo(element.getPercentChangeForAnomaly(anomaly), 33.6, 0.1);
    });

    it('generates the correct percent change for infinity', () => {
      const anomaly = dummyAnomaly(0);
      anomaly.median_before_anomaly = 0;
      assert.equal(element.getPercentChangeForAnomaly(anomaly), Number.MAX_VALUE);
    });
  });

  describe('get component radios', () => {
    it('generates the correct component radios', () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];
      const radios = element.getComponentRadios();
      assert.lengthOf(radios, 1);
    });
  });

  describe('get label checkboxes', () => {
    it('generates the correct label checkboxes', () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];
      const checkboxes = element.getLabelCheckboxes();
      assert.lengthOf(checkboxes, 2);
    });
  });

  describe('has labels', () => {
    it('returns true if there are labels', () => {
      const anomalies = [dummyAnomaly(0)];
      element.anomalies = anomalies;
      element.traceNames = [];
      assert.isTrue(element.hasLabels());
    });

    it('returns false if there are no labels', () => {
      const anomaly = dummyAnomaly(0);
      anomaly.bug_labels = [];
      const anomalies = [anomaly];
      element.anomalies = anomalies;
      element.traceNames = [];
      assert.isFalse(element.hasLabels());
    });
  });
});
