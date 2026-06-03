import '@angular/compiler';
import { TestBed } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { NewJobComponent } from './new-job.component';
import { assert } from 'chai';

describe('NewJobComponent', () => {
  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  function createComponent(): NewJobComponent {
    TestBed.configureTestingModule({
      providers: [NewJobComponent],
    });
    return TestBed.runInInjectionContext(() => new NewJobComponent());
  }

  function createValidComponent(): NewJobComponent {
    const component = createComponent();
    component.jobForm.get('bot')?.setValue('linux-perf');
    component.jobForm.get('benchmark')?.setValue('speedometer');
    component.jobForm.get('story')?.setValue('Speedometer3');
    component.jobForm.get('baseline.commit')?.setValue('abcd1234');
    return component;
  }

  it('should initialize form with default values', () => {
    const component = createComponent();
    assert.isNotNull(component.jobForm);
    assert.equal(component.jobForm.get('attempts')?.value, 30);
    assert.equal(component.jobForm.get('baseline.commit')?.value, '');
    assert.equal(component.jobForm.get('experiment.commit')?.value, '');
    assert.isFalse(component.jobForm.valid);
  });

  it('should create a valid form', () => {
    const form = createValidComponent().jobForm;
    assert.isTrue(form.valid);
  });

  it('should validate bot', () => {
    const form = createValidComponent().jobForm;
    form.get('bot')?.setValue('');
    assert.isFalse(form.valid);
  });

  it('should validate attempts count', () => {
    const form = createValidComponent().jobForm;
    form.get('attempts')?.setValue(0);
    assert.isFalse(form.valid);

    form.get('attempts')?.setValue(-5);
    assert.isFalse(form.valid);

    form.get('attempts')?.setValue(1);
    assert.isTrue(form.valid);
  });

  it('should validate bug ID', () => {
    const form = createValidComponent().jobForm;
    form.get('bugId')?.setValue('');
    assert.isTrue(form.valid);

    form.get('bugId')?.setValue(0);
    assert.isFalse(form.valid);

    form.get('bugId')?.setValue(-123);
    assert.isFalse(form.valid);
  });
});
