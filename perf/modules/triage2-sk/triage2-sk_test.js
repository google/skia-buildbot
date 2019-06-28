import './index.js'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('triage2-sk', function() {
  describe('event', function() {
    it('fires when button is clicked', function() {
      return window.customElements.whenDefined('triage2-sk').then(() => {
        container.innerHTML = `<triage2-sk value=untriaged></triage2-sk>`;
        let value = 'unfired';
        let tr = container.firstElementChild;
        tr.addEventListener('change', (e) => { value = e.detail; });
        tr.querySelector('.positive').click();
        assert.equal('positive', tr.value, 'Element is changed.');
        assert.equal('positive', value, 'Event was sent.');
      });
    });
	});
});
