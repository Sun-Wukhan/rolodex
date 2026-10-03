import { ApiError } from './client';

export interface DescribedError {
  message: string;
  requestId?: string;
  fields: Record<string, string>;
}

/** Converts any thrown value into a user-presentable message. */
export function describeError(err: unknown): DescribedError {
  if (err instanceof ApiError) {
    const message =
      err.status === 0
        ? 'Unable to reach the Rolodex API. Is it running?'
        : err.status === 429
          ? 'Too many attempts. Please wait a minute and try again.'
          : err.message;
    return { message, requestId: err.requestId, fields: err.fields };
  }
  return { message: 'Something went wrong. Please try again.', fields: {} };
}
