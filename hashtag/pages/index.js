import './index.scss'

// Super-simplistic polyfill for target=_inline proposal, that only does
// POST for forms of 'application/x-www-form-urlencoded'.
document.addEventListener('submit', async (e) => {
    // Only process forms when target=_inline.
    if (e.target.target != "_inline") {
        return;
    }

    // Do the form processing ourselves.
    e.preventDefault();

    // Do a POST of the form values.
    const resp = await fetch(e.target.action, {
        method: 'POST',
        body: new FormData(e.target),
    });
    // Replace the innerHTML of the form with the HTML returned from the
    // server.
    e.target.innerHTML = await resp.text();
});