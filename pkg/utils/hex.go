package utils

import (
	"fmt"
	"shinzo/version1/pkg/errors"
	"strconv"
)

// NumberToHex converts any numeric type to a hex string with "0x" prefix
// Supports int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64
func NumberToHex[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](num T) string {
	return fmt.Sprintf("0x%x", num)
}

// HexToInt converts a hex string (with or without 0x prefix) to an int64
func HexToInt(s string) (int64, error) {
	if s == "" {
		return 0, errors.NewInvalidHex("utils", "HexToInt", s, nil)
	}
	// strconv.ParseInt handles "0x" prefix automatically when base=0
	result, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, errors.NewInvalidHex("utils", "HexToInt", s, err)
	}
	return result, nil
}

// IntToString converts any integer type to a decimal string
func IntToString[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](num T) string {
	return fmt.Sprintf("%d", num)
}

// StringToInt converts a decimal string to an int64
func StringToInt(s string) (int64, error) {
	if s == "" {
		return 0, errors.NewParsingFailed("utils", "StringToInt", "empty string", nil)
	}
	result, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errors.NewParsingFailed("utils", "StringToInt", "decimal number", err)
	}
	return result, nil
}
