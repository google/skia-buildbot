import './index';
import { assert } from 'chai';
import {
  CommitDetailPanelSk,
  CommitDetailPanelSkCommitSelectedDetails,
} from './commit-detail-panel-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Commit, CommitNumber } from '../json';

describe('commit-detail-panel-sk', () => {
  const newInstance = setUpElementUnderTest<CommitDetailPanelSk>('commit-detail-panel-sk');

  let element: CommitDetailPanelSk;
  beforeEach(() => {
    element = newInstance();
  });

  const commits: Commit[] = [
    {
      author: 'alice@example.com',
      message: 'Fixed a bug',
      url: 'https://skia.googlesource.com/infra/+/1',
      ts: 1600000000,
      hash: '1111111111111111',
      offset: CommitNumber(100),
      body: 'Body 1',
    },
    {
      author: 'bob@example.com',
      message: 'Added a feature',
      url: 'https://skia.googlesource.com/infra/+/2',
      ts: 1600000060,
      hash: '2222222222222222',
      offset: CommitNumber(101),
      body: 'Body 2',
    },
  ];

  it('renders a list of commits', () => {
    element.details = commits;
    const rows = element.querySelectorAll('tr');
    assert.equal(rows.length, 2);
    assert.include(rows[0].textContent, 'alice@example.com');
    assert.include(rows[1].textContent, 'bob@example.com');
  });

  it('hides commits when hide is true', () => {
    element.details = commits;
    element.hide = true;
    assert.equal(element.querySelectorAll('tr').length, 0);
  });

  it('selects a commit and emits event', async () => {
    element.details = commits;
    element.selectable = true;

    const eventPromise = new Promise<CustomEvent<CommitDetailPanelSkCommitSelectedDetails>>(
      (resolve) => {
        element.addEventListener(
          'commit-selected',
          (e) => {
            resolve(e as CustomEvent<CommitDetailPanelSkCommitSelectedDetails>);
          },
          { once: true }
        );
      }
    );

    const secondRow = element.querySelectorAll('tr')[1];
    secondRow.click();

    const event = await eventPromise;
    assert.equal(event.detail.selected, 1);
    assert.equal(event.detail.commit.author, 'bob@example.com');
    assert.equal(element.selected, 1);
  });

  it('does not emit event if not selectable', () => {
    element.details = commits;
    element.selectable = false;

    let eventEmitted = false;
    element.addEventListener('commit-selected', () => {
      eventEmitted = true;
    });

    const firstRow = element.querySelectorAll('tr')[0];
    firstRow.click();

    assert.isFalse(eventEmitted);
    assert.equal(element.selected, -1);
  });

  it('renders nothing if details list is empty', () => {
    element.details = [];
    assert.equal(element.querySelectorAll('tr').length, 0);
  });

  it('highlights the selected row when selected property is set', async () => {
    element.details = commits;
    element.selectable = true;
    element.selected = 1;

    // Wait for render
    await new Promise((resolve) => setTimeout(resolve, 0));

    const rows = element.querySelectorAll('tr');
    assert.isFalse(rows[0].hasAttribute('selected'));
    assert.isTrue(rows[1].hasAttribute('selected'));
  });

  it('enables selection when selectable is toggled to true', async () => {
    element.details = commits;
    element.selectable = false;
    element.selectable = true;

    const eventPromise = new Promise<CustomEvent<CommitDetailPanelSkCommitSelectedDetails>>(
      (resolve) => {
        element.addEventListener(
          'commit-selected',
          (e) => {
            resolve(e as CustomEvent<CommitDetailPanelSkCommitSelectedDetails>);
          },
          { once: true }
        );
      }
    );

    const firstRow = element.querySelectorAll('tr')[0];
    firstRow.click();

    const event = await eventPromise;
    assert.equal(event.detail.selected, 0);
  });

  it('selects correctly when clicking a nested element', async () => {
    element.details = commits;
    element.selectable = true;

    const eventPromise = new Promise<CustomEvent<CommitDetailPanelSkCommitSelectedDetails>>(
      (resolve) => {
        element.addEventListener(
          'commit-selected',
          (e) => {
            resolve(e as CustomEvent<CommitDetailPanelSkCommitSelectedDetails>);
          },
          { once: true }
        );
      }
    );

    // Click the commit-detail-sk inside the cell, which is nested in the tr
    const nestedElement = element.querySelector('commit-detail-sk');
    assert.isNotNull(nestedElement);
    (nestedElement as HTMLElement).click();

    const event = await eventPromise;
    assert.equal(event.detail.selected, 0);
  });

  it('passes trace_id to children', async () => {
    element.details = commits;
    element.trace_id = 'test_trace_id';

    await new Promise((resolve) => setTimeout(resolve, 0));
    const detail = element.querySelector('commit-detail-sk');
    assert.isNotNull(detail);
    assert.equal((detail as any).trace_id, 'test_trace_id');
  });
});
