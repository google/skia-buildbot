import './index';
import { assert } from 'chai';
import { CommitRangeSk } from './commit-range-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber, TimestampSeconds } from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';

describe('commit-range-sk', () => {
  const newInstance = setUpElementUnderTest<CommitRangeSk>('commit-range-sk');

  let element: CommitRangeSk;
  beforeEach(() => {
    window.perf = {
      commit_range_url: 'http://example.com/range/{begin}/{end}',
      key_order: ['config'],
      demo: true,
      radius: 7,
      num_shift: 10,
      interesting: 25,
      step_up_only: false,
      display_group_by: true,
      hide_list_of_commits_on_explore: false,
      notifications: 'none',
      fetch_chrome_perf_anomalies: false,
      feedback_url: '',
      chat_url: '',
      help_url_override: '',
      trace_format: '',
      need_alert_action: false,
      bug_host_url: '',
      git_repo_url: '',
      keys_for_commit_range: [],
    };

    element = newInstance((el: CommitRangeSk) => {
      el.commitIndex = 2;
      el.header = [
        {
          offset: CommitNumber(64809),
          timestamp: TimestampSeconds(0),
        },
        {
          offset: CommitNumber(64810),
          timestamp: TimestampSeconds(0),
        },
        {
          offset: CommitNumber(64811),
          timestamp: TimestampSeconds(0),
        },
      ];
    });
  });

  describe('converts commit ids to hashes', () => {
    it('ignores MISSING_DATA_SENTINEL', async () => {
      // eslint-disable-next-line dot-notation
      element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
        assert.deepEqual(cids, [64809, 64811]);
        return [
          '1111111111111111111111111111111111111111',
          '3333333333333333333333333333333333333333',
        ];
      };
      // The MISSING_DATA_SENTINEL should be skipped.
      element.trace = [12, MISSING_DATA_SENTINEL, 13];
      await element.recalcLink();
      assert.equal(
        element.querySelector<HTMLAnchorElement>('a')!.href,
        'http://example.com/range/1111111111111111111111111111111111111111/3333333333333333333333333333333333333333'
      );
    });
  });
  it('returns the previous hash if there are no missing commits', async () => {
    // eslint-disable-next-line dot-notation
    element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
      // There were no commits to skip, so return the two consecutive hashes.
      assert.deepEqual(cids, [64810, 64811]);

      return [
        '1111111111111111111111111111111111111111',
        '2222222222222222222222222222222222222222',
      ];
    };
    element.trace = [11, 12, 13];
    await element.recalcLink();
    assert.equal(
      element.querySelector<HTMLAnchorElement>('a')!.href,
      'http://example.com/range/1111111111111111111111111111111111111111/2222222222222222222222222222222222222222'
    );
  });
});
