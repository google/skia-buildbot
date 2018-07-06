import './index.js'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('systemd-unit-status-sk', function() {
  describe('restart', function() {
    it('generates event when clicked', function() {
      return window.customElements.whenDefined('systemd-unit-status-sk').then(() => {
        container.innerHTML = `<systemd-unit-status-sk machine='skia-fiddle'><systemd-unit-status-sk>`;
        let ele = container.firstElementChild;
        ele.value = {
          "status": {
            "Name": "pulld.service",
          },
        };
        let detail = {};
        ele.addEventListener('unit-action', (e) => {
          detail = e.detail;
        });
        let button = ele.querySelectorAll('button')[2];
        assert.equal(button.textContent, 'Restart');
        button.click();
        assert.equal('skia-fiddle', detail.machine);
        assert.equal('pulld.service', detail.name);
        assert.equal('restart', detail.action);
      })
    });
  });
});
