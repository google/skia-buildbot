import { Injectable, signal, computed } from '@angular/core';

export enum JobTableColumn {
  Name = 'name',
  Benchmark = 'benchmark',
  Configuration = 'configuration',
  Story = 'story',
  JobType = 'jobType',
  Bug = 'bug',
  User = 'user',
  Created = 'created',
  JobStatus = 'jobStatus',
}

export interface ColumnInfo {
  id: JobTableColumn;
  label: string;
}

@Injectable({
  providedIn: 'root',
})
export class JobTableColumnsService {
  readonly allColumns: ColumnInfo[] = [
    { id: JobTableColumn.Name, label: 'Job Name' },
    { id: JobTableColumn.Benchmark, label: 'Benchmark' },
    { id: JobTableColumn.Configuration, label: 'Bot' },
    { id: JobTableColumn.Story, label: 'Story' },
    { id: JobTableColumn.JobType, label: 'Type' },
    { id: JobTableColumn.Bug, label: 'Bug' },
    { id: JobTableColumn.User, label: 'User' },
    { id: JobTableColumn.Created, label: 'Created' },
    { id: JobTableColumn.JobStatus, label: 'Status' },
  ];

  private _selectedColumnIds = signal<Set<string>>(new Set(this.allColumns.map((c) => c.id)));

  readonly selectedColumnIds = this._selectedColumnIds.asReadonly();

  readonly displayedColumns = computed(() =>
    this.allColumns.filter((c) => this._selectedColumnIds().has(c.id)).map((c) => c.id as string)
  );

  updateSelection(updated: Set<string>) {
    this._selectedColumnIds.set(updated);
  }
}
