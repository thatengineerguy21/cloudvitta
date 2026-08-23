package serverlessarchmap

import (
	"fmt"
	"strings"
)

var azureArchMap = map[string]string{
	"x86_64":                     ArchX86_64,
	"x86":                        ArchX86_64,
	"x86-64":                     ArchX86_64,
	"amd64":                      ArchX86_64,
	"intel":                      ArchX86_64,
	"standard":                   ArchX86_64,
	"consumption":                ArchX86_64,
	"standard total executions":  ArchX86_64,
	"standard execution time":    ArchX86_64,
	"total executions":           ArchX86_64,
	"execution time":             ArchX86_64,
	"flex consumption":           ArchX86_64,
	"flex_consumption":           ArchX86_64,
	"on demand":                  ArchX86_64,
	"on demand total executions": ArchX86_64,
	"on demand execution time":   ArchX86_64,
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
		return ArchX86_64, nil
	}

	return "", fmt.Errorf("%w: azure architecture %q", ErrUnmappedArchitecture, rawArch)
}
