import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import { languageList } from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../golden/modules/test_util';

describe('suggest-input-sk', () => {
  const newInstance = setUpElementUnderTest('suggest-input-sk');

  it('hides suggestions initially', () => {
    const suggestInput = newInstance();
    suggestInput.options = languageList;
    expect($$('div', suggestInput)).to.have.property('hidden', true);
  });

  it('shows suggestions when in focus', () => {
    const suggestInput = newInstance();
    suggestInput.options = languageList;
    $$('input', suggestInput).focus();
    expect($$('div', suggestInput)).to.have.property('hidden', false);
    expect($('li', suggestInput).length).to.equal(languageList.length);
  });

  it('hides suggestions when loses focus', () => {
    const suggestInput = newInstance();
    suggestInput.options = languageList;
    $$('input', suggestInput).focus();
    $$('input', suggestInput).blur();
    expect($$('div', suggestInput)).to.have.property('hidden', true);
  });

  it('shows only suggestions that substring match', () => {
    const suggestInput = newInstance();
    suggestInput.options = languageList;
    $$('input', suggestInput).focus();
    $$('input', suggestInput).value = "script";
    expect($$('div', suggestInput)).to.have.property('hidden', false);
    expect($('li', suggestInput).length).to.equal(3);
  });

/*
  it('delete option shown', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', false);
  });

  it('delete option hidden', async () => {
    const table = await loadTableWithReplies([singleResultNoDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', true);
  });

  it('delete flow works', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('dialog', table)).to.have.property('open', false);
    $$('delete-icon-sk', table).click();
    expect($$('dialog', table)).to.have.property('open', true);
    fetchMock.postOnce((url, options) => url.startsWith('/_/delete_') && options.body === JSON.stringify({ id: 1 }), 200);
    // TODO(weston): Update common-sk/confirm-dialog-sk to make this less
    // brittle.
    $$('dialog', table).querySelectorAll('button')[1].click();
    expect($$('dialog', table)).to.have.property('open', false);
  });

  it('task details works', async () => {
    const table = await loadTableWithReplies([resultSetOneItem]);

    expect($$('.dialog-background', table)).to.have.class('hidden');
    $$('.details', table).click();

    expect($$('.dialog-background', table)).to.not.have.class('hidden');
  });*/
});
