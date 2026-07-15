export const isValidUserConcurrency = (value: number): boolean =>
  Number.isFinite(value) && Number.isInteger(value) && value >= 0
