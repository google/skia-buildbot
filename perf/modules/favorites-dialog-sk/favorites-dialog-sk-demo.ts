import { $$ } from '../../../infra-sk/modules/dom';
import { FavoritesDialogSk } from './favorites-dialog-sk';
import './index';

const elem: FavoritesDialogSk | null = document.querySelector(
  'favorites-dialog-sk'
);
$$('#newFav')!.addEventListener('click', () => {
  elem!.open('', '', '', 'a.com');
});

$$('#editFav')!.addEventListener('click', () => {
  elem!.open('1234', 'Fav', 'Fav Desc', 'b.com');
});
