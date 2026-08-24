package alibaba

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SpecialURLEncode encodes a string using RFC 3986 percent-encoding conventions required by Alibaba Cloud RPC APIs.
// Specifically, space is encoded as %20 (not +), * as %2A, and ~ remains unescaped (~).
func SpecialURLEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// BuildCanonicalizedQueryString sorts parameter keys alphabetically and constructs the canonicalized query string.
func BuildCanonicalizedQueryString(params url.Values) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		vals := params[k]
		for _, v := range vals {
			pairs = append(pairs, SpecialURLEncode(k)+"="+SpecialURLEncode(v))
		}
	}

	return strings.Join(pairs, "&")
}

// BuildStringToSign builds the string-to-sign per Alibaba Cloud RPC signature specification.
// Format: HTTPMethod + "&" + SpecialURLEncode("/") + "&" + SpecialURLEncode(CanonicalizedQueryString)
func BuildStringToSign(method, canonicalizedQueryString string) string {
	return strings.ToUpper(method) + "&" + SpecialURLEncode("/") + "&" + SpecialURLEncode(canonicalizedQueryString)
}

// ComputeSignature calculates the HMAC-SHA1 signature using key (AccessKeySecret + "&") and returns the base64 string.
func ComputeSignature(stringToSign, accessKeySecret string) string {
	key := []byte(accessKeySecret + "&")
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// SignParams calculates the HMAC-SHA1 signature for a set of parameters.
func SignParams(method string, params url.Values, accessKeySecret string) string {
	canonicalQuery := BuildCanonicalizedQueryString(params)
	stringToSign := BuildStringToSign(method, canonicalQuery)
	return ComputeSignature(stringToSign, accessKeySecret)
}

// SignRequest constructs a signed URL for Alibaba Cloud RPC APIs.
// If common parameters (Format, Version, AccessKeyId, SignatureMethod, Timestamp, SignatureVersion, SignatureNonce)
// are not present, standard defaults are populated automatically.
func SignRequest(method, rawURL string, params url.Values, accessKeyID, accessKeySecret string) (string, error) {
	if params == nil {
		params = make(url.Values)
	}

	clone := make(url.Values, len(params)+8)
	for k, v := range params {
		clone[k] = append([]string(nil), v...)
	}

	if clone.Get("Format") == "" {
		clone.Set("Format", "JSON")
	}
	if clone.Get("Version") == "" {
		clone.Set("Version", "2014-05-26")
	}
	if clone.Get("AccessKeyId") == "" && accessKeyID != "" {
		clone.Set("AccessKeyId", accessKeyID)
	}
	if clone.Get("SignatureMethod") == "" {
		clone.Set("SignatureMethod", "HMAC-SHA1")
	}
	if clone.Get("Timestamp") == "" {
		clone.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	}
	if clone.Get("SignatureVersion") == "" {
		clone.Set("SignatureVersion", "1.0")
	}
	if clone.Get("SignatureNonce") == "" {
		clone.Set("SignatureNonce", uuid.New().String())
	}

	sig := SignParams(method, clone, accessKeySecret)
	canonicalQuery := BuildCanonicalizedQueryString(clone)
	signedQuery := canonicalQuery + "&Signature=" + SpecialURLEncode(sig)

	baseURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("alibaba signer: parse URL %q: %w", rawURL, err)
	}

	if baseURL.RawQuery != "" {
		baseURL.RawQuery = baseURL.RawQuery + "&" + signedQuery
	} else {
		baseURL.RawQuery = signedQuery
	}

	return baseURL.String(), nil
}
