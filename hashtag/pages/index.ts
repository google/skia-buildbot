import 'elements-sk/error-toast-sk';
import { $$, findParent } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import '../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/styles/buttons';

// Move to after theme-chooser-sk since that uses infra-sk styles right now.
import './index.scss';

// Handle the form action for toggling the 'hidden' state of any Artifact.
document.addEventListener('submit', (e) => {
  const form = e.target as HTMLFormElement;
  if (form.getAttribute('target') !== '__toggle_hidden') {
    return;
  }
  e.preventDefault();

  fetch(form.action, {
    method: 'POST',
    body: new FormData(form),
  }).then(jsonOrThrow).then(() => {
    findParent(form, 'LI')!.classList.toggle('hidden');
  }).catch(errorMessage);
});

// When someone clicks the 'Edit' checkbox we display the edit buttons next to
// each Artifact that allows the hidden state to be toggled. The displaying is
// done via CSS, here we just toggle a CSS class on the 'main' element.
$$<HTMLInputElement>('#edit-toggle')!.addEventListener('change', (e) => {
  $$<HTMLDivElement>('main')!.classList.toggle('edit-mode', (e.target as HTMLInputElement).checked);
});
