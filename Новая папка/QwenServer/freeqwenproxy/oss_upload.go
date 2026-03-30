package freeqwenproxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type StsTokenResponse struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecurityToken   string `json:"security_token"`

	Region    string `json:"region"`
	Bucket    string `json:"bucketname"`
	FileID    string `json:"file_id"`
	FilePath  string `json:"file_path"`
	FileURL   string `json:"file_url"`
	ExpiresAt string `json:"expiration,omitempty"`
}

type FileInfo struct {
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Filetype string `json:"filetype"`
}

func detectQwenFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "image"
	case ".pdf", ".doc", ".docx", ".txt":
		return "document"
	default:
		return "file"
	}
}

func (c *QwenClient) GetStsTokenRaw(ctx context.Context, info FileInfo) (json.RawMessage, *TokenEntry, error) {
	token, err := c.tokenSource.GetAvailableToken()
	if err != nil {
		return nil, nil, err
	}
	if token == nil || strings.TrimSpace(token.Token) == "" {
		return nil, nil, fmt.Errorf("no available qwen token")
	}

	raw, err := json.Marshal(info)
	if err != nil {
		return nil, token, err
	}
	u := c.baseURL + "/api/v1/files/getstsToken"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, token, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://chat.qwen.ai")
	req.Header.Set("Referer", "https://chat.qwen.ai/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, token, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, token, httpStatusError(resp.StatusCode, body)
	}

	return json.RawMessage(body), token, nil
}

func (c *QwenClient) GetStsToken(ctx context.Context, info FileInfo) (StsTokenResponse, *TokenEntry, error) {
	raw, token, err := c.GetStsTokenRaw(ctx, info)
	if err != nil {
		return StsTokenResponse{}, token, err
	}
	var sts StsTokenResponse
	if err := json.Unmarshal(raw, &sts); err != nil {
		return StsTokenResponse{}, token, fmt.Errorf("sts json: %w", err)
	}
	if strings.TrimSpace(sts.FilePath) == "" || strings.TrimSpace(sts.FileURL) == "" {
		return StsTokenResponse{}, token, fmt.Errorf("sts response missing file_path/file_url")
	}
	return sts, token, nil
}

func uploadToAliyunOSS(ctx context.Context, localPath string, sts StsTokenResponse) error {
	if strings.TrimSpace(sts.Region) == "" || strings.TrimSpace(sts.Bucket) == "" {
		return fmt.Errorf("sts missing region/bucketname")
	}
	if strings.TrimSpace(sts.AccessKeyID) == "" || strings.TrimSpace(sts.AccessKeySecret) == "" || strings.TrimSpace(sts.SecurityToken) == "" {
		return fmt.Errorf("sts missing credentials")
	}

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("local path is directory")
	}

	objectName := strings.TrimPrefix(sts.FilePath, "/")
	if objectName == "" {
		return fmt.Errorf("sts missing file_path")
	}

	host := sts.Bucket + "." + sts.Region + ".aliyuncs.com"
	reqURL := "https://" + host + "/" + objectName

	contentType := "application/octet-stream"
	date := time.Now().UTC().Format(http.TimeFormat)

	ossHeaders := map[string]string{
		"x-oss-security-token": sts.SecurityToken,
	}
	canonHeaders := canonicalizeOSSHeaders(ossHeaders)
	canonResource := "/" + sts.Bucket + "/" + objectName
	stringToSign := "PUT\n\n" + contentType + "\n" + date + "\n" + canonHeaders + canonResource

	mac := hmac.New(sha1.New, []byte(sts.AccessKeySecret))
	_, _ = mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authHeader := "OSS " + sts.AccessKeyID + ":" + signature

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, f)
	if err != nil {
		return err
	}
	req.Header.Set("Host", host)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", authHeader)
	for k, v := range ossHeaders {
		req.Header.Set(k, v)
	}
	req.ContentLength = st.Size()

	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return fmt.Errorf("oss put http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return nil
}

func canonicalizeOSSHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		kl := strings.ToLower(strings.TrimSpace(k))
		if strings.HasPrefix(kl, "x-oss-") {
			keys = append(keys, kl)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := strings.TrimSpace(h[k])
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.String()
}

func saveMultipartFile(uploadDir string, file multipart.File, header *multipart.FileHeader) (string, int64, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", 0, err
	}
	name := filepath.Base(header.Filename)
	unique := fmt.Sprintf("%d-%s-%s", time.Now().UnixMilli(), newHexID(8), name)
	dstPath := filepath.Join(uploadDir, unique)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", 0, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		_ = os.Remove(dstPath)
		return "", 0, err
	}
	return dstPath, written, nil
}
