import '@angular/compiler';
import { TestBed } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { ColumnSelectorComponent } from './column-selector.component';
import { JobTableColumnsService, JobTableColumn } from '../job-table-columns.service';
import { assert } from 'chai';

describe('ColumnSelectorComponent', () => {
  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  function createComponent(): {
    component: ColumnSelectorComponent;
    service: JobTableColumnsService;
  } {
    TestBed.configureTestingModule({
      providers: [ColumnSelectorComponent, JobTableColumnsService],
    });
    const component = TestBed.runInInjectionContext(() => new ColumnSelectorComponent());
    const service = TestBed.inject(JobTableColumnsService);
    return { component, service };
  }

  it('should compute filteredColumns correctly', () => {
    const { component } = createComponent();
    assert.equal(component.filteredColumns.length, component.allColumns.length);

    component.columnSearchQuery = 'type';
    assert.equal(component.filteredColumns.length, 1);
    assert.equal(component.filteredColumns[0].id, JobTableColumn.JobType);

    component.columnSearchQuery = '   bOt   ';
    assert.equal(component.filteredColumns.length, 1);
    assert.equal(component.filteredColumns[0].id, JobTableColumn.Configuration);
  });

  it('should compute allSelected and someSelected correctly', () => {
    const { component, service } = createComponent();
    assert.isTrue(component.allSelected);
    assert.isFalse(component.someSelected);

    service.updateSelection(new Set([JobTableColumn.Name]));
    assert.isFalse(component.allSelected);
    assert.isTrue(component.someSelected);

    service.updateSelection(new Set());
    assert.isFalse(component.allSelected);
    assert.isFalse(component.someSelected);
  });

  it('should update service selection on toggleColumn', () => {
    const { component, service } = createComponent();

    // Deselect Benchmark
    component.toggleColumn(JobTableColumn.Benchmark, false);
    assert.isFalse(service.selectedColumnIds().has(JobTableColumn.Benchmark));

    // Re-select Benchmark
    component.toggleColumn(JobTableColumn.Benchmark, true);
    assert.isTrue(service.selectedColumnIds().has(JobTableColumn.Benchmark));
  });

  it('should update service selection on toggleAll', () => {
    const { component, service } = createComponent();

    // Deselect all
    component.toggleAll(false);
    assert.equal(service.selectedColumnIds().size, 0);

    // Select all
    component.toggleAll(true);
    assert.equal(service.selectedColumnIds().size, component.allColumns.length);
  });
});
