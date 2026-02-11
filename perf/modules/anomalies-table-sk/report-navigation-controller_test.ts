import './report-navigation-controller';
import '../window/window';
import { ReportNavigationController } from './report-navigation-controller';
import { Anomaly, GetGroupReportResponse } from '../json';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

const dummyAnomaly = (
  id: string,
  bugId: number,
  startRev: number,
  endRev: number,
  testPath: string
): Anomaly => {
  return {
    id: id,
    bug_id: bugId,
    start_revision: startRev,
    end_revision: endRev,
    test_path: testPath,
    is_improvement: false,
    recovered: false,
    state: 'new',
    statistic: 'avg',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 0,
    median_after_anomaly: 0,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: 'test',
    bug_component: 'test',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  };
};

describe('ReportNavigationController', () => {
  let controller: ReportNavigationController;
  let host: any;

  beforeEach(() => {
    host = {
      addController: sinon.spy(),
      requestUpdate: sinon.spy(),
    };
    controller = new ReportNavigationController(host);
    window.perf = { fetch_anomalies_from_sql: false } as any;
  });

  afterEach(() => {
    fetchMock.reset();
    sinon.restore();
    delete (window as any).perf;
  });

  describe('openReportForAnomalyIds', () => {
    it('opens URL with anomaly ID when single anomaly is provided', async () => {
      const openStub = sinon.stub(window, 'open');
      const anomalies = [dummyAnomaly('123', 0, 100, 200, 'test/path')];

      await controller.openReportForAnomalyIds(anomalies);

      assert.isTrue(openStub.calledWith('/u/?anomalyIDs=123', '_blank'));
    });

    it('opens URL with SID for multiple anomalies', async () => {
      const openStub = sinon.stub(window, 'open');
      const anomalies = [
        dummyAnomaly('123', 0, 100, 200, 'test/path/1'),
        dummyAnomaly('456', 0, 100, 200, 'test/path/2'),
      ];

      fetchMock.post('/_/anomalies/group_report', {
        sid: 'test_sid_12345',
        anomaly_list: [],
        selected_keys: [],
        error: '',
        timerange_map: null,
        is_commit_number_based: false,
      } as GetGroupReportResponse);

      await controller.openReportForAnomalyIds(anomalies);

      assert.isTrue(openStub.calledWith('/u/?sid=test_sid_12345', '_blank'));
      assert.isTrue(fetchMock.called('/_/anomalies/group_report'));
    });

    it('handles fetch error gracefully', async () => {
      const openStub = sinon.stub(window, 'open');
      const anomalies = [
        dummyAnomaly('123', 0, 100, 200, 'test/path/1'),
        dummyAnomaly('456', 0, 100, 200, 'test/path/2'),
      ];

      fetchMock.post('/_/anomalies/group_report', 500);

      await controller.openReportForAnomalyIds(anomalies);

      assert.isFalse(openStub.called);
    });
  });

  describe('openMultiGraphUrl', () => {
    it('generates and opens multi-graph URL', async () => {
      const anomaly = dummyAnomaly('123', 0, 100, 200, 'test/path');
      const newTab = {
        location: { href: '' },
        close: sinon.spy(),
      } as unknown as Window;

      fetchMock.post('begin:/_/anomalies/group_report', {
        sid: 'sid',
        timerange_map: {
          '123': { begin: 1000, end: 2000 },
        },
      } as unknown as GetGroupReportResponse);

      fetchMock.post('begin:/_/shortcut/update', { id: 'shortcut_id' });

      await controller.openMultiGraphUrl(anomaly, newTab);

      assert.isFalse(
        (newTab.close as sinon.SinonSpy).called,
        'newTab.close() should not be called'
      );
      assert.include(newTab.location.href, 'shortcut=shortcut_id');
      assert.include(newTab.location.href, 'totalGraphs=1');
      assert.include(newTab.location.href, '/m/?begin=');
    });

    it('closes new tab on fetch failure', async () => {
      const anomaly = dummyAnomaly('123', 0, 100, 200, 'test/path');
      const newTab = {
        location: { href: '' },
        close: sinon.spy(),
      } as unknown as Window;

      fetchMock.post('/_/anomalies/group_report', 500);

      await controller.openMultiGraphUrl(anomaly, newTab);

      assert.isTrue((newTab.close as sinon.SinonSpy).calledOnce);
    });
  });
});
