package tokenizer

import (
	"encoding/json"
	"errors"
)

type Tokenizer struct {
	vocab map[string]int
}

func NewTokenizer(data []byte) (*Tokenizer, error) {
	var vocab map[string]int
	if err := json.Unmarshal(data, &vocab); err != nil {
		return nil, err
	}
	return &Tokenizer{vocab: vocab}, nil
}

func (t *Tokenizer) Encode(text string) ([]float32, error) {
	if t.vocab == nil {
		return nil, errors.New("tokenizer not initialized")
	}
	var tokens []float32
	for _, r := range text {
		if id, ok := t.vocab[string(r)]; ok {
			tokens = append(tokens, float32(id))
		}
	}
	if len(tokens) == 0 {
		tokens = []float32{1} // default token
	}
	return tokens, nil
}