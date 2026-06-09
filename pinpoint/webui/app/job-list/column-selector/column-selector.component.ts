import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatMenuModule } from '@angular/material/menu';
import { MatDividerModule } from '@angular/material/divider';
import { JobTableColumnsService, ColumnInfo } from '../job-table-columns.service';

@Component({
  selector: 'app-column-selector',
  standalone: true,
  imports: [
    MatButtonModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
    MatCheckboxModule,
    MatMenuModule,
    MatDividerModule,
    FormsModule,
  ],
  templateUrl: './column-selector.component.html',
  styleUrls: ['./column-selector.component.css'],
})
export class ColumnSelectorComponent {
  private columnsService = inject(JobTableColumnsService);

  readonly allColumns = [...this.columnsService.allColumns].sort((a, b) =>
    a.label.localeCompare(b.label)
  );

  columnSearchQuery = '';

  get selectedColumns(): Set<string> {
    return this.columnsService.selectedColumnIds();
  }

  get filteredColumns(): ColumnInfo[] {
    const query = this.columnSearchQuery.toLowerCase().trim();
    if (!query) {
      return this.allColumns;
    }
    return this.allColumns.filter((c) => c.label.toLowerCase().includes(query));
  }

  get allSelected(): boolean {
    return this.selectedColumns.size === this.allColumns.length;
  }

  get someSelected(): boolean {
    return this.selectedColumns.size > 0 && !this.allSelected;
  }

  toggleColumn(columnId: string, checked: boolean) {
    const updated = new Set(this.selectedColumns);
    if (checked) {
      updated.add(columnId);
    } else {
      updated.delete(columnId);
    }
    this.columnsService.updateSelection(updated);
  }

  toggleAll(checked: boolean) {
    const updated = checked ? new Set<string>(this.allColumns.map((c) => c.id)) : new Set<string>();
    this.columnsService.updateSelection(updated);
  }
}
