package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyUpstreamPassthroughHeadersSkipsHopByHopAndPreservesValues(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("X-Custom", "first")
	src.Add("X-Custom", "second")
	src.Add("Content-Length", "42")
	src.Add("Transfer-Encoding", "chunked")
	src.Add("Connection", "keep-alive")

	dst := http.Header{}
	copyUpstreamPassthroughHeaders(dst, src)

	require.Equal(t, []string{"application/json"}, dst.Values("Content-Type"))
	require.Equal(t, []string{"first", "second"}, dst.Values("X-Custom"))
	require.Empty(t, dst.Values("Content-Length"))
	require.Empty(t, dst.Values("Transfer-Encoding"))
	require.Empty(t, dst.Values("Connection"))
}
