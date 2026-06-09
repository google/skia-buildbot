import '@angular/compiler';
import { TestBed } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { JobTableColumnsService, JobTableColumn } from './job-table-columns.service';
import { assert } from 'chai';

describe('JobTableColumnsService', () => {
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

  function getService(): JobTableColumnsService {
    TestBed.configureTestingModule({
      providers: [JobTableColumnsService],
    });
    return TestBed.inject(JobTableColumnsService);
  }

  it('should initialize with all columns selected', () => {
    const service = getService();
    assert.equal(service.selectedColumnIds().size, service.allColumns.length);
    assert.equal(service.displayedColumns().length, service.allColumns.length);
    assert.deepEqual(service.displayedColumns(), [
      JobTableColumn.Name,
      JobTableColumn.Benchmark,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
  });

  it('should update column selection and preserve original order in displayedColumns', () => {
    const service = getService();

    const updated = new Set(service.selectedColumnIds());
    updated.delete(JobTableColumn.Name);
    updated.delete(JobTableColumn.Benchmark);
    service.updateSelection(updated);

    assert.equal(service.selectedColumnIds().size, 7);
    assert.deepEqual(service.displayedColumns(), [
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
    assert.deepEqual(JSON.parse(localStorage.getItem('selected_columns') || '[]'), [
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);

    // Re-select Benchmark - must appear in original index position.
    const reselect = new Set(service.selectedColumnIds());
    reselect.add(JobTableColumn.Benchmark);
    service.updateSelection(reselect);

    assert.equal(service.selectedColumnIds().size, 8);
    assert.deepEqual(service.displayedColumns(), [
      JobTableColumn.Benchmark,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
  });

  it('should support reordering columns', () => {
    const service = getService();

    service.reorderColumns(1, 3);

    const expectedOrder = [
      JobTableColumn.Name,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.Benchmark,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ];
    assert.deepEqual(service.displayedColumns(), expectedOrder);
    assert.deepEqual(JSON.parse(localStorage.getItem('ordered_columns') || '[]'), expectedOrder);
  });

  it('should maintain custom column order when columns are deselected', () => {
    const service = getService();

    service.reorderColumns(1, 3);

    const updated = new Set(service.selectedColumnIds());
    updated.delete(JobTableColumn.Benchmark);
    service.updateSelection(updated);

    assert.deepEqual(service.displayedColumns(), [
      JobTableColumn.Name,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
  });

  it('should restore column to its custom reordered position when re-selected', () => {
    const service = getService();

    service.reorderColumns(1, 3);

    const deselect = new Set(service.selectedColumnIds());
    deselect.delete(JobTableColumn.Benchmark);
    service.updateSelection(deselect);

    // Re-select Benchmark, it should reappear in its reordered position.
    const select = new Set(service.selectedColumnIds());
    select.add(JobTableColumn.Benchmark);
    service.updateSelection(select);

    assert.deepEqual(service.displayedColumns(), [
      JobTableColumn.Name,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.Benchmark,
      JobTableColumn.JobType,
      JobTableColumn.Bug,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
  });

  it('should restore columns selection and order when resetToDefault is called', () => {
    const service = getService();
    const updated = new Set(service.selectedColumnIds());
    updated.delete(JobTableColumn.Name);
    service.updateSelection(updated);
    service.reorderColumns(1, 3);

    service.resetToDefault();

    assert.equal(service.selectedColumnIds().size, service.allColumns.length);
    assert.deepEqual(
      service.displayedColumns(),
      service.defaultColumnOrder.map((c) => c.id)
    );
    assert.deepEqual(
      JSON.parse(localStorage.getItem('selected_columns') || '[]'),
      service.defaultColumnOrder.map((c) => c.id)
    );
    assert.deepEqual(
      JSON.parse(localStorage.getItem('ordered_columns') || '[]'),
      service.defaultColumnOrder.map((c) => c.id)
    );
  });
});
