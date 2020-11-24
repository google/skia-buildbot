import './index';

import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import { chromiumPatchResult } from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('patch-sk', () => {
  const newInstance = setUpElementUnderTest('patch-sk');
  fetchMock.config.overwriteRoutes = false;

  let patchSk;
  beforeEach(() => {
    patchSk = newInstance((ele) => {
      ele.patchType = 'chromium';
    });
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  const simulateClInput = (cl, resp) => {
    if (cl.length >= 3) {
      fetchMock.postOnce('begin:/_/cl_data', resp);
    }
    const input = $$('input-sk', patchSk);
    input.focus();
    input.value = cl;
    input.dispatchEvent(new Event('input', {
      bubbles: true,
      cancelable: true,
    }));
  };
  const simulatePatchEdit = (addition) => {
    const exTextarea = $$('expandable-textarea-sk', patchSk);
    $$('button', exTextarea).click();
    exTextarea.value = exTextarea.value + addition;
    exTextarea.dispatchEvent(new Event('input', {
      bubbles: true,
      cancelable: true,
    }));
  };

  it('loads a valid cl', async () => {
    const event = eventPromise('cl-description-changed');
    simulateClInput('123', chromiumPatchResult);
    await event;

    expect(patchSk).to.have.property('cl', '123');
    expect(patchSk.clDescription).to.contain('googlesource.com')
      .and.to.contain('Roll Skia');
    expect(patchSk.patch).to.contain('diff --git');
  });

  it('supports patch edit', async () => {
    let event = eventPromise('cl-description-changed');
    simulateClInput('123', chromiumPatchResult);
    await event;
    event = eventPromise('patch-changed');
    simulatePatchEdit('sweet new content in my patch');
    await event;

    expect(patchSk.patch).to.contain('new content');
  });

  it('errors on network error', async () => {
    const event = eventPromise('cl-description-changed');
    simulateClInput('123', 503);
    await event;

    expect(patchSk).to.have.nested.property('_clData.error.message',
      'Bad network response: Service Unavailable');
    expect(patchSk.clDescription).to.equal('');
  });

});
