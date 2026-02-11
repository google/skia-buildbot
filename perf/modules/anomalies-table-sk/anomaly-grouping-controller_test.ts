import { assert } from 'chai';
import { AnomalyGroupingController } from './anomaly-grouping-controller';
import { ReactiveControllerHost } from 'lit';
import { Anomaly } from '../json';
import { RevisionGroupingMode } from './grouping';
import * as sinon from 'sinon';

const GROUPING_CONFIG_STORAGE_KEY = 'perf-grouping-config';

class MockHost implements ReactiveControllerHost {
  controllers: any[] = [];

  requestUpdateCalled = false;

  addController(controller: any) {
    this.controllers.push(controller);
  }

  removeController(_controller: any) {}

  requestUpdate() {
    this.requestUpdateCalled = true;
  }

  get updateComplete() {
    return Promise.resolve(true);
  }
}

describe('AnomalyGroupingController', () => {
  let host: MockHost;
  let controller: AnomalyGroupingController;

  const anomaly1: Anomaly = {
    id: '1',
    test_path: 'benchmark/test1',
    bug_id: 123,
    start_revision: 100,
    end_revision: 110,
    is_improvement: false,
    recovered: true,
    state: 'new',
    statistic: 'avg',
    units: 'ms',
    degrees_of_freedom: 1,
    median_before_anomaly: 10,
    median_after_anomaly: 15,
    p_value: 0.01,
    segment_size_after: 10,
    segment_size_before: 10,
    std_dev_before_anomaly: 1,
    t_statistic: 2,
    subscription_name: 'sub1',
    bug_component: 'comp1',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  };

  const anomaly2: Anomaly = {
    id: '2',
    test_path: 'benchmark/test2',
    bug_id: 123,
    start_revision: 100,
    end_revision: 110,
    is_improvement: false,
    recovered: true,
    state: 'new',
    statistic: 'avg',
    units: 'ms',
    degrees_of_freedom: 1,
    median_before_anomaly: 20,
    median_after_anomaly: 25,
    p_value: 0.01,
    segment_size_after: 10,
    segment_size_before: 10,
    std_dev_before_anomaly: 1,
    t_statistic: 2,
    subscription_name: 'sub1',
    bug_component: 'comp1',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  };

  beforeEach(() => {
    host = new MockHost();
    window.localStorage.clear();
    controller = new AnomalyGroupingController(host);
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  describe('Initialization', () => {
    it('initializes with default config when localStorage is empty', () => {
      controller.hostConnected();
      assert.equal(controller.config.revisionMode, 'OVERLAPPING');
      assert.isTrue(controller.config.groupBy.has('BENCHMARK'));
      assert.isTrue(controller.config.groupSingles);
    });

    it('loads config from localStorage if present', () => {
      const storedConfig = {
        revisionMode: 'EXACT' as RevisionGroupingMode,
        groupBy: ['BOT'],
        groupSingles: false,
      };
      window.localStorage.setItem(GROUPING_CONFIG_STORAGE_KEY, JSON.stringify(storedConfig));

      // Re-initialize controller to simulate page load
      controller = new AnomalyGroupingController(host);
      controller.hostConnected();

      assert.equal(controller.config.revisionMode, 'EXACT');
      assert.isTrue(controller.config.groupBy.has('BOT'));
      assert.isFalse(controller.config.groupBy.has('BENCHMARK'));
      assert.isFalse(controller.config.groupSingles);
    });

    it('handles invalid JSON in localStorage', () => {
      window.localStorage.setItem(GROUPING_CONFIG_STORAGE_KEY, '{invalid json');

      const consoleStub = sinon.stub(console, 'error');
      try {
        controller.hostConnected();
        assert.equal(controller.config.revisionMode, 'OVERLAPPING');
        assert.isNull(window.localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY));
      } finally {
        consoleStub.restore();
      }
    });
  });

  describe('Config Updates', () => {
    beforeEach(() => {
      controller.hostConnected();
      host.requestUpdateCalled = false;
    });

    it('setRevisionMode updates config, saves, and refreshes', () => {
      controller.setRevisionMode('EXACT');

      assert.equal(controller.config.revisionMode, 'EXACT');
      assert.isTrue(host.requestUpdateCalled);

      const saved = JSON.parse(window.localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY)!);
      assert.equal(saved.revisionMode, 'EXACT');
    });

    it('toggleGroupBy updates config, saves, and refreshes', () => {
      controller.toggleGroupBy('BOT', true);
      assert.isTrue(controller.config.groupBy.has('BOT'));
      assert.isTrue(host.requestUpdateCalled);

      let saved = JSON.parse(window.localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY)!);
      assert.include(saved.groupBy, 'BOT');

      host.requestUpdateCalled = false;
      controller.toggleGroupBy('BOT', false);
      assert.isFalse(controller.config.groupBy.has('BOT'));

      saved = JSON.parse(window.localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY)!);
      assert.notInclude(saved.groupBy, 'BOT');
    });

    it('setGroupSingles updates config, saves, and refreshes', () => {
      controller.setGroupSingles(false);
      assert.isFalse(controller.config.groupSingles);
      assert.isTrue(host.requestUpdateCalled);

      const saved = JSON.parse(window.localStorage.getItem(GROUPING_CONFIG_STORAGE_KEY)!);
      assert.isFalse(saved.groupSingles);
    });
  });

  describe('Grouping Logic (Integration)', () => {
    beforeEach(() => {
      controller.hostConnected();
    });

    it('setAnomalies creates groups and triggers update', () => {
      controller.setAnomalies([anomaly1, anomaly2]);

      assert.isTrue(host.requestUpdateCalled);
      assert.isNotEmpty(controller.groups);
    });

    it('refreshGrouping is called on config change', () => {
      controller.setAnomalies([anomaly1, anomaly2]);
      host.requestUpdateCalled = false;

      controller.setRevisionMode('ANY');
      assert.isTrue(host.requestUpdateCalled);
    });
  });
});
