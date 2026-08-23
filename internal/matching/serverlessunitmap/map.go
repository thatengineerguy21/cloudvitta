package serverlessunitmap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// ErrUnmappedUnit is returned when a raw unit or meter description cannot be mapped to a canonical serverless unit.
var ErrUnmappedUnit = errors.New("serverlessunitmap: unmapped unit")

// SupportedCanonicalUnits returns all supported canonical serverless units.
func SupportedCanonicalUnits() []string {
	return []string{
		domain.UnitPerMillionRequests,
		domain.UnitPerRequest,
		domain.UnitPer10Requests,
		domain.UnitPerGBSecond,
		domain.UnitPerGHzSecond,
		domain.UnitPerVCPUSecond,
		domain.UnitPerGiBSecond,
	}
}

// MapAWSUnit maps AWS Lambda unit/usage/group metadata to a canonical serverless unit.
func MapAWSUnit(rawUnit, componentType string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawUnit))
	switch componentType {
	case domain.ComponentTypeRequestFee:
		switch key {
		case "requests", "request", "reqs", "calls", "invocations", "lambda-reqs":
			return domain.UnitPerMillionRequests, nil
		}
		if strings.Contains(key, "request") || strings.Contains(key, "invocation") {
			return domain.UnitPerMillionRequests, nil
		}
	case domain.ComponentTypeDurationFee:
		switch key {
		case "seconds", "second", "s", "gb-second", "gb-seconds", "gb-s", "lambda-gb-second", "lambda-gb-second-arm":
			return domain.UnitPerGBSecond, nil
		}
		if strings.Contains(key, "second") || strings.Contains(key, "gb") {
			return domain.UnitPerGBSecond, nil
		}
	}
	return "", fmt.Errorf("%w: aws raw unit %q component %q", ErrUnmappedUnit, rawUnit, componentType)
}

// MapAzureUnit maps Azure Functions UnitOfMeasure and meter metadata to a canonical serverless unit.
func MapAzureUnit(rawUnit, componentType string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawUnit))
	switch componentType {
	case domain.ComponentTypeRequestFee:
		switch key {
		case "10", "10 executions", "10x executions", "10 executions/month", "executions":
			return domain.UnitPer10Requests, nil
		case "1m executions", "1 million executions", "1000000 executions":
			return domain.UnitPerMillionRequests, nil
		}
		if strings.Contains(key, "10") || strings.Contains(key, "execution") {
			return domain.UnitPer10Requests, nil
		}
	case domain.ComponentTypeDurationFee:
		switch key {
		case "1 gb second", "1 gb-s", "gb-s", "gb-second", "gb-seconds", "seconds", "second", "s":
			return domain.UnitPerGBSecond, nil
		}
		if strings.Contains(key, "gb") || strings.Contains(key, "second") {
			return domain.UnitPerGBSecond, nil
		}
	}
	return "", fmt.Errorf("%w: azure raw unit %q component %q", ErrUnmappedUnit, rawUnit, componentType)
}

// MapGCPUnit maps GCP Cloud Functions usage unit and component type to a canonical serverless unit.
func MapGCPUnit(rawUnit, componentType string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawUnit))
	switch componentType {
	case domain.ComponentTypeRequestFee:
		switch key {
		case "calls", "call", "invocations", "invocation", "requests", "request":
			return domain.UnitPerRequest, nil
		}
		if strings.Contains(key, "call") || strings.Contains(key, "invocation") || strings.Contains(key, "request") {
			return domain.UnitPerRequest, nil
		}
	case domain.ComponentTypeDurationFeeMemory, domain.ComponentTypeDurationFee:
		switch key {
		case "giby.s", "gby.s", "gb.s", "gib-second", "gb-second", "gb-seconds", "gib-seconds", "gb-s", "s", "seconds", "second":
			return domain.UnitPerGBSecond, nil
		}
		if strings.Contains(key, "gb") || strings.Contains(key, "gib") || key == "s" || strings.Contains(key, "second") {
			return domain.UnitPerGBSecond, nil
		}
	case domain.ComponentTypeDurationFeeCPU:
		switch key {
		case "ghz.s", "ghz-second", "ghz-seconds", "ghz-s":
			return domain.UnitPerGHzSecond, nil
		case "vcpu.s", "vcpu-second", "vcpu-seconds", "vcpu-s":
			return domain.UnitPerVCPUSecond, nil
		case "s", "seconds", "second":
			return domain.UnitPerGHzSecond, nil
		}
		if strings.Contains(key, "ghz") {
			return domain.UnitPerGHzSecond, nil
		}
		if strings.Contains(key, "vcpu") {
			return domain.UnitPerVCPUSecond, nil
		}
		if key == "s" || strings.Contains(key, "second") {
			return domain.UnitPerGHzSecond, nil
		}
	}
	return "", fmt.Errorf("%w: gcp raw unit %q component %q", ErrUnmappedUnit, rawUnit, componentType)
}

// NormalizeUnit resolves a provider-specific raw unit and component type to a canonical serverless unit.
func NormalizeUnit(provider, rawUnit, componentType string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSUnit(rawUnit, componentType)
	case "azure":
		return MapAzureUnit(rawUnit, componentType)
	case "gcp":
		return MapGCPUnit(rawUnit, componentType)
	default:
		return "", fmt.Errorf("serverlessunitmap: unmapped provider %q", provider)
	}
}
