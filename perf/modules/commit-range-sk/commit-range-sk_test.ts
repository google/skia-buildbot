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
      instance_url: '',
      commit_range_url: 'http://example.com/range/+log/{begin}..{end}',
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
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: 'fake-tag',
      remove_default_stat_value: false,
      enable_skia_bridge_aggregation: false,
      show_json_file_display: false,
      always_show_commit_info: false,
      show_triage_link: true,
      show_bisect_btn: true,
    };

    element = newInstance((el: CommitRangeSk) => {
      el.commitIndex = 2;
      el.header = [
        {
          offset: CommitNumber(64809),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(64810),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(64811),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ];
    });
  });

  describe('converts commit ids to hashes', () => {
    it('ignores MISSING_DATA_SENTINEL', async () => {
      // eslint-disable-next-line dot-notation
      element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
        assert.deepEqual(cids, [64809, 64811]);
        return ['11111111111111111111111111111', '33333333333333333333333333333'];
      };
      // The MISSING_DATA_SENTINEL should be skipped.
      element.trace = [12, MISSING_DATA_SENTINEL, 13];
      await element.recalcLink();
      assert.equal(
        element.querySelector<HTMLAnchorElement>('a')!.href,
        'http://example.com/range/+log/11111111111111111111111111111..33333333333333333333333333333'
      );
    });
  });
  it('returns the previous hash if there are no missing commits', async () => {
    // eslint-disable-next-line dot-notation
    element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
      // There were no commits to skip, so return the two consecutive hashes.
      assert.deepEqual(cids, [64810, 64811]);
      return ['11111111111111111111111111111', '22222222222222222222222222222'];
    };
    element.trace = [11, 12, 13];
    await element.recalcLink();
    assert.equal(
      element.querySelector<HTMLAnchorElement>('a')!.href,
      'http://example.com/range/+/22222222222222222222222222222'
    );
  });
  it('handles GitHub commit URL format', async () => {
    window.perf.commit_range_url = 'https://github.com/example/repo/commits/{end}';
    // eslint-disable-next-line dot-notation
    element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
      assert.deepEqual(cids, [64810, 64811]);
      return ['11111111111111111111111111111', '22222222222222222222222222222'];
    };
    element.trace = [11, 12, 13];
    await element.recalcLink();
    assert.equal(
      element.querySelector<HTMLAnchorElement>('a')!.href,
      'https://github.com/example/repo/commits/22222222222222222222222222222'
    );
  });
});
