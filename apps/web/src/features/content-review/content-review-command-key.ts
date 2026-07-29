export interface ReviewCommandKey {
  signature: string;
  key: string;
}

export function reviewCommandKey(
  current: ReviewCommandKey | null,
  signature: string,
  create: () => string = () => crypto.randomUUID(),
): ReviewCommandKey {
  return current?.signature === signature
    ? current
    : { signature, key: create() };
}

export function isUncertainReviewCommandError(cause: unknown) {
  const status =
    typeof cause === "object" && cause !== null && "status" in cause
      ? Number((cause as { status?: unknown }).status)
      : 0;
  return !Number.isFinite(status) || status === 0 || status >= 500;
}
