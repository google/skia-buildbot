import './index';

import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  eventPromise,
  eventSequencePromise,
  expectQueryStringToEqual,
  setQueryString,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { fakeNow, ignoreRules_10 } from './test_data';

describe('ignores-page-sk', () => {
  const newInstance = setUpElementUnderTest('ignores-page-sk');

  const regularNow = Date.now;
  let ignoresPageSk;

  beforeEach(async () => {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');
    // These will get called on page load.
    fetchMock.get('/json/v1/ignores?counts=1', ignoreRules_10);
    // We only need a few params to make sure the edit-ignore-rule-dialog works properly and it
    // does not matter really what they are, so we use a small subset of actual params.
    const someParams = {
      alpha_type: ['Opaque', 'Premul'],
      arch: ['arm', 'arm64', 'x86', 'x86_64'],
    };
    fetchMock.get('/json/v1/paramset', someParams);
    // set the time to our mocked Now
    Date.now = () => fakeNow;

    const event = eventPromise('end-task');
    ignoresPageSk = newInstance();
    await event;
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.

    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
    // reset the time
    Date.now = regularNow;
  });

  describe('html layout', () => {
    it('should make a table with 10 rows in the body', () => {
      const rows = $('table tbody tr', ignoresPageSk);
      expect(rows).to.have.length(10);
    });

    it('creates links to test the filter', () => {
      const rows = $('table tbody tr', ignoresPageSk);
      const queryLink = $$('.query a', rows[9]);
      expect(queryLink.href).to.contain(
        'include=true&query=config%3Dglmsaa4%26cpu_or_gpu_value%3DTegraX1%26name%3Drg1024_green_grapes.svg',
      );
      expect(queryLink.textContent).to.equal(
        'config=glmsaa4\ncpu_or_gpu_value=TegraX1\nname=rg1024_green_grapes.svg',
      );
    });

    it('has some expired and some not expired rules', () => {
      const rows = $('table tbody tr', ignoresPageSk);
      const firstRow = rows[0];
      expect(firstRow.className).to.contain('expired');
      let timeBox = $$('.expired', firstRow);
      expect(timeBox.innerText).to.contain('Expired');

      const fourthRow = rows[4];
      expect(fourthRow.className).to.not.contain('expired');
      timeBox = $$('.expired', fourthRow);
      expect(timeBox).to.be.null;
    });
  }); // end describe('html layout')

  describe('interaction', () => {
    it('toggles between counting traces with untriaged digests and all traces', () => {
      let checkbox = findUntriagedDigestsCheckbox(ignoresPageSk);
      expect(checkbox.checked).to.be.true;
      expect(findMatchesTextForRow(2, ignoresPageSk)).to.contain('0 / 4');
      expectQueryStringToEqual('');

      clickUntriagedDigestsCheckbox(ignoresPageSk);

      checkbox = findUntriagedDigestsCheckbox(ignoresPageSk);
      expect(checkbox.checked).to.be.false;
      expect(findMatchesTextForRow(2, ignoresPageSk)).to.contain('6 / 10');
      expectQueryStringToEqual('?count_all=true');

      clickUntriagedDigestsCheckbox(ignoresPageSk);

      checkbox = findUntriagedDigestsCheckbox(ignoresPageSk);
      expect(checkbox.checked).to.be.true;
      expect(findMatchesTextForRow(2, ignoresPageSk)).to.contain('0 / 4');
      expectQueryStringToEqual('');
    });

    it('responds to back and forward browser buttons', async () => {
      // Create some mock history so we can use the back button.
      setQueryString('?count_all=true');
      setQueryString('');

      // We should go back to the count_all=true setting
      await goBack();
      let checkbox = findUntriagedDigestsCheckbox(ignoresPageSk);
      expect(checkbox.checked).to.be.false;

      // And now return to the default view.
      await goForward();
      checkbox = findUntriagedDigestsCheckbox(ignoresPageSk);
      expect(checkbox.checked).to.be.true;
    });

    it('prompts "are you sure" before deleting an ignore rule', () => {
      const dialog = findConfirmDeleteDialog(ignoresPageSk);
      expect(dialog.hasAttribute('open')).to.be.false;

      const del = findDeleteForRow(2);
      del.click();

      expect(dialog.hasAttribute('open')).to.be.true;
      const msg = $$('.message', dialog);
      expect(msg.innerText).to.contain('Are you sure you want to delete');
    });

    it('deletes an existing ignore rule', async () => {
      const idOfThirdRule = '7589748925671328782';
      const del = findDeleteForRow(2);
      del.click();

      fetchMock.post(`/json/v1/ignores/del/${idOfThirdRule}`, '{"deleted": "true"}');
      const p = eventPromise('end-task');
      clickConfirmDeleteButton(ignoresPageSk);
      await p;
    });

    it('adds a new ignore rule', async () => {
      const dialog = findCreateEditIgnoreRuleDialog(ignoresPageSk);
      expect(dialog.hasAttribute('open')).to.be.false;

      clickCreateIgnoreRuleButton(ignoresPageSk);
      expect(dialog.hasAttribute('open')).to.be.true;

      setIgnoreRuleProperties(ignoresPageSk, 'alpha=beta&gamma=delta',
        '2020-02-01T00:00:00Z', 'see skia:9525');
      fetchMock.post('/json/v1/ignores/add/', (url, opts) => {
        expect(opts.body).to.equal(
          '{"duration":"5w","filter":"alpha=beta&gamma=delta","note":"see skia:9525"}',
        );
        return '{"created": "true"}';
      });

      const createBtn = findConfirmSaveIgnoreRuleButton(ignoresPageSk);
      expect(createBtn.innerText).to.equal('Create');

      const p = eventPromise('end-task');
      createBtn.click();
      await p;

      expect(dialog.hasAttribute('open')).to.be.false;
    });

    it('updates an existing ignore rule', async () => {
      const idOfThirdRule = '7589748925671328782';
      const edit = findUpdateForRow(2);
      edit.click();

      const dialog = findCreateEditIgnoreRuleDialog(ignoresPageSk);
      expect(dialog.hasAttribute('open')).to.be.true;

      setIgnoreRuleProperties(ignoresPageSk, 'alpha=beta&gamma=delta',
        '2020-02-01T00:00:00Z', 'see skia:9525');
      fetchMock.post(`/json/v1/ignores/save/${idOfThirdRule}`, (url, opts) => {
        expect(opts.body).to.equal(
          '{"duration":"5w","filter":"alpha=beta&gamma=delta","note":"see skia:9525"}',
        );
        return '{"created": "true"}';
      });

      const updateBtn = findConfirmSaveIgnoreRuleButton(ignoresPageSk);
      expect(updateBtn.innerText).to.equal('Update');

      const p = eventPromise('end-task');
      updateBtn.click();
      await p;
    });

    it('does not create a new ignore rule if form is empty', async () => {
      clickCreateIgnoreRuleButton(ignoresPageSk);

      // The rule at this point has no fields set, so this should show an error.
      findConfirmSaveIgnoreRuleButton(ignoresPageSk).click();
      const dialog = findCreateEditIgnoreRuleDialog(ignoresPageSk);
      expect(dialog.hasAttribute('open')).to.be.true;
    });
  });
});

