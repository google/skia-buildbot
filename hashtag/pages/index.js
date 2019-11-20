import './index.scss'
import { $$, findParent} from 'common-sk/modules/dom'

// Handle the form action for toggling the 'hidden' state of any Artifact.
document.addEventListener('submit', async (e) => {
    if (e.target.target != "__toggle_hidden") {
        return;
    }
    e.preventDefault();
    const resp = await fetch(e.target.action, {
        method: 'POST',
        body: new FormData(e.target),
    });
    await resp.text();
    findParent(e.target, 'LI').classList.toggle('hidden');
});

// When someone clicks the 'Edit' checkbox we display the edit buttons next to
// each Artifact that allows the hidden state to be toggled. The displaying is
// done via CSS, here we just toggle a CSS class on the 'main' element.
$$('#edit-toggle').addEventListener('change', (e) => {
    $$('main').classList.toggle('edit-mode', e.target.checked);
});