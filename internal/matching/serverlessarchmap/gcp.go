package serverlessarchmap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var gcpArchMap = map[string]string{
	"x86_64":               domain.ArchitectureX86_64,
	"x86":                  domain.ArchitectureX86_64,
	"x86-64":               domain.ArchitectureX86_64,
	"amd64":                domain.ArchitectureX86_64,
	"intel":                domain.ArchitectureX86_64,
	"cloud functions":      domain.ArchitectureX86_64,
	"cloud run functions":  domain.ArchitectureX86_64,
	"cloud run":            domain.ArchitectureX86_64,
	"1st_gen":              domain.ArchitectureX86_64,
	"2nd_gen":              domain.ArchitectureX86_64,
	"invocations":          domain.ArchitectureX86_64,
	"function invocations": domain.ArchitectureX86_64,
	"requests":             domain.ArchitectureX86_64,
	"request":              domain.ArchitectureX86_64,
	"execution time":       domain.ArchitectureX86_64,
	"memory time":          domain.ArchitectureX86_64,
	"cpu time":             domain.ArchitectureX86_64,
	"gb-seconds":           domain.ArchitectureX86_64,
	"ghz-seconds":          domain.ArchitectureX86_64,
	"gib-seconds":          domain.ArchitectureX86_64,
	"vcpu-seconds":         domain.ArchitectureX86_64,
	"cpu allocation":       domain.ArchitectureX86_64,
	"memory allocation":    domain.ArchitectureX86_64,
	"cpu":                  domain.ArchitectureX86_64,
	"memory":               domain.ArchitectureX86_64,
	"instance":             domain.ArchitectureX86_64,
	"instances":            domain.ArchitectureX86_64,
	"worker pool":          domain.ArchitectureX86_64,
	"worker pools":         domain.ArchitectureX86_64,
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

	if strings.Contains(key, "function") || strings.Contains(key, "invocation") || strings.Contains(key, "request") ||
		strings.Contains(key, "time") || strings.Contains(key, "second") || strings.Contains(key, "cloud run") ||
		strings.Contains(key, "allocation") || strings.Contains(key, "cpu") || strings.Contains(key, "memory") ||
		strings.Contains(key, "gib-second") || strings.Contains(key, "vcpu-second") || strings.Contains(key, "instance") ||
		strings.Contains(key, "job") || strings.Contains(key, "worker pool") {
		return domain.ArchitectureX86_64, nil
	}

	return "", fmt.Errorf("%w: gcp architecture %q", ErrUnmappedArchitecture, rawArch)
}
