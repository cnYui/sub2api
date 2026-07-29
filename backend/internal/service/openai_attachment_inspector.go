package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/ledongthuc/pdf"
)

const (
	OpenAIAttachmentMaxBytes       = 20 * 1024 * 1024
	OpenAIAttachmentMaxPages       = 200
	openAIAttachmentInspectTimeout = 2 * time.Second
	openAIAttachmentMaxRedirects   = 3
)

var ErrOpenAIAttachmentNotLocallyReadable = errors.New("openai attachment is not locally readable")
var ErrOpenAIAttachmentInvalid = errors.New("openai attachment is invalid")
var ErrOpenAIAttachmentTooLarge = errors.New("openai attachment exceeds size limit")
var ErrOpenAIAttachmentPageLimit = errors.New("openai attachment exceeds page limit")

type OpenAIAttachmentInspector interface {
	Inspect(context.Context, []byte) (OpenAIAttachmentInspection, error)
}

type OpenAIAttachmentInspection struct {
	Images []OpenAIImageInput
	PDFs   []OpenAIPDFInspection
}

type OpenAIImageInput struct {
	Width  int
	Height int
	Detail string
}

type OpenAIPDFInspection struct {
	Text       string
	TextTokens int
	PageCount  int
	Pages      []OpenAIImageInput
}

type openAIAttachmentInspector struct {
	httpClient        *http.Client
	allowPrivateHosts bool
	inspectPDF        func([]byte) (OpenAIPDFInspection, error)
}

func NewOpenAIAttachmentInspector() OpenAIAttachmentInspector {
	return openAIAttachmentInspector{
		httpClient: newOpenAIAttachmentHTTPClient(false),
		inspectPDF: inspectOpenAIInlinePDFBytes,
	}
}

func (i openAIAttachmentInspector) Inspect(ctx context.Context, body []byte) (OpenAIAttachmentInspection, error) {
	if err := ctx.Err(); err != nil {
		return OpenAIAttachmentInspection{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, openAIAttachmentInspectTimeout)
	defer cancel()

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return OpenAIAttachmentInspection{}, ErrOpenAIAttachmentInvalid
	}

	inspection := OpenAIAttachmentInspection{}
	if err := inspectOpenAIAttachmentValue(ctx, decoded, &inspection, i); err != nil {
		return OpenAIAttachmentInspection{}, err
	}
	return inspection, nil
}

func inspectOpenAIAttachmentValue(ctx context.Context, value any, inspection *OpenAIAttachmentInspection, inspector openAIAttachmentInspector) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			if err := inspectOpenAIAttachmentValue(ctx, child, inspection, inspector); err != nil {
				return err
			}
		}
	case map[string]any:
		if isOpenAIFileIDReference(typed) {
			return ErrOpenAIAttachmentNotLocallyReadable
		}
		if fileData, ok := typed["file_data"].(string); ok && strings.TrimSpace(fileData) != "" {
			bytesValue, mediaType, err := decodeOpenAIInlineAttachment(fileData, openAIAttachmentMediaType(typed))
			if err != nil {
				return err
			}
			switch {
			case mediaType == "application/pdf":
				inspectionPDF, inspectErr := inspector.inspectOpenAIInlinePDF(ctx, bytesValue)
				if inspectErr != nil {
					return inspectErr
				}
				inspection.PDFs = append(inspection.PDFs, inspectionPDF)
			case strings.HasPrefix(mediaType, "image/"):
				imageInput, inspectErr := inspectOpenAIImageBytes(bytesValue, mediaType, "auto")
				if inspectErr != nil {
					return inspectErr
				}
				inspection.Images = append(inspection.Images, imageInput)
			default:
				return ErrOpenAIAttachmentInvalid
			}
			return nil
		}
		if imageURL, detail, ok := openAIImageReference(typed); ok {
			imageInput, err := inspector.inspectOpenAIImage(ctx, imageURL, detail)
			if err != nil {
				return err
			}
			inspection.Images = append(inspection.Images, imageInput)
			return nil
		}
		for _, child := range typed {
			if err := inspectOpenAIAttachmentValue(ctx, child, inspection, inspector); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i openAIAttachmentInspector) inspectOpenAIInlinePDF(ctx context.Context, bytesValue []byte) (OpenAIPDFInspection, error) {
	type result struct {
		inspection OpenAIPDFInspection
		err        error
	}
	inspectPDF := i.inspectPDF
	if inspectPDF == nil {
		inspectPDF = inspectOpenAIInlinePDFBytes
	}
	resultCh := make(chan result, 1)
	go func() {
		inspection, err := inspectPDF(bytesValue)
		resultCh <- result{inspection: inspection, err: err}
	}()

	select {
	case result := <-resultCh:
		return result.inspection, result.err
	case <-ctx.Done():
		return OpenAIPDFInspection{}, ctx.Err()
	}
}

