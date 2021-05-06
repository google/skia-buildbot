import './index';
import { EmailChooserSk } from './email-chooser-sk';

window.addEventListener('load', () => {
  (document.getElementById('email-chooser-no-owner') as EmailChooserSk).open(
    ['alice@example.com', 'bob@example.com', 'claire@example.com'],
    'bob@example.com',
  );
  (document.getElementById('email-chooser-with-owner') as EmailChooserSk).open(
    ['alice@example.com', 'bob@example.com', 'claire@example.com'],
    'bob@example.com',
  );
});
