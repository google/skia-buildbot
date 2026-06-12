import '@angular/compiler';
import { Injector, runInInjectionContext, signal, WritableSignal } from '@angular/core';
import { HeaderComponent, DOCUMENTATION_URL, BUG_REPORT_URL } from './header.component';
import { GatewayService } from '../gateway/gateway.service';
import { ThemeService } from '../theme/theme.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('HeaderComponent', () => {
  let stubConsoleError: sinon.SinonStub;
  let openStub: sinon.SinonStub;
  let mockIsDarkMode: WritableSignal<boolean>;
  let toggleThemeSpy: sinon.SinonSpy;

  beforeEach(() => {
    stubConsoleError = sinon.stub(console, 'error');
    openStub = sinon.stub(window, 'open');
    mockIsDarkMode = signal<boolean>(false);
    toggleThemeSpy = sinon.spy();
  });

  afterEach(() => {
    stubConsoleError.restore();
    openStub.restore();
  });

  function createHeaderComponent(mockGateway?: Partial<GatewayService>): HeaderComponent {
    const gateway = mockGateway || {
      GetUserInfo: async () => ({ email: 'somebody@google.com' }),
    };
    const mockThemeService: Partial<ThemeService> = {
      isDarkMode: mockIsDarkMode,
      toggleTheme: toggleThemeSpy,
    };
    const injector = Injector.create({
      providers: [
        { provide: GatewayService, useValue: gateway },
        { provide: ThemeService, useValue: mockThemeService },
      ],
    });
    let component!: HeaderComponent;
    runInInjectionContext(injector, () => {
      component = new HeaderComponent();
    });
    return component;
  }

  it('should set userEmail when GetUserInfo succeeds', async () => {
    const component = createHeaderComponent();
    await component.ngOnInit();
    assert.equal(component.userEmail(), 'somebody@google.com');
  });

  it('should set userEmail to "Unknown user" when GetUserInfo returns no email', async () => {
    const component = createHeaderComponent({
      GetUserInfo: async () => ({ email: '' }),
    });
    await component.ngOnInit();
    assert.equal(component.userEmail(), 'Unknown user');
  });

  it('should log an error and set userEmail to an error when GetUserInfo fails', async () => {
    const testError = new Error('Network Error');
    const component = createHeaderComponent({
      GetUserInfo: async () => {
        throw testError;
      },
    });
    await component.ngOnInit();
    assert.equal(component.userEmail(), 'Error loading user');
    assert.isTrue(stubConsoleError.calledOnceWithExactly('Failed to load user info:', testError));
  });

  it('should redirect to the proxy logout URL on sign out click', () => {
    const component = createHeaderComponent();
    let redirectedUrl = '';
    component.redirect = (url: string) => {
      redirectedUrl = url;
    };
    component.onSignOutClick();
    assert.equal(redirectedUrl, `/logout/?redirect=${encodeURIComponent(window.location.origin)}`);
  });

  it('should open documentation on help click', () => {
    const component = createHeaderComponent();
    component.onHelpClick();
    assert.isTrue(openStub.calledOnceWithExactly(DOCUMENTATION_URL, '_blank'));
  });

  it('should open buganizer on bug click', () => {
    const component = createHeaderComponent();
    component.onBugClick();
    assert.isTrue(openStub.calledOnceWithExactly(BUG_REPORT_URL, '_blank'));
  });

  describe('theme toggle', () => {
    it('should toggle theme when toggleTheme is called', () => {
      const component = createHeaderComponent();
      component.toggleTheme();
      assert.isTrue(toggleThemeSpy.calledOnce);
    });

    it('should expose isDarkMode signal from ThemeService', () => {
      const component = createHeaderComponent();
      assert.isFalse(component.isDarkMode());
      mockIsDarkMode.set(true);
      assert.isTrue(component.isDarkMode());
    });
  });
});
