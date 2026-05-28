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

  afterEach(() => {
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

    // Deselect Name and Benchmark
    service.updateSelection(
      new Set([
        JobTableColumn.Configuration,
        JobTableColumn.Story,
        JobTableColumn.JobType,
        JobTableColumn.Bug,
        JobTableColumn.User,
        JobTableColumn.Created,
        JobTableColumn.JobStatus,
      ])
    );

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

    // Re-select Benchmark - must appear in original index position, not at the end!
    service.updateSelection(
      new Set([
        JobTableColumn.Benchmark,
        JobTableColumn.Configuration,
        JobTableColumn.Story,
        JobTableColumn.JobType,
        JobTableColumn.Bug,
        JobTableColumn.User,
        JobTableColumn.Created,
        JobTableColumn.JobStatus,
      ])
    );

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
});
