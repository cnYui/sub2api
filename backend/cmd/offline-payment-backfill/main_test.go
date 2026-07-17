package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBackfillArgs(t *testing.T) {
	t.Run("必须提供执行人", func(t *testing.T) {
		_, err := parseBackfillArgs(nil)
		require.ErrorContains(t, err, "operator is required")
	})

	t.Run("默认使用 dry-run", func(t *testing.T) {
		opts, err := parseBackfillArgs([]string{"--operator", "admin:13"})
		require.NoError(t, err)
		require.False(t, opts.execute)
		require.Equal(t, "admin:13", opts.operator)
	})

	t.Run("执行必须确认", func(t *testing.T) {
		_, err := parseBackfillArgs([]string{"--execute", "--operator", "admin:13"})
		require.Error(t, err)
	})

	t.Run("确认后允许执行", func(t *testing.T) {
		opts, err := parseBackfillArgs([]string{
			"--execute",
			"--confirm", offlinePaymentBackfillConfirmToken,
			"--operator", "admin:13",
		})
		require.NoError(t, err)
		require.True(t, opts.execute)
	})

	t.Run("不能同时指定 dry-run 和 execute", func(t *testing.T) {
		_, err := parseBackfillArgs([]string{
			"--dry-run",
			"--execute",
			"--confirm", offlinePaymentBackfillConfirmToken,
			"--operator", "admin:13",
		})
		require.ErrorContains(t, err, "cannot be used together")
	})
}
