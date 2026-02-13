package services

import (
	"fmt"
	"imageapp/internal/models"
	"math"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type EmbeddingService struct {
	mu            sync.Mutex
	session       *ort.AdvancedSession
	tokenizer     *models.Tokenizer
	inputIDs      *ort.Tensor[int64]
	attentionMask *ort.Tensor[int64]
	tokenTypeIDs  *ort.Tensor[int64]
	output        *ort.Tensor[float32]
	once          sync.Once
}

func NewEmbeddingService(modelPath, tokenizerPath string) (*EmbeddingService, error) {
	ort.SetSharedLibraryPath("./model/libonnxruntime.so")

	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("init onnx: %w", err)
	}

	inputShape := ort.NewShape(1, 128)
	attShape := ort.NewShape(1, 128)
	tokenTypeShape := ort.NewShape(1, 128)
	outputShape := ort.NewShape(1, 128, 384)

	inputIDs, err := ort.NewTensor(inputShape, make([]int64, 128))
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}

	attentionMask, err := ort.NewTensor(attShape, make([]int64, 128))
	if err != nil {
		return nil, fmt.Errorf("create attention tensor: %w", err)
	}

	tokenTypeIDs, err := ort.NewTensor(tokenTypeShape, make([]int64, 128))
	if err != nil {
		return nil, fmt.Errorf("create token type tensor: %w", err)
	}

	output, err := ort.NewTensor(outputShape, make([]float32, 128*384))
	if err != nil {
		return nil, fmt.Errorf("create output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		[]ort.ArbitraryTensor{inputIDs, attentionMask, tokenTypeIDs},
		[]ort.ArbitraryTensor{output},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	tokenizer, err := models.NewTokenizer(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	return &EmbeddingService{
		session:       session,
		tokenizer:     tokenizer,
		inputIDs:      inputIDs,
		attentionMask: attentionMask,
		tokenTypeIDs:  tokenTypeIDs,
		output:        output,
	}, nil
}

func (e *EmbeddingService) EmbedTags(tags ...string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	text := strings.Join(tags, " ")

	inputIDs, attentionMask, err := e.tokenizer.Encode(text, 128)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	copy(e.inputIDs.GetData(), inputIDs)
	copy(e.attentionMask.GetData(), attentionMask)

	if err := e.session.Run(); err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	embedding := meanPooling(e.output.GetData(), attentionMask, 128, 384)
	normalize(embedding)

	return embedding, nil
}

func meanPooling(output []float32, mask []int64, seqLen, dim int) []float32 {
	embedding := make([]float32, dim)
	count := float32(0)

	for i := 0; i < seqLen; i++ {
		if mask[i] == 0 {
			continue
		}
		count++
		for j := 0; j < dim; j++ {
			embedding[j] += output[i*dim+j]
		}
	}

	for j := range embedding {
		embedding[j] /= count
	}

	return embedding
}

func normalize(v []float32) {
	var sum float64
	for _, val := range v {
		sum += float64(val * val)
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}

func (e *EmbeddingService) Close() {
	e.once.Do(func() {
		e.session.Destroy()
		e.inputIDs.Destroy()
		e.attentionMask.Destroy()
		e.tokenTypeIDs.Destroy()
		e.output.Destroy()
		ort.DestroyEnvironment()
	})
}
