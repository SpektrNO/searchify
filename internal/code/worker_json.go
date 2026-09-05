package code

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type workerOut struct {
	result Result
	err    string
}

func parseWorkerOut(raw []byte) (workerOut, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return workerOut{}, fmt.Errorf("empty codeparse worker output")
	}
	var envelope struct {
		Units   []Unit   `json:"units"`
		Symbols []Symbol `json:"symbols"`
		Refs    []Ref    `json:"refs"`
		Error   string   `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return workerOut{}, fmt.Errorf("decode codeparse JSON: %w", err)
	}
	return workerOut{
		result: Result{Units: envelope.Units, Symbols: envelope.Symbols, Refs: envelope.Refs},
		err:    envelope.Error,
	}, nil
}
