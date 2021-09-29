import './index';

import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import fetchMock from 'fetch-mock';
import { html } from 'lit-html';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ListPageSk } from './list-page-sk';

interface PigglyWiggly {
	Piggly: string;
	Wiggly: string;
}

/**
 * An arbitrary concretization of the abstract ListPageSk class for testing
 */
class PigglyWigglyPageSk extends ListPageSk<PigglyWiggly> {
  _fetchPath = '/_/pigglywigglies';

  tableHeaders() {
    return html`
      <th>Piggly</th>
      <th>Wiggly</th>
    `;
  }

  tableRow(pw: PigglyWiggly) {
    return html`
      <tr>
        <td id=${pw.Piggly}>${pw.Piggly}</td>
        <td>${pw.Wiggly}</td>
      </tr>
    `;
  }
}

const newPigglyWigglyElement = async (): Promise<PigglyWigglyPageSk> => {
  fetchMock.reset();
  fetchMock.config.overwriteRoutes = true;
  fetchMock.get('/_/pigglywigglies', [
    {
      Piggly: 'pigglyA',
      Wiggly: 'wigglyA',
    },
  ]);

  document.body.innerHTML = '<piggly-wiggly-page-sk></piggly-wiggly-page-sk>';

  // Wait for the fetch that initially populates the list:
  await fetchMock.flush(true);

  return document.body.firstElementChild as PigglyWigglyPageSk;
};

describe('list-page-sk', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  define('piggly-wiggly-page-sk', PigglyWigglyPageSk);

  it('loads data by fetch on connectedCallback', async () => {
    // Each row has an ID set to the Piggly property of the list item:
    assert.isNotNull($$('td#pigglyA', await newPigglyWigglyElement()));
  });

  it('filters out elements that do not match', async () => {
    const e = await newPigglyWigglyElement();
    const filterElement = $$<HTMLInputElement>('#filter-input', e)!;
    filterElement.value = 'this string does not appear in any machine';
    filterElement.dispatchEvent(new InputEvent('input'));

    assert.isNull($$('td#pigglyA', e));
  });
});
