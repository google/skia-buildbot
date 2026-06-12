import '@angular/compiler';
import { Injector, runInInjectionContext } from '@angular/core';
import { ThemeService } from './theme.service';
import { SettingsService, Theme } from '../settings/settings.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('ThemeService', () => {
  let initialClasses: string[];
  let getThemeStub: sinon.SinonStub;
  let setThemeSpy: sinon.SinonSpy;

  beforeEach(() => {
    initialClasses = Array.from(document.documentElement.classList);
    document.documentElement.className = '';
    getThemeStub = sinon.stub();
    setThemeSpy = sinon.spy();
  });

  afterEach(() => {
    document.documentElement.className = initialClasses.join(' ');
  });

  function createService(): ThemeService {
    const mockSettings: Partial<SettingsService> = {
      getTheme: getThemeStub,
      setTheme: setThemeSpy,
    };
    const injector = Injector.create({
      providers: [{ provide: SettingsService, useValue: mockSettings }, ThemeService],
    });
    let service!: ThemeService;
    runInInjectionContext(injector, () => {
      service = injector.get(ThemeService);
    });
    return service;
  }

  it('should initialize to light theme by default when no saved theme', () => {
    getThemeStub.returns(Theme.Light);

    const service = createService();

    assert.isFalse(service.isDarkMode());
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));
    assert.isTrue(getThemeStub.calledOnce);
  });

  it('should initialize to dark theme when saved theme is dark', () => {
    getThemeStub.returns(Theme.Dark);

    const service = createService();

    assert.isTrue(service.isDarkMode());
    assert.isTrue(document.documentElement.classList.contains('dark-theme'));
  });

  it('should toggle theme and save using SettingsService', () => {
    getThemeStub.returns(Theme.Light);

    const service = createService();
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));

    service.toggleTheme();
    assert.isTrue(service.isDarkMode());
    assert.isTrue(setThemeSpy.calledWith(Theme.Dark));
    assert.isTrue(document.documentElement.classList.contains('dark-theme'));

    service.toggleTheme();
    assert.isFalse(service.isDarkMode());
    assert.isTrue(setThemeSpy.calledWith(Theme.Light));
    assert.isFalse(document.documentElement.classList.contains('dark-theme'));
  });
});
