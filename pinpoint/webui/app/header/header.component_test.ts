import '@angular/compiler';
import { Injector, runInInjectionContext } from '@angular/core';
import { HeaderComponent } from './header.component';
import { GatewayService } from '../gateway/gateway.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('HeaderComponent', () => {
  let stubConsoleError: sinon.SinonStub;

  beforeEach(() => {
    stubConsoleError = sinon.stub(console, 'error');
  });

  afterEach(() => {
    stubConsoleError.restore();
  });

  it('should set userEmail when GetUserInfo succeeds', async () => {
    const mockGateway = {
      GetUserInfo: async () => ({ email: 'somebody@google.com' }),
    };
    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: mockGateway }],
    });

    let component!: HeaderComponent;
    runInInjectionContext(injector, () => {
      component = new HeaderComponent();
    });

    await component.ngOnInit();
    assert.equal(component.userEmail(), 'somebody@google.com');
  });

  it('should set userEmail to "Unknown user" when GetUserInfo returns no email', async () => {
    const mockGateway = {
      GetUserInfo: async () => ({ email: '' }),
    };
    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: mockGateway }],
    });

    let component!: HeaderComponent;
    runInInjectionContext(injector, () => {
      component = new HeaderComponent();
    });

    await component.ngOnInit();
    assert.equal(component.userEmail(), 'Unknown user');
  });

  it('should log an error and set userEmail to an error when GetUserInfo fails', async () => {
    const testError = new Error('Network Error');
    const mockGateway = {
      GetUserInfo: async () => {
        throw testError;
      },
    };
    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: mockGateway }],
    });

    let component!: HeaderComponent;
    runInInjectionContext(injector, () => {
      component = new HeaderComponent();
    });

    await component.ngOnInit();
    assert.equal(component.userEmail(), 'Error loading user');
    assert.isTrue(stubConsoleError.calledOnceWithExactly('Failed to load user info:', testError));
  });

  it('should redirect to the proxy logout URL on sign out click', () => {
    const mockGateway = {
      GetUserInfo: async () => ({ email: 'somebody@google.com' }),
    };

    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: mockGateway }],
    });

    let component!: HeaderComponent;
    runInInjectionContext(injector, () => {
      component = new HeaderComponent();
    });

    let redirectedUrl = '';
    component.redirect = (url: string) => {
      redirectedUrl = url;
    };

    component.onSignOutClick();
    assert.equal(redirectedUrl, `/logout/?redirect=${encodeURIComponent(window.location.origin)}`);
  });
});
