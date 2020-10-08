import './index';

window.addEventListener('load', () => {
  document.getElementById('email-chooser').open(
    ['alice@example.com', 'bob@example.com', 'claire@example.com'],
    'bob@example.com',
  );
});
