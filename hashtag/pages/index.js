import './index.scss'

import 'elements-sk/error-toast-sk'
import { $$, findParent } from 'common-sk/modules/dom'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

// Handle the form action for toggling the 'hidden' state of any Artifact.
document.addEventListener('submit', (e) => {
    if (e.target.target != "__toggle_hidden") {
        return;
    }
    e.preventDefault();
    fetch(e.target.action, {
        method: 'POST',
        body: new FormData(e.target),
    }).then(jsonOrThrow).then((json) => {
        findParent(e.target, 'LI').classList.toggle('hidden');
    }).catch(errorMessage);
});

// When someone clicks the 'Edit' checkbox we display the edit buttons next to
// each Artifact that allows the hidden state to be toggled. The displaying is
// done via CSS, here we just toggle a CSS class on the 'main' element.
$$('#edit-toggle').addEventListener('change', (e) => {
    $$('main').classList.toggle('edit-mode', e.target.checked);
});