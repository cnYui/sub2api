//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInspectOpenAIAttachmentsCountsImageDimensionsDetailAndQuantity(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()
	imageData := inlinePNGDataURL(t, 1024, 768)
	body := []byte(fmt.Sprintf(`{
"input":[{"type":"message","content":[
{"type":"input_image","image_url":"%s","detail":"high"},
{"type":"input_image","image_url":"%s","detail":"high"}
]}]
}`, imageData, imageData))

	got, err := inspector.Inspect(context.Background(), body)

	require.NoError(t, err)
	require.Len(t, got.Images, 2)
	require.Equal(t, 1024, got.Images[0].Width)
	require.Equal(t, 768, got.Images[0].Height)
	require.Equal(t, "high", got.Images[0].Detail)
}

func TestInspectOpenAIAttachmentsPreservesImageDetail(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()
	imageData := inlinePNGDataURL(t, 512, 512)
	body := []byte(fmt.Sprintf(`{
"input":[{"type":"message","content":[
{"type":"input_image","image_url":"%s","detail":"low"},
{"type":"input_image","image_url":"%s","detail":"high"},
{"type":"input_image","image_url":"%s","detail":"auto"}
]}]
}`, imageData, imageData, imageData))

	got, err := inspector.Inspect(context.Background(), body)

	require.NoError(t, err)
	require.Equal(t, []string{"low", "high", "auto"}, []string{
		got.Images[0].Detail,
		got.Images[1].Detail,
		got.Images[2].Detail,
	})
}

func TestInspectOpenAIAttachmentsRejectsUnresolvableFileID(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()

	_, err := inspector.Inspect(context.Background(), []byte(`{
"input":[{"type":"input_file","file_id":"file_1"}]
}`))

	require.ErrorIs(t, err, ErrOpenAIAttachmentNotLocallyReadable)
}

func TestInspectOpenAIAttachmentsPricesPDFTextAndEveryPageVision(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()
	pdfData := inlinePDFDataURL(t, "hello", "world")
	body := []byte(fmt.Sprintf(`{
"input":[{"type":"input_file","file_data":"%s"}]
}`, pdfData))

	got, err := inspector.Inspect(context.Background(), body)

	require.NoError(t, err)
	require.Len(t, got.PDFs, 1)
	require.Equal(t, 2, got.PDFs[0].PageCount)
	require.Greater(t, got.PDFs[0].TextTokens, 0)
	require.Len(t, got.PDFs[0].Pages, 2)
	require.Equal(t, 612, got.PDFs[0].Pages[0].Width)
	require.Equal(t, 792, got.PDFs[0].Pages[0].Height)
}

func TestInspectOpenAIAttachmentsReadsRawBase64PDF(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()
	rawPDF := base64.StdEncoding.EncodeToString(twoPagePDFWithText("hello", "world"))
	body := []byte(fmt.Sprintf(`{
"input":[{"type":"input_file","file_data":"%s"}]
}`, rawPDF))

	got, err := inspector.Inspect(context.Background(), body)

	require.NoError(t, err)
	require.Len(t, got.PDFs, 1)
	require.Equal(t, 2, got.PDFs[0].PageCount)
}

func TestInspectOpenAIAttachmentsRejectsInvalidAndOversizedPDF(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()

	_, err := inspector.Inspect(context.Background(), []byte(`{
"input":[{"type":"input_file","file_data":"data:application/pdf;base64,aGVsbG8="}]
}`))
	require.ErrorIs(t, err, ErrOpenAIAttachmentInvalid)

	overLimitPDF := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("x"), OpenAIAttachmentMaxBytes+1))
	_, err = inspector.Inspect(context.Background(), []byte(`{"input":[{"type":"input_file","file_data":"data:application/pdf;base64,`+overLimitPDF+`"}]}`))
	require.ErrorIs(t, err, ErrOpenAIAttachmentTooLarge)
}

func TestInspectOpenAIAttachmentsRejectsPDFOverPageLimit(t *testing.T) {
	inspector := NewOpenAIAttachmentInspector()
	pages := make([]string, OpenAIAttachmentMaxPages+1)
	body := []byte(fmt.Sprintf(`{"input":[{"type":"input_file","file_data":"%s"}]}`, inlinePDFDataURL(t, pages...)))

	_, err := inspector.Inspect(context.Background(), body)

	require.ErrorIs(t, err, ErrOpenAIAttachmentPageLimit)
}

