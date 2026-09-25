/** Extract a user-facing message from an API/Axios-style error. */
export function apiErrorMessage(error: unknown, fallback: string): string {
  const responseError = (error as { response?: { data?: { error?: unknown } } })?.response?.data?.error;
  if (typeof responseError === 'string' && responseError) {
    return responseError;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}
