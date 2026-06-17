import './index';
import { TriagePanelSk } from './triage-panel-sk';
import { TriageBucket, updateBucketDirtyState } from './buckets-controller';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import { Anomaly } from '../json';

describe('triage-panel-sk', () => {
  const newInstance = setUpElementUnderTest<TriagePanelSk>('triage-panel-sk');
  let element: TriagePanelSk;

  const mockAnomaly1: Anomaly = {
    id: '100',
    test_path: 'Master/Bot/Benchmark/Story/Metric1',
    bug_id: 0,
    start_revision: 123,
    end_revision: 124,
    display_commit_number: 124,
    is_improvement: false,
    recovered: false,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 10,
    median_after_anomaly: 20,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  };

  const mockAnomaly2: Anomaly = {
    id: '101',
    test_path: 'Master/Bot/Benchmark/Story/Metric2',
    bug_id: 0,
    start_revision: 125,
    end_revision: 126,
    display_commit_number: 126,
    is_improvement: false,
    recovered: false,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 15,
    median_after_anomaly: 30,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  };

  beforeEach(() => {
    localStorage.clear();
    element = newInstance();
  });

  describe('Buckets Management', () => {
    it('initializes with empty buckets', () => {
      assert.isEmpty(element.buckets);
    });

    it('adds a new bucket', () => {
      element.newBucketName = 'Regression A';
      element.addNewBucket();
      assert.lengthOf(element.buckets, 1);
      assert.equal(element.buckets[0].name, 'Regression A');
      assert.equal(element.buckets[0].triageState, 'NEW');
    });

    it('deletes a bucket via event', () => {
      element.newBucketName = 'Regression A';
      element.addNewBucket();
      assert.lengthOf(element.buckets, 1);

      element.dispatchEvent(
        new CustomEvent('bucket-deleted', { detail: { name: 'Regression A' }, bubbles: true })
      );
      assert.isEmpty(element.buckets);
    });

    it('updates a bucket via event', () => {
      element.newBucketName = 'Regression A';
      element.addNewBucket();
      const updatedBucket: TriageBucket = {
        name: 'Regression A',
        anomalies: [mockAnomaly1],
        triageState: 'NEW',
      };

      element.dispatchEvent(
        new CustomEvent('bucket-updated', { detail: { bucket: updatedBucket }, bubbles: true })
      );
      assert.lengthOf(element.buckets[0].anomalies, 1);
    });

    it('stages anomalies into a bucket', () => {
      element.newBucketName = 'Regression A';
      element.addNewBucket();
      element.addToBucket('Regression A', [mockAnomaly1]);
      assert.lengthOf(element.buckets[0].anomalies, 1);
      assert.equal(element.buckets[0].anomalies[0].id, '100');
    });

    it('gets all staged anomaly ids across buckets', () => {
      element.newBucketName = 'Bucket 1';
      element.addNewBucket();
      element.addToBucket('Bucket 1', [mockAnomaly1]);

      element.newBucketName = 'Bucket 2';
      element.addNewBucket();
      element.addToBucket('Bucket 2', [mockAnomaly2]);

      const staged = element.getStagedAnomalyIds();
      assert.equal(staged.size, 2);
      assert.isTrue(staged.has('100'));
      assert.isTrue(staged.has('101'));
    });
  });

  describe('Triage States (NEW -> APPLIED -> DIRTY)', () => {
    it('maintains NEW state when adding anomalies to an untriaged bucket', () => {
      const bucket: TriageBucket = {
        name: 'Untriaged',
        anomalies: [mockAnomaly1],
        triageState: 'NEW',
      };
      const updated = updateBucketDirtyState(bucket);
      assert.equal(updated.triageState, 'NEW', 'Bucket remains NEW until initial triage action');
    });

    it('transitions to APPLIED when initial triage action is performed', () => {
      const triagedBucket: TriageBucket = {
        name: 'Triaged',
        anomalies: [mockAnomaly1],
        triageState: 'APPLIED',
        actionType: 'EXISTING_BUG',
        bugId: 12345,
        lastAppliedIds: ['100'],
      };
      const updated = updateBucketDirtyState(triagedBucket);
      assert.equal(
        updated.triageState,
        'APPLIED',
        'Bucket is APPLIED when all anomalies match lastAppliedIds'
      );
    });

    it('transitions to DIRTY when new unapplied anomalies are staged into an APPLIED bucket', () => {
      const triagedBucket: TriageBucket = {
        name: 'Triaged',
        anomalies: [mockAnomaly1, mockAnomaly2], // mockAnomaly2 was just added
        triageState: 'APPLIED',
        actionType: 'EXISTING_BUG',
        bugId: 12345,
        lastAppliedIds: ['100'], // only mockAnomaly1 was previously applied
      };
      const updated = updateBucketDirtyState(triagedBucket);
      assert.equal(
        updated.triageState,
        'DIRTY',
        'Bucket becomes DIRTY because mockAnomaly2 is unapplied'
      );
    });

    it('returns to APPLIED once pending changes are applied', () => {
      const dirtyBucket: TriageBucket = {
        name: 'Triaged',
        anomalies: [mockAnomaly1, mockAnomaly2],
        triageState: 'DIRTY',
        actionType: 'EXISTING_BUG',
        bugId: 12345,
        lastAppliedIds: ['100', '101'], // user clicked Apply (Pending), updating lastAppliedIds
      };
      const updated = updateBucketDirtyState(dirtyBucket);
      assert.equal(
        updated.triageState,
        'APPLIED',
        'Bucket returns to APPLIED after lastAppliedIds is updated'
      );
    });
  });
});
