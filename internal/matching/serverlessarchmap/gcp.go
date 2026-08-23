package serverlessarchmap

import (
	"fmt"
	"strings"
)

var gcpArchMap = map[string]string{
	"x86_64":               ArchX86_64,
	"x86":                  ArchX86_64,
	"x86-64":               ArchX86_64,
	"amd64":                ArchX86_64,
	"intel":                ArchX86_64,
	"cloud functions":      ArchX86_64,
	"cloud run functions":  ArchX86_64,
	"1st_gen":              ArchX86_64,
	"2nd_gen":              ArchX86_64,
	"invocations":          ArchX86_64,
	"function invocations": ArchX86_64,
	"execution time":       ArchX86_64,
	"memory time":          ArchX86_64,
	"cpu time":             ArchX86_64,
	"gb-seconds":           ArchX86_64,
	"ghz-seconds":          ArchX86_64,
	"gib-seconds":          ArchX86_64,
	"vcpu-seconds":         ArchX86_64,
}

// MapGCPArchitecture maps a GCP Cloud Functions SKU description or architecture string to a canonical architecture.
// GCP Cloud Functions/Cloud Run functions consumption tiers default to x86_64.
func MapGCPArchitecture(rawArch string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawArch))
	if canonical, ok := gcpArchMap[key]; ok {
		return canonical, nil
	}

	if strings.Contains(key, "arm") {
		return "", fmt.Errorf("%w: gcp does not support ARM architecture for serverless: %q", ErrUnmappedArchitecture, rawArch)
	}

	if strings.Contains(key, "function") || strings.Contains(key, "invocation") || strings.Contains(key, "time") || strings.Contains(key, "second") {
		return ArchX86_64, nil
	}

	return "", fmt.Errorf("%w: gcp architecture %q", ErrUnmappedArchitecture, rawArch)
}
