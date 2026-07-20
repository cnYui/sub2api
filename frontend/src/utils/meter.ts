export function meterScale(
  value: number | null | undefined,
  limit: number | null | undefined,
): number {
  if (
    value == null ||
    limit == null ||
    !Number.isFinite(value) ||
    !Number.isFinite(limit) ||
    limit <= 0
  ) {
    return 0
  }

  return Math.min(Math.max(value / limit, 0), 1)
}
