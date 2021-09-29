import './index.js';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('push-selection-sk', () => {
  describe('event', () => {
    it('generated when selection is clicked', () => window.customElements.whenDefined('push-selection-sk').then(() => {
      container.innerHTML = '<push-selection-sk></push-selection-sk>';

      const dialog = container.firstElementChild;
      dialog.choices = [
        {
          Built: '2018-02-02T18:04:45Z',
          Dirty: false,
          Hash: '6487269f0f7efd26073feed08810ce7cda49e330',
          Name: 'skiaperfd/skiaperfd:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-02T18:04:45Z:6487269f0f7efd26073feed08810ce7cda49e330.deb',
          Note: '[perf] Fix test logic',
          Services: ['skiaperfd.service'],
          UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
        }, {
          Built: '2018-02-02T17:59:24Z',
          Dirty: true,
          Hash: '11b68a4cd135029e7f10ed2765d678b09c8ccbca',
          Name: 'skiaperfd/skiaperfd:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-02T17:59:24Z:11b68a4cd135029e7f10ed2765d678b09c8ccbca.deb',
          Note: '[perf] Fix test logic',
          Services: ['skiaperfd.service'],
          UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
        },
      ];
      dialog.chosen = 1;
      dialog.show();

      let detail = {};
      dialog.addEventListener('package-change', (e) => {
        detail = e.detail;
      });

      const target = dialog.querySelector('div.pushSelection');
      target.click();
      assert.equal('skiaperfd/skiaperfd:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-02T18:04:45Z:6487269f0f7efd26073feed08810ce7cda49e330.deb', detail.name);
    }));
  });
});
