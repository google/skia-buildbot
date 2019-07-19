import './index.js'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = '';
});

function _regexSetup() {
  return window.customElements.whenDefined('query-values-sk').then(() => {
    container.innerHTML = `<query-values-sk></query-values-sk>`;
    let q = container.querySelector('query-values-sk');
    q.options = ['x86', 'arm'];
    q.selected = ['~ar'];
    return q;
  });
}

function _normalSetup() {
  return window.customElements.whenDefined('query-values-sk').then(() => {
    container.innerHTML = `<query-values-sk></query-values-sk>`;
    let q = container.querySelector('query-values-sk');
    q.options = ['x86', 'arm'];
    q.selected = ['arm'];
    return q;
  });
}

function _invertSetup() {
  return window.customElements.whenDefined('query-values-sk').then(() => {
    container.innerHTML = `<query-values-sk></query-values-sk>`;
    let q = container.querySelector('query-values-sk');
    q.options = ['x86', 'arm'];
    q.selected = ['!arm'];
    return q;
  });
}

describe('query-values-sk', function() {
  describe('event', function() {

    it('toggles a regex correctly on invert click', function() {
      return _regexSetup().then((q) => {
        assert.isTrue(q.querySelector('#regex').checked);
        let value = 'Unfired';
        q.addEventListener('query-values-changed', (e) => { value = e.detail; });
        q.querySelector('#invert').click();
        assert.deepEqual([], value, 'Event was sent.');
        // Regex and Invert are mutually exclusive.
        assert.isFalse(q.querySelector('#regex').checked, 'Regex checkbox is unchecked.');
        assert.isTrue(q.querySelector('#invert').checked);
      });
    });

    it('toggles a regex correctly for regex click', function() {
      return _regexSetup().then((q) => {
        assert.isTrue(q.querySelector('#regex').checked);
        let value = 'Unfired';
        q.addEventListener('query-values-changed', (e) => { value = e.detail; });
        q.querySelector('#regex').click();
        assert.deepEqual([], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked, 'Regex is unchecked');
        assert.isFalse(q.querySelector('#invert').checked, 'Invert stays unchecked');

        // No go back to regex.
        q.querySelector('#regex').click();
        assert.deepEqual(['~ar'], value, 'Event was sent.');
        assert.isTrue(q.querySelector('#regex').checked, 'Regex is checked');
        assert.isFalse(q.querySelector('#invert').checked, 'Invert stays unchecked');
      });
    });

    it('is toggles invert correctly for invert click', function() {
      return _normalSetup().then((q) => {
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isFalse(q.querySelector('#invert').checked);
        let value = 'Unfired';
        q.addEventListener('query-values-changed', (e) => { value = e.detail; });
        q.querySelector('#invert').click();

        assert.deepEqual(['!arm'], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isTrue(q.querySelector('#invert').checked);

        q.querySelector('#invert').click();
        assert.deepEqual(['arm'], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isFalse(q.querySelector('#invert').checked);
      });
    });

    it('is toggles correctly for invert click when starting inverted', function() {
      return _invertSetup().then((q) => {
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isTrue(q.querySelector('#invert').checked);
        let value = 'Unfired';
        q.addEventListener('query-values-changed', (e) => { value = e.detail; });
        q.querySelector('#invert').click();

        assert.deepEqual(['arm'], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isFalse(q.querySelector('#invert').checked);

        q.querySelector('#invert').click();
        assert.deepEqual(['!arm'], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isTrue(q.querySelector('#invert').checked);
      });
    });

    it('is sends right event when value is clicked', function() {
      return _invertSetup().then((q) => {
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isTrue(q.querySelector('#invert').checked);
        let value = 'Unfired';
        q.addEventListener('query-values-changed', (e) => { value = e.detail; });
        q.querySelector('#values').children[0].click();

        assert.deepEqual(['!x86', '!arm'], value, 'Event was sent.');

        q.querySelector('#invert').click();
        assert.deepEqual(['x86', 'arm'], value, 'Event was sent.');
        assert.isFalse(q.querySelector('#regex').checked);
        assert.isFalse(q.querySelector('#invert').checked);
      });
    });

	});
});
