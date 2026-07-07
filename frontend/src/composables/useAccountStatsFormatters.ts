export function useAccountStatsFormatters() {
  const formatCost = (value: number): string => {
    if (value >= 1000) {
      return (value / 1000).toFixed(2) + 'K'
    }
    if (value >= 1) {
      return value.toFixed(2)
    }
    if (value >= 0.01) {
      return value.toFixed(3)
    }
    return value.toFixed(4)
  }

  const formatNumber = (value: number): string => {
    if (value >= 1_000_000) {
      return (value / 1_000_000).toFixed(2) + 'M'
    }
    if (value >= 1_000) {
      return (value / 1_000).toFixed(2) + 'K'
    }
    return value.toLocaleString()
  }

  const formatTokens = (value: number): string => {
    if (value >= 1_000_000_000) {
      return `${(value / 1_000_000_000).toFixed(2)}B`
    }
    if (value >= 1_000_000) {
      return `${(value / 1_000_000).toFixed(2)}M`
    }
    if (value >= 1_000) {
      return `${(value / 1_000).toFixed(2)}K`
    }
    return value.toLocaleString()
  }

  const formatDuration = (ms: number): string => {
    if (ms >= 1000) {
      return `${(ms / 1000).toFixed(2)}s`
    }
    return `${Math.round(ms)}ms`
  }

  return {
    formatCost,
    formatNumber,
    formatTokens,
    formatDuration
  }
}
