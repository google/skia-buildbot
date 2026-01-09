import './index';
import { TriagePageSk } from './triage-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ClusterSummary2Sk } from '../cluster-summary2-sk/cluster-summary2-sk';
import { TriageStatusSkStartTriageEventDetails } from '../triage-status-sk/triage-status-sk';
import { CommitNumber, ReadOnlyParamSet, TimestampSeconds, TraceSet } from '../json';

describe('triage-page-sk', () => {
  const newInstance = setUpElementUnderTest<TriagePageSk>('triage-page-sk');

  let element: TriagePageSk;

  beforeEach(() => {
    element = newInstance((_) => {
      // Mock fetch calls to avoid errors during connection
      fetchMock.post('/_/reg/', {
        header: [],
        table: [],
        categories: [],
      });
    });
  });

  afterEach(() => {
    fetchMock.reset();
  });

  describe('keyboard shortcuts', () => {
    let clusterSummary: ClusterSummary2Sk;
    let dialog: HTMLDialogElement;

    beforeEach(async () => {
      // Simulate starting triage to open the dialog and render cluster-summary2-sk
      dialog = element.querySelector('#triage-dialog')!;
      const eventDetails: TriageStatusSkStartTriageEventDetails = {
        alert: {
          bug_uri_template: '',
          algo: 'kmeans',
          display_name: '',
          interesting: 0,
          owner: '',
          query: '',
          sparse: false,
          state: 'ACTIVE',
          step_up_only: false,
          sub_name: '',
          id_as_string: '123',
          issue_tracker_component: '123' as any,
          alert: '',
          step: '',
          direction: 'BOTH',
          radius: 0,
          k: 0,
          group_by: '',
          minimum_num: 0,
          category: '',
        },
        cluster_type: 'low',
        full_summary: {
          frame: {
            dataframe: {
              traceset: {} as TraceSet,
              header: [
                {
                  offset: CommitNumber(100),
                  timestamp: TimestampSeconds(1234567890),
                  hash: 'abc',
                  author: 'me',
                  message: 'msg',
                  url: 'http://example.com',
                },
              ],
              paramset: {} as ReadOnlyParamSet,
              skip: 0,
              traceMetadata: [],
            },
            skps: [],
            msg: '',
            display_mode: 'display_plot',
            anomalymap: {},
          },
          summary: {
            centroid: [],
            shortcut: 'test_shortcut',
            step_fit: {
              least_squares: 0,
              turning_point: 0,
              step_size: 0,
              regression: 0,
              status: 'Low',
            },
            step_point: {
              offset: CommitNumber(0),
              timestamp: TimestampSeconds(0),
              hash: '',
              author: '',
              message: '',
              url: '',
            },
            num: 0,
            ts: '',
            param_summaries2: [],
          },
          triage: {
            message: '',
            status: 'untriaged',
          },
        },
        triage: {
          message: '',
          status: 'untriaged',
        },
        element: element.querySelector('triage-status-sk')!,
      };

      // We need to trigger the triage_start method.
      // Since it's private, we can dispatch the event it listens to,
      // OR we can just manually set the state and call render if we could access it.
      // But the event listener is on the table, which is in the template.
      // Let's try calling the private method via 'any' cast or dispatching event on the table.
      // The table has `@start-triage=${ele.triage_start}`.
      // Call triage_start manually to ensure it runs
      (element as any).triage_start(
        new CustomEvent<TriageStatusSkStartTriageEventDetails>('start-triage', {
          detail: eventDetails,
          bubbles: true,
        })
      );

      // Wait for render
      await new Promise((resolve) => setTimeout(resolve, 0));

      clusterSummary = element.querySelector('cluster-summary2-sk')!;
      assert.isNotNull(clusterSummary);
      assert.isTrue(dialog.open);
    });

    it('sets status to positive on "p" key', () => {
      let updateCalled = false;
      clusterSummary.update = () => {
        updateCalled = true;
      };

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'p' }));

      assert.equal(clusterSummary.triage.status, 'positive');
      assert.isTrue(updateCalled);
    });

    it('sets status to negative on "n" key', () => {
      let updateCalled = false;
      clusterSummary.update = () => {
        updateCalled = true;
      };

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'n' }));

      assert.equal(clusterSummary.triage.status, 'negative');
      assert.isTrue(updateCalled);
    });

    it('opens dashboard with correct URL format on "g" key', () => {
      const windowOpenStub = sinon.stub(window, 'open');

      // Simulate "g" key press which triggers
      // onOpenReport -> clusterSummary.openShortcut -> openKeys
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));

      assert.isTrue(windowOpenStub.calledOnce);
      const url = windowOpenStub.firstCall.args[0] as string;

      // Verify no spaces around 'e' and query params
      assert.match(url, /^\/e\/\?/, 'URL should start with /e/?');
      assert.notMatch(url, /%20/, 'URL should not contain encoded spaces');

      // Verify important query parameters are present
      assert.include(url, 'keys=test_shortcut', 'URL should contain correct keys');
      assert.include(url, 'xbaroffset=0', 'URL should contain correct xbaroffset');
      assert.include(url, 'num_commits=50', 'URL should contain num_commits');
      assert.include(url, 'request_type=1', 'URL should contain request_type');

      windowOpenStub.restore();
    });

    it('does not trigger if dialog is closed', () => {
      dialog.close();
      let updateCalled = false;
      clusterSummary.update = () => {
        updateCalled = true;
      };

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'n' }));

      assert.isFalse(updateCalled);
    });

    it('opens the correct dialog (avoiding ambiguity with calendar-input-sk)', () => {
      assert.equal(dialog.id, 'triage-dialog');
      assert.isTrue(dialog.open);

      // Ensure we didn't accidentally open a calendar dialog (which doesn't have this ID)
      const allDialogs = element.querySelectorAll('dialog');
      // Triage dialog + 2 calendar input dialogs (begin/end) = 3 dialogs total
      // But only triage dialog should be open.

      let openDialogCount = 0;
      allDialogs.forEach((d) => {
        if (d.open) openDialogCount++;
      });

      assert.equal(openDialogCount, 1, 'Only one dialog should be open');
    });
  });
});
