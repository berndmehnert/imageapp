package models

import (
	"github.com/daulet/tokenizers"
)

type Tokenizer struct {
	tk *tokenizers.Tokenizer
}

func NewTokenizer(path string) (*Tokenizer, error) {
	tk, err := tokenizers.FromFile(path)
	if err != nil {
		return nil, err
	}
	return &Tokenizer{tk: tk}, nil
}

func (t *Tokenizer) Encode(text string, maxLen int) ([]int64, []int64, error) {
	ids, _ := t.tk.Encode(text, true)

	inputIDs := make([]int64, maxLen)
	mask := make([]int64, maxLen)

	for i := 0; i < len(ids) && i < maxLen; i++ {
		inputIDs[i] = int64(ids[i])
		mask[i] = 1
	}

	return inputIDs, mask, nil
}
