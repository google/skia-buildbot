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

  // Tracks which columns are selected (visible), separate from ordering so
  // selection changes don't lose the custom order of columns (e.g. if a reordered
  // column is hidden and then shown again, it will reappear in its custom position).
  private _selectedColumnIds = signal<Set<string>>(new Set(this.allColumns.map((c) => c.id)));

  readonly selectedColumnIds = this._selectedColumnIds.asReadonly();

  // Tracks the custom horizontal layout order of all columns (both visible and hidden).
  private _orderedColumnIds = signal<string[]>(this.allColumns.map((c) => c.id));

  readonly displayedColumns = computed(() =>
    this._orderedColumnIds().filter((id) => this._selectedColumnIds().has(id))
  );

  updateSelection(updated: Set<string>) {
    this._selectedColumnIds.set(updated);
  }

  reorderColumns(previousIndex: number, currentIndex: number) {
    const movedColumn = this.displayedColumns()[previousIndex];
    const targetColumn = this.displayedColumns()[currentIndex];

    const fromIdx = this._orderedColumnIds().indexOf(movedColumn);
    const toIdx = this._orderedColumnIds().indexOf(targetColumn);
    if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) {
      return;
    }

    this._orderedColumnIds.update((currentOrdered) => {
      const newOrdered = [...currentOrdered];
      newOrdered.splice(fromIdx, 1);
      newOrdered.splice(toIdx, 0, movedColumn);
      return newOrdered;
    });
  }
}
