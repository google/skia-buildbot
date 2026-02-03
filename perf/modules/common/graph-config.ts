import { DataService, DataServiceError } from '../data-service/data-service';
import { errorMessage } from '../errorMessage';

export class GraphConfig {
  formulas: string[] = []; // Formulas

  queries: string[] = []; // Queries

  keys: string = ''; // Keys
}

/**
 * Creates a shortcut ID for the given Graph Configs.
 *
 */
export const updateShortcut = async (graphConfigs: GraphConfig[]): Promise<string> => {
  try {
    return await DataService.getInstance().updateShortcut(graphConfigs);
  } catch (err: unknown) {
    if (err instanceof DataServiceError) {
      if (err.status === 500) {
        errorMessage('Unable to update shortcut.', 2000);
      } else {
        errorMessage(err.message);
      }
    } else {
      errorMessage(err as string);
    }
    return '';
  }
};
