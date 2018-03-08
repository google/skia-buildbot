import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('systemd-unit-status-sk', function() {
  describe('stop', function() {
    it('generated event when clicked', function() {
      return window.customElements.whenDefined('systemd-unit-status-sk').then(() => {
        container.innerHTML = `<systemd-unit-status-sk machine='skia-fiddle'><systemd-unit-status-sk>`;
        let s = container.firstElementChild;
        s.value = {
          "status": {
            "Name": "pulld.service",
          },
        };
        let detail = {};
        s.addEventListener('unit-action', (e) => {
          detail = e.detail;
        });
        let b = s.querySelectorAll('button')[2];
        assert.equal(b.textContent, 'Restart');
        b.click();
        let expected = {
          machine: 'skia-fiddle',
          name: 'pulld.service',
          action: 'restart',
        }
        assert.equal(expected.machine, detail.machine);
        assert.equal(expected.name, detail.name);
        assert.equal(expected.action, detail.action);
      })
    });
  });

});
