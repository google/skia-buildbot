import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { JSONSourceSk } from './json-source-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber } from '../json';

describe('json-source-sk', () => {
  const newInstance = setUpElementUnderTest<JSONSourceSk>('json-source-sk');

  let element: JSONSourceSk;
  beforeEach(async () => {
    element = newInstance();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });

  afterEach(() => {
    fetchMock.restore();
  });

  it('is hidden when traceid is invalid', async () => {
    element.traceid = '';
    await element.updateComplete;
    const controls = element.querySelector('.controls')!;
    assert.isTrue(controls.hasAttribute('hidden'));
  });

  it('is visible when traceid is valid', async () => {
    element.traceid = ',config=8888,';
    await element.updateComplete;
    const controls = element.querySelector('.controls')!;
    assert.isFalse(controls.hasAttribute('hidden'));
  });

  it('loads source when "View Json File" is clicked', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    await element.updateComplete;

    const response = { foo: 'bar' };
    fetchMock.post('/_/details/', response);

    element.querySelector<HTMLButtonElement>('.view-source')!.click();
    await element.updateComplete; // State change triggered task

    // Wait for fetch to complete and task to update
    await fetchMock.flush(true);
    await element.updateComplete;

    const dialog = element.querySelector<HTMLDialogElement>('.json-dialog')!;
    assert.isTrue(dialog.open);
    assert.include(element.querySelector('pre')!.textContent, '"foo": "bar"');
  });

  it('loads source when "View Short Json File" is clicked', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    await element.updateComplete;

    const response = { short: 'data' };
    fetchMock.post('/_/details/?results=false', response);

    element.querySelector<HTMLButtonElement>('.load-source')!.click();
    await element.updateComplete;

    await fetchMock.flush(true);
    await element.updateComplete;

    const dialog = element.querySelector<HTMLDialogElement>('.json-dialog')!;
    assert.isTrue(dialog.open);
    assert.include(element.querySelector('pre')!.textContent, '"short": "data"');
  });

  it('closes the dialog', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    await element.updateComplete;

    fetchMock.post('/_/details/', {});

    element.querySelector<HTMLButtonElement>('.view-source')!.click();
    await element.updateComplete;

    await fetchMock.flush(true);
    await element.updateComplete;

    const dialog = element.querySelector<HTMLDialogElement>('.json-dialog')!;
    assert.isTrue(dialog.open);

    element.querySelector<HTMLButtonElement>('.closeIcon')!.click();
    await element.updateComplete;

    assert.isFalse(dialog.open);
  });

  it('handles fetch error', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    await element.updateComplete;

    fetchMock.post('/_/details/', { status: 500, body: 'Error' });
    const errorPromise = new Promise<void>((resolve) => {
      document.addEventListener('error-sk', () => resolve(), { once: true });
    });

    element.querySelector<HTMLButtonElement>('.view-source')!.click();
    await element.updateComplete;

    await fetchMock.flush(true);
    await element.updateComplete;

    await errorPromise;
    const dialog = element.querySelector<HTMLDialogElement>('.json-dialog')!;
    assert.isTrue(dialog.open, 'Dialog should be open to show error or spinner');
    // We can check if error message is displayed
    assert.isNotNull(element.querySelector('.error'));
  });
});
