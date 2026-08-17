package provider

import (
	"encoding/json"
)

// SkipJSONValue consumes and discards a single JSON token or delimited JSON object/array
// from the decoder stream without allocating memory.
func SkipJSONValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	_, isDelim := t.(json.Delim)
	if !isDelim {
		return nil
	}

	depth := 1
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}