func TestInspectOpenAIAttachmentsRejectsVideo(t *testing.T) {
	_, err := NewOpenAIAttachmentInspector().Inspect(context.Background(), []byte(`{
"input":[{"type":"input_file","file_data":"data:video/mp4;base64,aGVsbG8="}]
}`))

	require.ErrorIs(t, err, ErrOpenAIAttachmentInvalid)
}

func TestInspectOpenAIAttachmentsRejectsPrivateRemoteImageBeforeDownload(t *testing.T) {
	var downloads int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	body := []byte(fmt.Sprintf(`{
"input":[{"type":"input_image","image_url":"%s/image.png"}]
}`, server.URL))

	_, err := NewOpenAIAttachmentInspector().Inspect(context.Background(), body)

	require.ErrorIs(t, err, ErrOpenAIAttachmentInvalid)
	require.Zero(t, downloads)
}

func TestInspectOpenAIAttachmentsDownloadsRemoteImageWithinLimit(t *testing.T) {
	imageBytes := inlinePNGBytes(t, 1024, 768)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()

	body := []byte(fmt.Sprintf(`{
"input":[{"type":"input_image","image_url":"%s/image.png","detail":"high"}]
}`, server.URL))
	inspector := openAIAttachmentInspector{httpClient: server.Client(), allowPrivateHosts: true}

	got, err := inspector.Inspect(context.Background(), body)

	require.NoError(t, err)
	require.Len(t, got.Images, 1)
	require.Equal(t, 1024, got.Images[0].Width)
	require.Equal(t, 768, got.Images[0].Height)
}

func TestInspectOpenAIAttachmentsRejectsOversizedRemoteImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bytes.Repeat([]byte("x"), OpenAIAttachmentMaxBytes+1))
	}))
	defer server.Close()

	body := []byte(fmt.Sprintf(`{"input":[{"type":"input_image","image_url":"%s/image.png"}]}`, server.URL))
	inspector := openAIAttachmentInspector{httpClient: server.Client(), allowPrivateHosts: true}

	_, err := inspector.Inspect(context.Background(), body)

	require.ErrorIs(t, err, ErrOpenAIAttachmentTooLarge)
}

func TestInspectOpenAIAttachmentsTimesOutRemoteDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	body := []byte(fmt.Sprintf(`{"input":[{"type":"input_image","image_url":"%s/image.png"}]}`, server.URL))
	inspector := openAIAttachmentInspector{httpClient: server.Client(), allowPrivateHosts: true}

	_, err := inspector.Inspect(context.Background(), body)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestInspectOpenAIAttachmentsTimesOutPDFInspection(t *testing.T) {
	neverReturns := make(chan struct{})
	inspector := openAIAttachmentInspector{
		inspectPDF: func([]byte) (OpenAIPDFInspection, error) {
			<-neverReturns
			return OpenAIPDFInspection{}, nil
		},
	}
	body := []byte(fmt.Sprintf(`{"input":[{"type":"input_file","file_data":"%s"}]}`, inlinePDFDataURL(t, "hello")))

	_, err := inspector.Inspect(context.Background(), body)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func inlinePNGDataURL(t *testing.T, width, height int) string {
	t.Helper()

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(inlinePNGBytes(t, width, height))
}

func inlinePNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()

	imageValue := image.NewRGBA(image.Rect(0, 0, width, height))
	imageValue.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, imageValue))
	return encoded.Bytes()
}

func inlinePDFDataURL(t *testing.T, pages ...string) string {
	t.Helper()
	return "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(twoPagePDFWithText(pages...))
}

func twoPagePDFWithText(pages ...string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", pdfPageReferences(len(pages)), len(pages)),
	}
	for index := range pages {
		contentsID := 3 + len(pages) + index
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", 3+len(pages)*2, contentsID))
	}
	for _, text := range pages {
		stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&document, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return document.Bytes()
}

func pdfPageReferences(pageCount int) string {
	var refs bytes.Buffer
	for index := 0; index < pageCount; index++ {
		if index > 0 {
			refs.WriteByte(' ')
		}
		refs.WriteString(strconv.Itoa(index + 3))
		refs.WriteString(" 0 R")
	}
	return refs.String()
}
