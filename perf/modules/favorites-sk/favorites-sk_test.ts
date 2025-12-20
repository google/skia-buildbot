import './index';
import { assert } from 'chai';
import { FavoritesSk } from './favorites-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

describe('favorites-sk', () => {
  const newInstance = setUpElementUnderTest<FavoritesSk>('favorites-sk');

  let element: FavoritesSk;

  const setupFetchMock = () => {
    fetchMock.get('/_/favorites/', {
      sections: [
        {
          name: 'My Favorites',
          links: [{ id: '1', text: 'Fav 1', description: 'Desc 1', href: '/fav1' }],
        },
      ],
    });
  };

  beforeEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  it('fetches and renders favorites on connect', async () => {
    setupFetchMock();
    element = newInstance();
    await fetchMock.flush(true);
    assert.isNotNull(element.querySelector('table'));
    assert.equal(element.querySelector('a')?.textContent, 'Fav 1');
  });

  it('renders no favorites message if config is empty', async () => {
    fetchMock.get('/_/favorites/', { sections: [] });
    element = newInstance();
    await fetchMock.flush(true);
    assert.include(element.textContent || '', 'No favorites have been configured');
  });

  it('deletes a favorite', async () => {
    setupFetchMock();
    element = newInstance();
    await fetchMock.flush(true);

    fetchMock.post('/_/favorites/delete', 200);
    // Reload favorites after delete
    fetchMock.get('/_/favorites/', { sections: [] }, { overwriteRoutes: true });

    // Mock confirm dialog
    const confirmStub = sinon.stub(window, 'confirm').returns(true);

    const deleteBtn = element.querySelector('.delete-favorite') as HTMLButtonElement;
    deleteBtn.click();

    await fetchMock.flush(true);
    assert.isTrue(confirmStub.called);
    assert.isTrue(fetchMock.called('/_/favorites/delete'));
  });

  it('does not delete if not confirmed', async () => {
    setupFetchMock();
    element = newInstance();
    await fetchMock.flush(true);

    fetchMock.post('/_/favorites/delete', 200);

    const confirmStub = sinon.stub(window, 'confirm').returns(false);

    const deleteBtn = element.querySelector('.delete-favorite') as HTMLButtonElement;
    deleteBtn.click();

    assert.isTrue(confirmStub.called);
    assert.isFalse(fetchMock.called('/_/favorites/delete'));
  });

  it('opens edit dialog', async () => {
    setupFetchMock();
    element = newInstance();
    await fetchMock.flush(true);

    const editBtn = element.querySelector('.edit-favorite') as HTMLButtonElement;
    const dialog = element.querySelector('#fav-dialog') as any;
    const openStub = sinon.stub(dialog, 'open').resolves();

    editBtn.click();
    assert.isTrue(openStub.calledWith('1', 'Fav 1', 'Desc 1', '/fav1'));
  });
});
