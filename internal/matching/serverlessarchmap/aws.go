package serverlessarchmap

import (
	"fmt"
	"strings"
)

var awsArchMap = map[string]string{
	"x86_64":                    ArchX86_64,
	"x86":                       ArchX86_64,
	"x86-64":                    ArchX86_64,
	"amd64":                     ArchX86_64,
	"intel":                     ArchX86_64,
	"request":                   ArchX86_64,
	"requests":                  ArchX86_64,
	"aws-lambda-requests":       ArchX86_64,
	"lambda-gb-second":          ArchX86_64,
	"aws-lambda-duration":       ArchX86_64,
	"use2-request":              ArchX86_64,
	"use2-lambda-gb-second":     ArchX86_64,
	"arm64":                     ArchARM64,
	"arm":                       ArchARM64,
	"graviton":                  ArchARM64,
	"graviton2":                 ArchARM64,
	"request-arm":               ArchARM64,
	"requests-arm":              ArchARM64,
	"aws-lambda-requests-arm":   ArchARM64,
	"lambda-gb-second-arm":      ArchARM64,
	"aws-lambda-duration-arm":   ArchARM64,
	"use2-request-arm":          ArchARM64,
	"use2-lambda-gb-second-arm": ArchARM64,
}

// MapAWSArchitecture maps an AWS Lambda group, usage type, or architecture string to a canonical architecture.
func MapAWSArchitecture(rawArch string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawArch))
	if canonical, ok := awsArchMap[key]; ok {
		return canonical, nil
	}

	if strings.Contains(key, "arm") || strings.Contains(key, "graviton") {
		return ArchARM64, nil
	}
	if strings.Contains(key, "x86") || strings.Contains(key, "request") || strings.Contains(key, "duration") || strings.Contains(key, "gb-second") {
		return ArchX86_64, nil
	}

	return "", fmt.Errorf("%w: aws architecture %q", ErrUnmappedArchitecture, rawArch)
}