func inspectOpenAIInlinePDFBytes(bytesValue []byte) (OpenAIPDFInspection, error) {
	reader, err := pdf.NewReader(bytes.NewReader(bytesValue), int64(len(bytesValue)))
	if err != nil {
		return OpenAIPDFInspection{}, ErrOpenAIAttachmentInvalid
	}
	pageCount := reader.NumPage()
	if pageCount <= 0 {
		return OpenAIPDFInspection{}, ErrOpenAIAttachmentInvalid
	}
	if pageCount > OpenAIAttachmentMaxPages {
		return OpenAIPDFInspection{}, ErrOpenAIAttachmentPageLimit
	}

	plainText, err := reader.GetPlainText()
	if err != nil {
		return OpenAIPDFInspection{}, ErrOpenAIAttachmentInvalid
	}
	textBytes, err := io.ReadAll(plainText)
	if err != nil {
		return OpenAIPDFInspection{}, ErrOpenAIAttachmentInvalid
	}

	pages := make([]OpenAIImageInput, 0, pageCount)
	for pageIndex := 1; pageIndex <= pageCount; pageIndex++ {
		page := reader.Page(pageIndex)
		width, height, ok := openAIPDFPageDimensions(page.V.Key("MediaBox"))
		if !ok {
			return OpenAIPDFInspection{}, ErrOpenAIAttachmentInvalid
		}
		pages = append(pages, OpenAIImageInput{Width: width, Height: height, Detail: "high"})
	}

	text := string(textBytes)
	return OpenAIPDFInspection{
		Text:       text,
		TextTokens: estimateOpenAIStringTokens(text),
		PageCount:  pageCount,
		Pages:      pages,
	}, nil
}

func openAIPDFPageDimensions(mediaBox pdf.Value) (int, int, bool) {
	if mediaBox.Len() != 4 {
		return 0, 0, false
	}
	width := int(math.Ceil(math.Abs(mediaBox.Index(2).Float64() - mediaBox.Index(0).Float64())))
	height := int(math.Ceil(math.Abs(mediaBox.Index(3).Float64() - mediaBox.Index(1).Float64())))
	return width, height, width > 0 && height > 0
}

func isOpenAIFileIDReference(value map[string]any) bool {
	fileID, _ := value["file_id"].(string)
	return strings.TrimSpace(fileID) != ""
}

func openAIImageReference(value map[string]any) (string, string, bool) {
	imageURL, exists := value["image_url"]
	if !exists {
		return "", "", false
	}

	detail, _ := value["detail"].(string)
	if strings.TrimSpace(detail) == "" {
		detail = "auto"
	}
	switch typed := imageURL.(type) {
	case string:
		return typed, detail, strings.TrimSpace(typed) != ""
	case map[string]any:
		url, _ := typed["url"].(string)
		if nestedDetail, ok := typed["detail"].(string); ok && strings.TrimSpace(nestedDetail) != "" {
			detail = nestedDetail
		}
		return url, detail, strings.TrimSpace(url) != ""
	default:
		return "", "", false
	}
}

func (i openAIAttachmentInspector) inspectOpenAIImage(ctx context.Context, rawURL, detail string) (OpenAIImageInput, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "data:") {
		bytesValue, mediaType, err := decodeOpenAIInlineAttachment(rawURL, "")
		if err != nil {
			return OpenAIImageInput{}, err
		}
		return inspectOpenAIImageBytes(bytesValue, mediaType, detail)
	}

	bytesValue, mediaType, err := i.downloadOpenAIImage(ctx, rawURL)
	if err != nil {
		return OpenAIImageInput{}, err
	}
	return inspectOpenAIImageBytes(bytesValue, mediaType, detail)
}

