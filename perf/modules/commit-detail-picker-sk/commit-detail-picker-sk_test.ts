import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { CommitDetailPickerSk } from './commit-detail-picker-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Commit, CommitNumber } from '../json';

describe('commit-detail-picker-sk', () => {
  const newInstance = setUpElementUnderTest<CommitDetailPickerSk>('commit-detail-picker-sk');

  let element: CommitDetailPickerSk;

  beforeEach(() => {
    void fetchMock.post('/_/cidRange/', [
      {
        author: 'alice@example.com',
        message: 'Fixed a bug',
        url: 'https://skia.googlesource.com/infra/+/1',
        ts: 1600000000,
        hash: '1111111111111111',
        offset: CommitNumber(100),
        body: 'Body 1',
      } as Commit,
    ]);
  });

  afterEach(() => {
    fetchMock.restore();
  });

  it('renders and loads commits', async () => {
    element = newInstance();
    element.selection = CommitNumber(100);
    await new Promise((resolve) => setTimeout(resolve, 0));
    const _ = await fetchMock.flush(true);

    const button = element.querySelector('button')!;
    assert.include(button.textContent, 'alice@example.com');
  });

  it('opens and closes the dialog', async () => {
    element = newInstance();
    const __ = await fetchMock.flush(true);

    const dialog = element.querySelector('dialog')!;
    assert.isFalse(dialog.open);

    element.querySelector<HTMLButtonElement>('button')!.click();
    assert.isTrue(dialog.open);

    element.querySelector<HTMLButtonElement>('.close-dialog')!.click();
    assert.isFalse(dialog.open);
  });

  it('updates when selection is set', async () => {
    element = newInstance();
    const ___ = await fetchMock.flush(true);

    void fetchMock.post(
      '/_/cidRange/',
      [
        {
          author: 'bob@example.com',
          message: 'Added a feature',
          url: 'https://skia.googlesource.com/infra/+/2',
          ts: 1600000060,
          hash: '2222222222222222',
          offset: CommitNumber(101),
          body: 'Body 2',
        } as Commit,
      ],
      { overwriteRoutes: true }
    );

    element.selection = CommitNumber(101);
    await new Promise((resolve) => setTimeout(resolve, 0));
    const ____ = await fetchMock.flush(true);

    const button = element.querySelector('button')!;
    assert.include(button.textContent, 'bob@example.com');
  });
});
