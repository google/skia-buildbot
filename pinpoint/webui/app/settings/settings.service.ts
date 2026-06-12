import { Injectable } from '@angular/core';

export enum SettingKey {
  ShowOnlyUserJobs = 'show_only_user_jobs',
  OrderedColumns = 'ordered_columns',
  SelectedColumns = 'selected_columns',
  Theme = 'theme',
}

export enum Theme {
  Dark = 'dark',
  Light = 'light',
}

@Injectable({
  providedIn: 'root',
})
export class SettingsService {
  getTheme(defaultValue = Theme.Light): Theme {
    return this.read(SettingKey.Theme, defaultValue);
  }

  setTheme(value: Theme): void {
    this.write(SettingKey.Theme, value);
  }

  getShowOnlyUserJobs(defaultValue: boolean): boolean {
    return this.read(SettingKey.ShowOnlyUserJobs, defaultValue);
  }

  setShowOnlyUserJobs(value: boolean): void {
    this.write(SettingKey.ShowOnlyUserJobs, value);
  }

  getOrderedColumns(defaultValue: string[]): string[] {
    return this.read(SettingKey.OrderedColumns, defaultValue);
  }

  setOrderedColumns(value: string[]): void {
    this.write(SettingKey.OrderedColumns, value);
  }

  getSelectedColumns(defaultValue: string[]): string[] {
    return this.read(SettingKey.SelectedColumns, defaultValue);
  }

  setSelectedColumns(value: string[]): void {
    this.write(SettingKey.SelectedColumns, value);
  }

  private read<T>(key: SettingKey, defaultValue: T): T {
    const value = localStorage.getItem(key);
    if (value === null) {
      return defaultValue;
    }

    try {
      return JSON.parse(value) as T;
    } catch (e) {
      console.error(`Failed to parse key "${key}" from localStorage`, e);
      return defaultValue;
    }
  }

  private write<T>(key: SettingKey, value: T): void {
    localStorage.setItem(key, JSON.stringify(value));
  }
}
