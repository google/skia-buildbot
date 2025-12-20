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
});
