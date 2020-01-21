import './index.js';

import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, expectQueryStringToEqual } from '../test_util';
import { fakeNow, ignoreRules_10 } from './test_data';
import { fetchMock }  from 'fetch-mock';

describe('ignores-page-sk', () => {
  const regularNow = Date.now;
  let ignoresSk;

  beforeEach(async function () {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');
    // These are the default offset/page_size params
    fetchMock.get('/json/ignores?counts=1', JSON.stringify(ignoreRules_10));
    // set the time to our mocked Now
    Date.now = () => fakeNow;

    const event = eventPromise('end-task');
    ignoresSk = document.createElement('ignores-page-sk');
    document.body.appendChild(ignoresSk);
    await event;
  });

  afterEach(function () {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.

    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
    // reset the time
    Date.now = regularNow;
    // Remove the stale instance under test.
    if (ignoresSk) {
      document.body.removeChild(ignoresSk);
      ignoresSk = null;
    }
  });

  //===============TESTS START====================================

  describe('html layout', () => {
    it('should make a table with 10 rows in the body', () => {
      const rows = $('table tbody tr', ignoresSk);
      expect(rows).to.have.length(10);
    });

    it('creates links to test the filter', () => {
      const rows = $('table tbody tr', ignoresSk);
      const queryLink = $$('.query a', rows[9]);
      expect(queryLink.href).to.contain('include=true&query=config%3Dglmsaa4%26cpu_or_gpu_value%3DTegraX1%26name%3Drg1024_green_grapes.svg');
      expect(queryLink.textContent).to.equal(`config=glmsaa4\ncpu_or_gpu_value=TegraX1\nname=rg1024_green_grapes.svg`);
    });

    it('has some expired and some not expired rules', () => {
      const rows = $('table tbody tr', ignoresSk);
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
      let checkbox = findUntriagedDigestsCheckbox(ignoresSk);
      expect(checkbox.checked).to.be.true;
      expect(findMatchesTextForRow(2, ignoresSk)).to.contain('0 / 4');
      expectQueryStringToEqual('');

      clickUntriagedDigestsCheckbox(ignoresSk);

      checkbox = findUntriagedDigestsCheckbox(ignoresSk);
      expect(checkbox.checked).to.be.false;
      expect(findMatchesTextForRow(2, ignoresSk)).to.contain('6 / 10');
      expectQueryStringToEqual('?count_all=true');

      clickUntriagedDigestsCheckbox(ignoresSk);

      checkbox = findUntriagedDigestsCheckbox(ignoresSk);
      expect(checkbox.checked).to.be.true;
      expect(findMatchesTextForRow(2, ignoresSk)).to.contain('0 / 4');
      expectQueryStringToEqual('');
    });

    it('responds to back and forward browser buttons', async () => {
      // Create some mock history so we can use the back button.
      setQueryString('?count_all=true');
      setQueryString('');

      // We should go back to the count_all=true setting
      await goBack();
      let checkbox = findUntriagedDigestsCheckbox(ignoresSk);
      expect(checkbox.checked).to.be.false;

      // And now return to the default view.
      await goForward();
      checkbox = findUntriagedDigestsCheckbox(ignoresSk);
      expect(checkbox.checked).to.be.true;
    });

    it('prompts "are you sure" before deleting an ignore rule', () => {
      let dialog = findDialog(ignoresSk);
      expect(dialog.hasAttribute('open')).to.be.false;

      const del = findDeleteForRow(2);
      del.click();

      expect(dialog.hasAttribute('open')).to.be.true;
      const msg = $$('.message', dialog);
      expect(msg.innerText).to.contain('Are you sure you want to delete');
    });

    it('fires a post request to delete an ignore rule', async () => {
      const idOfThirdRule = '7589748925671328782';
      const del = findDeleteForRow(2);
      del.click();

      fetchMock.post(`/json/ignores/del/${idOfThirdRule}`, '{"deleted": "true"}');
      const p = eventPromise('end-task');
      clickOkDialogButton(ignoresSk);
      await p;
    });
  });
});

function setQueryString(q) {
  history.pushState(
    null, '', window.location.origin + window.location.pathname + q);
}

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

function findDialog(ele) {
  return $$('confirm-dialog-sk dialog', ele);
}

function clickUntriagedDigestsCheckbox(ele) {
  // We need to click on the input element to accurately mimic a user event. This is
  // because the checkbox-sk element listens for the click event created by the
  // internal input event.
  const input = $$('input[type="checkbox"]', findUntriagedDigestsCheckbox(ele));
  input.click();
}

function clickOkDialogButton(ele) {
  const ok = $$('button.confirm', findDialog(ele));
  ok.click();
}

function goBack() {
  const event = eventPromise('end-task');
  history.back();
  return event;
}

function goForward() {
  const event = eventPromise('end-task');
  history.forward();
  return event;
}
