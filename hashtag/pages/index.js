import './index.scss'
import { $$, findParent} from 'common-sk/modules/dom'

// Handle the toggle hidden forms.
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

$$('#edit-toggle').addEventListener('change', (e) => {
    console.log(e);
    $$('main').classList.toggle('edit-mode', e.target.checked);
});