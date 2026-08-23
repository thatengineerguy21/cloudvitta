package serverlessarchmap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var awsArchMap = map[string]string{
	"x86_64":                    domain.ArchitectureX86_64,
	"x86":                       domain.ArchitectureX86_64,
	"x86-64":                    domain.ArchitectureX86_64,
	"amd64":                     domain.ArchitectureX86_64,
	"intel":                     domain.ArchitectureX86_64,
	"request":                   domain.ArchitectureX86_64,
	"requests":                  domain.ArchitectureX86_64,
	"aws-lambda-requests":       domain.ArchitectureX86_64,
	"lambda-gb-second":          domain.ArchitectureX86_64,
	"aws-lambda-duration":       domain.ArchitectureX86_64,
	"use2-request":              domain.ArchitectureX86_64,
	"use2-lambda-gb-second":     domain.ArchitectureX86_64,
	"arm64":                     domain.ArchitectureARM64,
	"arm":                       domain.ArchitectureARM64,
	"graviton":                  domain.ArchitectureARM64,
	"graviton2":                 domain.ArchitectureARM64,
	"request-arm":               domain.ArchitectureARM64,
	"requests-arm":              domain.ArchitectureARM64,
	"aws-lambda-requests-arm":   domain.ArchitectureARM64,
	"lambda-gb-second-arm":      domain.ArchitectureARM64,
	"aws-lambda-duration-arm":   domain.ArchitectureARM64,
	"use2-request-arm":          domain.ArchitectureARM64,
	"use2-lambda-gb-second-arm": domain.ArchitectureARM64,
}

// MapAWSArchitecture maps an AWS Lambda group, usage type, or architecture string to a canonical architecture.
func MapAWSArchitecture(rawArch string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawArch))
	if canonical, ok := awsArchMap[key]; ok {
		return canonical, nil
	}

	if strings.Contains(key, "arm") || strings.Contains(key, "graviton") {
		return domain.ArchitectureARM64, nil
	}
	if strings.Contains(key, "x86") || strings.Contains(key, "request") || strings.Contains(key, "duration") || strings.Contains(key, "gb-second") {
		return domain.ArchitectureX86_64, nil
	}

	return "", fmt.Errorf("%w: aws architecture %q", ErrUnmappedArchitecture, rawArch)
}
