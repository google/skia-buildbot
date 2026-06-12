import '@angular/compiler';
import { TestBed } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { SettingsService, SettingKey, Theme } from './settings.service';
import { assert } from 'chai';

describe('SettingsService', () => {
  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
    TestBed.resetTestingModule();
  });

  function getService(): SettingsService {
    TestBed.configureTestingModule({
      providers: [SettingsService],
    });
    return TestBed.inject(SettingsService);
  }

  it('should return default value when showOnlyUserJobs is not set', () => {
    const service = getService();
    assert.isTrue(service.getShowOnlyUserJobs(true));
    assert.isFalse(service.getShowOnlyUserJobs(false));
  });

  it('should set and get showOnlyUserJobs correctly', () => {
    const service = getService();
    service.setShowOnlyUserJobs(true);
    assert.isTrue(service.getShowOnlyUserJobs(false));
    assert.equal(localStorage.getItem(SettingKey.ShowOnlyUserJobs), 'true');

    service.setShowOnlyUserJobs(false);
    assert.isFalse(service.getShowOnlyUserJobs(true));
    assert.equal(localStorage.getItem(SettingKey.ShowOnlyUserJobs), 'false');
  });

  it('should return default value when orderedColumns is not set', () => {
    const service = getService();
    const defaults = ['col1', 'col2'];
    assert.deepEqual(service.getOrderedColumns(defaults), defaults);
  });

  it('should set and get orderedColumns correctly', () => {
    const service = getService();
    const cols = ['col3', 'col4'];
    service.setOrderedColumns(cols);
    assert.deepEqual(service.getOrderedColumns([]), cols);
    assert.equal(localStorage.getItem(SettingKey.OrderedColumns), JSON.stringify(cols));
  });

  it('should return default value when selectedColumns is not set', () => {
    const service = getService();
    const defaults = ['col1', 'col2'];
    assert.deepEqual(service.getSelectedColumns(defaults), defaults);
  });

  it('should set and get selectedColumns correctly', () => {
    const service = getService();
    const cols = ['col3', 'col4'];
    service.setSelectedColumns(cols);
    assert.deepEqual(service.getSelectedColumns([]), cols);
    assert.equal(localStorage.getItem(SettingKey.SelectedColumns), JSON.stringify(cols));
  });

  it('should handle corrupted json gracefully and return default value', () => {
    const service = getService();
    localStorage.setItem(SettingKey.OrderedColumns, '{bad json}');
    const defaults = ['col1'];
    assert.deepEqual(service.getOrderedColumns(defaults), defaults);
  });

  it('should return default value when theme is not set', () => {
    const service = getService();
    assert.equal(service.getTheme(), Theme.Light);
    assert.equal(service.getTheme(Theme.Dark), Theme.Dark);
  });

  it('should set and get theme correctly', () => {
    const service = getService();
    service.setTheme(Theme.Dark);
    assert.equal(service.getTheme(), Theme.Dark);
    assert.equal(localStorage.getItem(SettingKey.Theme), JSON.stringify(Theme.Dark));

    service.setTheme(Theme.Light);
    assert.equal(service.getTheme(Theme.Dark), Theme.Light);
    assert.equal(localStorage.getItem(SettingKey.Theme), JSON.stringify(Theme.Light));
  });
});
