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

  it('is hidden when traceid is invalid', () => {
    element.traceid = '';
    const controls = element.querySelector('#controls')!;
    assert.isTrue(controls.hasAttribute('hidden'));
  });

  it('is visible when traceid is valid', () => {
    element.traceid = ',config=8888,';
    const controls = element.querySelector('#controls')!;
    assert.isFalse(controls.hasAttribute('hidden'));
  });

  it('loads source when "View Json File" is clicked', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);

    const response = { foo: 'bar' };
    fetchMock.post('/_/details/', response);

    element.querySelector<HTMLButtonElement>('#view-source')!.click();
    const _ = await fetchMock.flush(true);
    await new Promise((resolve) => setTimeout(resolve, 0));

    const dialog = element.querySelector<HTMLDialogElement>('#json-dialog')!;
    assert.isTrue(dialog.open);
    assert.include(element.querySelector('pre')!.textContent, '"foo": "bar"');
  });

  it('loads source when "View Short Json File" is clicked', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);

    const response = { short: 'data' };
    fetchMock.post('/_/details/?results=false', response);

    element.querySelector<HTMLButtonElement>('#load-source')!.click();
    const _ = await fetchMock.flush(true);
    await new Promise((resolve) => setTimeout(resolve, 0));

    const dialog = element.querySelector<HTMLDialogElement>('#json-dialog')!;
    assert.isTrue(dialog.open);
    assert.include(element.querySelector('pre')!.textContent, '"short": "data"');
  });

  it('closes the dialog', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    fetchMock.post('/_/details/', {});

    element.querySelector<HTMLButtonElement>('#view-source')!.click();
    const _ = await fetchMock.flush(true);
    await new Promise((resolve) => setTimeout(resolve, 0));

    const dialog = element.querySelector<HTMLDialogElement>('#json-dialog')!;
    assert.isTrue(dialog.open);

    element.querySelector<HTMLButtonElement>('#closeIcon')!.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.isFalse(dialog.open);
    assert.isNull(element.querySelector('pre'));
  });

  it('handles fetch error', async () => {
    element.traceid = ',config=8888,';
    element.cid = CommitNumber(100);
    fetchMock.post('/_/details/', { status: 500, body: 'Error' });

    const errorPromise = new Promise<void>((resolve) => {
      document.addEventListener('error-sk', () => resolve(), { once: true });
    });

    element.querySelector<HTMLButtonElement>('#view-source')!.click();
    const _ = await fetchMock.flush(true);

    await errorPromise;
    const dialog = element.querySelector<HTMLDialogElement>('#json-dialog')!;
    assert.isFalse(dialog.open);
  });
});
