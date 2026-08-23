package serverlessarchmap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var azureArchMap = map[string]string{
	"x86_64":                     domain.ArchitectureX86_64,
	"x86":                        domain.ArchitectureX86_64,
	"x86-64":                     domain.ArchitectureX86_64,
	"amd64":                      domain.ArchitectureX86_64,
	"intel":                      domain.ArchitectureX86_64,
	"standard":                   domain.ArchitectureX86_64,
	"consumption":                domain.ArchitectureX86_64,
	"standard total executions":  domain.ArchitectureX86_64,
	"standard execution time":    domain.ArchitectureX86_64,
	"total executions":           domain.ArchitectureX86_64,
	"execution time":             domain.ArchitectureX86_64,
	"flex consumption":           domain.ArchitectureX86_64,
	"flex_consumption":           domain.ArchitectureX86_64,
	"on demand":                  domain.ArchitectureX86_64,
	"on demand total executions": domain.ArchitectureX86_64,
	"on demand execution time":   domain.ArchitectureX86_64,
}

// MapAzureArchitecture maps an Azure Functions SKU name, meter name, or architecture string to a canonical architecture.
// Azure Functions consumption tiers only offer x86_64 execution.
func MapAzureArchitecture(rawArch string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawArch))
	if canonical, ok := azureArchMap[key]; ok {
		return canonical, nil
	}

	if strings.Contains(key, "arm") {
		return "", fmt.Errorf("%w: azure does not support ARM architecture for serverless: %q", ErrUnmappedArchitecture, rawArch)
	}

	if strings.Contains(key, "execution") || strings.Contains(key, "consumption") || strings.Contains(key, "functions") {
		return domain.ArchitectureX86_64, nil
	}

	return "", fmt.Errorf("%w: azure architecture %q", ErrUnmappedArchitecture, rawArch)
}
