

export const EXIT_OK = 0;

export const EXIT_ERROR = 1;

export const EXIT_AUTH = 2;

export const EXIT_FORBIDDEN = 3;

export const EXIT_NOT_FOUND = 4;

export const EXIT_PAYMENT = 5;

export const EXIT_VALIDATION = 6;

export const EXIT_RATE_LIMIT = 7;

export const EXIT_UNAVAILABLE = 8;

export const EXIT_TIMEOUT = 9;

export function exitCodeForStatus(status: number): number {
  if (status === 0) return EXIT_ERROR;
  if (status >= 200 && status < 300) return EXIT_OK;
  switch (status) {
    case 401:
      return EXIT_AUTH;
    case 403:
      return EXIT_FORBIDDEN;
    case 404:
      return EXIT_NOT_FOUND;
    case 402:
      return EXIT_PAYMENT;
    case 400:
    case 422:
      return EXIT_VALIDATION;
    case 429:
      return EXIT_RATE_LIMIT;
    case 504:
      return EXIT_TIMEOUT;
    default:
      return status >= 500 ? EXIT_UNAVAILABLE : EXIT_ERROR;
  }
}
