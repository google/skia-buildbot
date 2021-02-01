import './index';
import { EmailChooserSk } from './email-chooser-sk';

window.addEventListener('load', () => {
  (document.getElementById('email-chooser') as EmailChooserSk).open(
    ['alice@example.com', 'bob@example.com', 'claire@example.com'],
    'bob@example.com',
  );
});
