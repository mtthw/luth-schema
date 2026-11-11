// Command check_parity answers tool/check-parity.mjs requests with the Go SDK,
// mirroring tool/check_parity.dart. Request values are JSON definitions, or
// JSON source strings for the decode action.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mtthw/luth-schema/exercise"
)

type request struct {
	Action string          `json:"action"`
	Value  json.RawMessage `json:"value"`
}

type result struct {
	OK     bool    `json:"ok"`
	Source *string `json:"source,omitempty"`
}

func main() {
	var requests []request
	if err := json.NewDecoder(os.Stdin).Decode(&requests); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	results := make([]result, len(requests))
	for i, r := range requests {
		results[i] = answer(r)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func answer(r request) result {
	var definition exercise.Definition
	var err error
	switch r.Action {
	case "exercise", "validate":
		definition, err = exercise.Decode(r.Value)
	case "decode":
		var source string
		if err = json.Unmarshal(r.Value, &source); err == nil {
			definition, err = exercise.Decode([]byte(source))
		}
	case "migrate":
		definition, err = exercise.Migrate(r.Value)
	default:
		fmt.Fprintln(os.Stderr, "unknown parity action", r.Action)
		os.Exit(1)
	}
	if err != nil {
		return result{}
	}
	if r.Action == "validate" {
		return result{OK: true}
	}
	source, err := exercise.Encode(definition)
	if err != nil {
		return result{}
	}
	text := string(source)
	return result{OK: true, Source: &text}
}
