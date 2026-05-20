import '@angular/compiler';
import { ThemeService } from './theme.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('ThemeService', () => {
  let localStorageGetStub: sinon.SinonStub;
  let localStorageSetStub: sinon.SinonStub;
  let initialClasses: string[];

  beforeEach(() => {
    localStorageGetStub = sinon.stub(localStorage, 'getItem');
    localStorageSetStub = sinon.stub(localStorage, 'setItem');
    initialClasses = Array.from(document.documentElement.classList);
    document.documentElement.className = '';
  });

  afterEach(() => {
    localStorageGetStub.restore();
    localStorageSetStub.restore();
    document.documentElement.className = initialClasses.join(' ');
  });

  it('should initialize to light theme by default when no saved theme', () => {
    localStorageGetStub.returns(null);

    const service = new ThemeService();

    assert.isFalse(service.isDarkMode());
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));
  });

  it('should initialize to dark theme when saved theme is dark', () => {
    localStorageGetStub.withArgs('theme').returns('dark');

    const service = new ThemeService();

    assert.isTrue(service.isDarkMode());
    assert.isTrue(document.documentElement.classList.contains('dark-theme'));
  });

  it('should toggle theme and save to localStorage', () => {
    localStorageGetStub.returns(null);

    const service = new ThemeService();
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));

    service.toggleTheme();
    assert.isTrue(service.isDarkMode());
    assert.isTrue(localStorageSetStub.calledWith('theme', 'dark'));
    assert.isTrue(document.documentElement.classList.contains('dark-theme'));

    service.toggleTheme();
    assert.isFalse(service.isDarkMode());
    assert.isTrue(localStorageSetStub.calledWith('theme', 'light'));
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));
  });
});
