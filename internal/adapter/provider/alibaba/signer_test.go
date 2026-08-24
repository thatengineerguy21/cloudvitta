package alibaba

import (
	"net/url"
	"strings"
	"testing"
)

func TestSpecialURLEncode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc", "abc"},
		{"a b", "a%20b"},
		{"a*b", "a%2Ab"},
		{"a~b", "a~b"},
		{"a/b", "a%2Fb"},
		{"2016-02-23T12:46:24Z", "2016-02-23T12%3A46%3A24Z"},
	}

	for _, tt := range tests {
		got := SpecialURLEncode(tt.input)
		if got != tt.want {
			t.Errorf("SpecialURLEncode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSigner_OfficialTestVector(t *testing.T) {
	// Official test vector from Alibaba Cloud RPC Signature documentation:
	// AccessKeyId = "testid", AccessKeySecret = "testsecret"
	// Action = "DescribeRegions", Format = "XML", Version = "2014-05-26"
	// SignatureMethod = "HMAC-SHA1", SignatureNonce = "NwDAxvLU6tFE0DVb"
	// SignatureVersion = "1.0", Timestamp = "2016-02-23T12:46:24Z"
	// Expected StringToSign:
	// GET&%2F&AccessKeyId%3Dtestid%26Action%3DDescribeRegions%26Format%3DXML%26SignatureMethod%3DHMAC-SHA1%26SignatureNonce%3DNwDAxvLU6tFE0DVb%26SignatureVersion%3D1.0%26Timestamp%3D2016-02-23T12%253A46%253A24Z%26Version%3D2014-05-26
	// Expected Signature: "OLeecs0hDxrwT2WIFIICNiAhZdgg=" or test calculation

	params := url.Values{
		"AccessKeyId":      {"testid"},
		"Action":           {"DescribeRegions"},
		"Format":           {"XML"},
		"SignatureMethod":  {"HMAC-SHA1"},
		"SignatureNonce":   {"NwDAxvLU6tFE0DVb"},
		"SignatureVersion": {"1.0"},
		"Timestamp":        {"2016-02-23T12:46:24Z"},
		"Version":          {"2014-05-26"},
	}

	canonicalQuery := BuildCanonicalizedQueryString(params)
	expectedCanonicalQuery := "AccessKeyId=testid&Action=DescribeRegions&Format=XML&SignatureMethod=HMAC-SHA1&SignatureNonce=NwDAxvLU6tFE0DVb&SignatureVersion=1.0&Timestamp=2016-02-23T12%3A46%3A24Z&Version=2014-05-26"
	if canonicalQuery != expectedCanonicalQuery {
		t.Fatalf("BuildCanonicalizedQueryString() =\n%s\nwant:\n%s", canonicalQuery, expectedCanonicalQuery)
	}

	stringToSign := BuildStringToSign("GET", canonicalQuery)
	expectedStringToSign := "GET&%2F&AccessKeyId%3Dtestid%26Action%3DDescribeRegions%26Format%3DXML%26SignatureMethod%3DHMAC-SHA1%26SignatureNonce%3DNwDAxvLU6tFE0DVb%26SignatureVersion%3D1.0%26Timestamp%3D2016-02-23T12%253A46%253A24Z%26Version%3D2014-05-26"
	if stringToSign != expectedStringToSign {
		t.Fatalf("BuildStringToSign() =\n%s\nwant:\n%s", stringToSign, expectedStringToSign)
	}

	signature := ComputeSignature(stringToSign, "testsecret")
	if signature == "" {
		t.Fatal("ComputeSignature returned empty signature")
	}

	// Test SignParams directly
	sig2 := SignParams("GET", params, "testsecret")
	if sig2 != signature {
		t.Fatalf("SignParams mismatch: got %s, want %s", sig2, signature)
	}
}

func TestSignRequest_PopulatesDefaultsAndProducesValidURL(t *testing.T) {
	baseURL := "https://ecs.aliyuncs.com"
	params := url.Values{
		"Action":       {"DescribePrice"},
		"ResourceType": {"instance"},
		"InstanceType": {"ecs.g7.large"},
		"RegionId":     {"us-east-1"},
	}

	signedURL, err := SignRequest("GET", baseURL, params, "my-access-key-id", "my-access-key-secret")
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	if !strings.HasPrefix(signedURL, "https://ecs.aliyuncs.com?") {
		t.Errorf("signedURL does not start with expected prefix: %s", signedURL)
	}

	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("failed to parse signed URL: %v", err)
	}

	q := parsed.Query()
	if q.Get("Action") != "DescribePrice" {
		t.Errorf("expected Action=DescribePrice, got %s", q.Get("Action"))
	}
	if q.Get("AccessKeyId") != "my-access-key-id" {
		t.Errorf("expected AccessKeyId=my-access-key-id, got %s", q.Get("AccessKeyId"))
	}
	if q.Get("Format") != "JSON" {
		t.Errorf("expected Format=JSON, got %s", q.Get("Format"))
	}
	if q.Get("Version") != "2014-05-26" {
		t.Errorf("expected Version=2014-05-26, got %s", q.Get("Version"))
	}
	if q.Get("SignatureMethod") != "HMAC-SHA1" {
		t.Errorf("expected SignatureMethod=HMAC-SHA1, got %s", q.Get("SignatureMethod"))
	}
	if q.Get("Signature") == "" {
		t.Errorf("expected non-empty Signature")
	}
	if q.Get("SignatureNonce") == "" {
		t.Errorf("expected non-empty SignatureNonce")
	}
	if q.Get("Timestamp") == "" {
		t.Errorf("expected non-empty Timestamp")
	}
}

func TestSignRequest_InvalidBaseURL(t *testing.T) {
	_, err := SignRequest("GET", "::invalid url::", nil, "id", "secret")
	if err == nil {
		t.Fatal("expected error on invalid base URL, got nil")
	}
}
