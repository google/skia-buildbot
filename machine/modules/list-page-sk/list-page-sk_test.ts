import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import fetchMock from 'fetch-mock';
import { html } from 'lit-html';
import { ListPageSk } from './list-page-sk';

interface PigglyWiggly {
	Piggly: string;
	Wiggly: string;
}

/**
 * An arbitrary concretization of the abstract ListPageSk class for testing
 */
class PigglyWigglyPageSk extends ListPageSk<PigglyWiggly> {
  fetchPath = '/_/pigglywigglies';

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

  document.body.innerHTML = '<piggly-wiggly-table-sk></piggly-wiggly-table-sk>';

  // Wait for the fetch that initially populates the list:
  await fetchMock.flush(true);

  return document.body.firstElementChild as PigglyWigglyPageSk;
};

describe('list-page-sk', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  define('piggly-wiggly-table-sk', PigglyWigglyPageSk);

  it('loads data by fetch on update()', async () => {
    const element = await newPigglyWigglyElement();
    await element.update();
    // Each row has an ID set to the Piggly property of the list item:
    assert.isNotNull($$('td#pigglyA', element));
  });

  it('filters out elements that do not match', async () => {
    const e = await newPigglyWigglyElement();
    e.filterChanged('this string does not appear in any machine');
    assert.isNull($$('td#pigglyA', e));
  });
});
