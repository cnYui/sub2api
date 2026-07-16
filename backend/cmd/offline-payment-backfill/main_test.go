package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBackfillArgsRequiresOperator(t *testing.T) {
	_, err := parseBackfillArgs(nil)
	require.ErrorContains(t, err, "operator is required")
}

func TestParseBackfillArgsRequiresConfirmTokenForExecute(t *testing.T) {
	_, err := parseBackfillArgs([]string{"--execute", "--operator=admin:13"})
	require.ErrorContains(t, err, "confirmation token")
}

func TestParseBackfillArgsAllowsDryRunWithOperator(t *testing.T) {
	opts, err := parseBackfillArgs([]string{"--dry-run", "--operator=admin:13"})
	require.NoError(t, err)
	require.False(t, opts.execute)
	require.Equal(t, "admin:13", opts.operator)
}

func TestParseBackfillArgsAllowsExecuteWithConfirmTokenAndOperator(t *testing.T) {
	opts, err := parseBackfillArgs([]string{
		"--execute",
		"--confirm=offline-paid-backfill-20260716",
		"--operator=admin:13",
	})
	require.NoError(t, err)
	require.True(t, opts.execute)
	require.Equal(t, "admin:13", opts.operator)
}

func TestParseBackfillArgsRejectsDryRunAndExecuteTogether(t *testing.T) {
	_, err := parseBackfillArgs([]string{
		"--dry-run",
		"--execute",
		"--confirm=offline-paid-backfill-20260716",
		"--operator=admin:13",
	})
	require.ErrorContains(t, err, "cannot be used together")
}