func inspectOpenAIImageBytes(bytesValue []byte, mediaType, detail string) (OpenAIImageInput, error) {
	mediaType = normalizeOpenAIAttachmentMediaType(mediaType)
	if !strings.HasPrefix(mediaType, "image/") {
		return OpenAIImageInput{}, ErrOpenAIAttachmentInvalid
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(bytesValue))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return OpenAIImageInput{}, ErrOpenAIAttachmentInvalid
	}
	return OpenAIImageInput{Width: config.Width, Height: config.Height, Detail: strings.ToLower(strings.TrimSpace(detail))}, nil
}

func (i openAIAttachmentInspector) downloadOpenAIImage(ctx context.Context, rawURL string) ([]byte, string, error) {
	normalizedURL, err := validateOpenAIAttachmentURL(ctx, rawURL, i.allowPrivateHosts)
	if err != nil {
		return nil, "", err
	}
	client := i.httpClient
	if client == nil {
		client = newOpenAIAttachmentHTTPClient(i.allowPrivateHosts)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	bytesValue, err := io.ReadAll(io.LimitReader(response.Body, OpenAIAttachmentMaxBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	if len(bytesValue) > OpenAIAttachmentMaxBytes {
		return nil, "", ErrOpenAIAttachmentTooLarge
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		mediaType = ""
	}
	if mediaType == "" {
		mediaType = http.DetectContentType(bytesValue)
	}
	return bytesValue, mediaType, nil
}

func validateOpenAIAttachmentURL(ctx context.Context, rawURL string, allowPrivateHosts bool) (string, error) {
	normalizedURL, err := urlvalidator.ValidateHTTPURL(rawURL, true, urlvalidator.ValidationOptions{AllowPrivate: allowPrivateHosts})
	if err != nil {
		return "", ErrOpenAIAttachmentInvalid
	}
	parsed, err := url.Parse(normalizedURL)
	if err != nil {
		return "", ErrOpenAIAttachmentInvalid
	}
	if !allowPrivateHosts {
		if _, err := urlvalidator.ResolvePublicHost(ctx, parsed.Hostname()); err != nil {
			return "", ErrOpenAIAttachmentInvalid
		}
	}
	return normalizedURL, nil
}

func newOpenAIAttachmentHTTPClient(allowPrivateHosts bool) *http.Client {
	dialer := &net.Dialer{}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if allowPrivateHosts {
			return dialer.DialContext(ctx, network, address)
		}
		ips, err := urlvalidator.ResolvePublicHost(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	client := &http.Client{Transport: transport}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= openAIAttachmentMaxRedirects {
			return ErrOpenAIAttachmentInvalid
		}
		_, err := validateOpenAIAttachmentURL(request.Context(), request.URL.String(), allowPrivateHosts)
		return err
	}
	return client
}

func decodeOpenAIInlineAttachment(value, declaredMediaType string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	mediaType := normalizeOpenAIAttachmentMediaType(declaredMediaType)
	encoded := value
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		separator := strings.IndexByte(value, ',')
		if separator <= len("data:") {
			return nil, "", ErrOpenAIAttachmentInvalid
		}
		metadata := strings.ToLower(strings.TrimSpace(value[len("data:"):separator]))
		if !strings.Contains(metadata, ";base64") {
			return nil, "", ErrOpenAIAttachmentInvalid
		}
		mediaType = normalizeOpenAIAttachmentMediaType(strings.Split(metadata, ";")[0])
		encoded = strings.TrimSpace(value[separator+1:])
	}
	if encoded == "" {
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(OpenAIAttachmentMaxBytes) {
		return nil, "", ErrOpenAIAttachmentTooLarge
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	if len(decoded) > OpenAIAttachmentMaxBytes {
		return nil, "", ErrOpenAIAttachmentTooLarge
	}
	if mediaType == "" {
		mediaType = normalizeOpenAIAttachmentMediaType(http.DetectContentType(decoded))
	}
	if mediaType == "" {
		return nil, "", ErrOpenAIAttachmentInvalid
	}
	return decoded, mediaType, nil
}

func openAIAttachmentMediaType(value map[string]any) string {
	for _, field := range []string{"mime_type", "content_type", "file_type"} {
		if mediaType, ok := value[field].(string); ok && strings.TrimSpace(mediaType) != "" {
			return mediaType
		}
	}
	if filename, ok := value["filename"].(string); ok {
		switch {
		case strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".pdf"):
			return "application/pdf"
		case strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".png"):
			return "image/png"
		case strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".jpg"), strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".jpeg"):
			return "image/jpeg"
		}
	}
	return ""
}

func normalizeOpenAIAttachmentMediaType(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}
