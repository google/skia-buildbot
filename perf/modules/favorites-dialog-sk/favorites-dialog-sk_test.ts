import './index';
import { expect } from 'chai';
import { FavoritesDialogSk } from './favorites-dialog-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('favorites-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<FavoritesDialogSk>(
    'favorites-dialog-sk'
  );

  let element: FavoritesDialogSk;
  beforeEach(() => {
    element = newInstance((el: FavoritesDialogSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  it('renders for new', async () => {
    expect(element).to.not.be.null;
    element.open('12345', '', '', 'url1.com').then(() => {
      const n = document.getElementById('name');
      expect(n?.nodeValue).to.be.equal('');
      expect(n?.nodeValue).to.be.equal('');
      expect(n?.nodeValue).to.be.equal('url1.com');
    });
  });

  it('renders for update', async () => {
    expect(element).to.not.be.null;
    element.open('', 'Fav', 'Fav Desc', 'url.com').then(() => {
      const n = document.getElementById('name');
      expect(n?.nodeValue).to.be.equal('Fav');
      expect(n?.nodeValue).to.be.equal('Fav Desc');
      expect(n?.nodeValue).to.be.equal('url.com');
    });
  });
});
