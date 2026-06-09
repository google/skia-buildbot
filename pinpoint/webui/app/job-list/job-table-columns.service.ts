import { Injectable, signal, computed, inject } from '@angular/core';
import { SettingsService } from '../settings/settings.service';

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

  private settingsService = inject(SettingsService);

  readonly defaultColumnOrder: ColumnInfo[] = this.allColumns;

  // Tracks which columns are selected (visible), separate from ordering so
  // selection changes don't lose the custom order of columns (e.g. if a reordered
  // column is hidden and then shown again, it will reappear in its custom position).
  private _selectedColumnIds = signal<Set<string>>(
    new Set(this.settingsService.getSelectedColumns(this.defaultColumnOrder.map((c) => c.id)))
  );

  readonly selectedColumnIds = this._selectedColumnIds.asReadonly();

  // Tracks the custom horizontal layout order of all columns (both visible and hidden).
  private _orderedColumnIds = signal<string[]>(
    this.settingsService.getOrderedColumns(this.defaultColumnOrder.map((c) => c.id))
  );

  readonly displayedColumns = computed(() =>
    this._orderedColumnIds().filter((id) => this._selectedColumnIds().has(id))
  );

  updateSelection(updated: Set<string>) {
    this._selectedColumnIds.set(updated);
    this.settingsService.setSelectedColumns([...updated]);
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
      this.settingsService.setOrderedColumns(newOrdered);
      return newOrdered;
    });
  }

  resetToDefault() {
    const defaultColumnIds = this.defaultColumnOrder.map((c) => c.id as string);
    this._selectedColumnIds.set(new Set(defaultColumnIds));
    this._orderedColumnIds.set(defaultColumnIds);
    this.settingsService.setSelectedColumns(defaultColumnIds);
    this.settingsService.setOrderedColumns(defaultColumnIds);
  }
}
