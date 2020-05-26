import './index';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('theme-chooser-sk', () => {
  it('generates event and toggles from darkmode when clicked', async () => {
    await window.customElements.whenDefined('theme-chooser-sk');
    container.innerHTML = '<theme-chooser-sk></theme-chooser-sk>';
    const chooser = container.firstElementChild;
    chooser.darkmode = true;
    chooser.addEventListener('theme-chooser-toggle', (e) => {
      assert.isFalse(e.detail.darkmode);
    });
    chooser.click();
  });

  it('generates event and toggles to darkmode when clicked', async () => {
    await window.customElements.whenDefined('theme-chooser-sk');
    container.innerHTML = '<theme-chooser-sk></theme-chooser-sk>';
    const chooser = container.firstElementChild;
    chooser.darkmode = false;
    chooser.addEventListener('theme-chooser-toggle', (e) => {
      assert.isTrue(e.detail.darkmode);
    });
    chooser.click();
  });
});
