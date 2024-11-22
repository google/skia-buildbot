import { errorMessage as elementsErrorMessage } from '../../../elements-sk/modules/errorMessage';

/**
 * This is the same function as element-sk errorMessage, but defaults to a 0s
 * delay, which means the toast doesn't close automatically.
 */
export const errorMessage = (
  message: string | { message: string } | { resp: Response } | object,
  duration: number = 0
): void => {
  elementsErrorMessage(message, duration);
};