function findUntriagedDigestsCheckbox(ele) {
  return $$('.controls checkbox-sk', ele);
}

function findMatchesTextForRow(n, ele) {
  const row = $('table tbody tr', ele)[n];
  const cell = $$('td.matches', row);
  // condense all whitespace and then trim to avoid the formatting of
  // the html from impacting the tests too much (e.g. extraneous \n)
  return cell.innerText;
}

function findDeleteForRow(n, ele) {
  const row = $('table tbody tr', ele)[n];
  return $$('.mutate-icons delete-icon-sk', row);
}

function findUpdateForRow(n, ele) {
  const row = $('table tbody tr', ele)[n];
  return $$('.mutate-icons mode-edit-icon-sk', row);
}

function findConfirmDeleteDialog(ele) {
  return $$('confirm-dialog-sk dialog', ele);
}

function findCreateEditIgnoreRuleDialog(ele) {
  return $$('dialog#edit-ignore-rule-dialog', ele);
}

function findConfirmSaveIgnoreRuleButton(ele) {
  return $$('#edit-ignore-rule-dialog button#ok', ele);
}

function clickUntriagedDigestsCheckbox(ele) {
  // We need to click on the input element to accurately mimic a user event. This is
  // because the checkbox-sk element listens for the click event created by the
  // internal input event.
  const input = $$('input[type="checkbox"]', findUntriagedDigestsCheckbox(ele));
  input.click();
}

function clickConfirmDeleteButton(ele) {
  const ok = $$('button.confirm', findConfirmDeleteDialog(ele));
  ok.click();
}

function clickCreateIgnoreRuleButton(ele) {
  $$('.controls button.create', ele).click();
}

function setIgnoreRuleProperties(ele, query, expires, note) {
  const editor = $$('edit-ignore-rule-sk', findCreateEditIgnoreRuleDialog(ele));
  editor.query = query;
  editor.expires = expires;
  editor.note = note;
}

async function goBack() {
  // Wait for /json/v1/ignores and /json/v1/paramset RPCs to complete.
  const events = eventSequencePromise(['end-task', 'end-task']);
  history.back();
  await events;
}

async function goForward() {
  // Wait for /json/v1/ignores and /json/v1/paramset RPCs to complete.
  const events = eventSequencePromise(['end-task', 'end-task']);
  history.forward();
  await events;
}
