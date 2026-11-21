package agent

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

type autoTitleProvider struct {
	reply string
	err   error
	req   llm.CompletionRequest
}

func (p *autoTitleProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.req = req
	if p.err != nil {
		return llm.Message{}, "", llm.Usage{}, p.err
	}
	return llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(p.reply)}}, "stop", llm.Usage{}, nil
}

func (p *autoTitleProvider) Stream(context.Context, llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(func(llm.StreamEvent, error) bool) {}
}

func TestGenerateAutoTitleCleansResponse(t *testing.T) {
	long := strings.Repeat("很长", 30)
	p := &autoTitleProvider{reply: "\n\"标题：" + long + "\"\n第二行"}
	got, err := GenerateAutoTitle(t.Context(), p, "用户希望整理项目设置")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, "\r\n") || strings.Contains(got, "标题：") || strings.HasPrefix(got, "\"") || len([]rune(got)) > 40 {
		t.Fatalf("unclean title %q", got)
	}
	if p.req.Thinking != "disabled" || p.req.MaxTokens != autoTitleMaxTokens || len(p.req.Tools) != 0 {
		t.Fatalf("request options: %+v", p.req)
	}

	quoted := &autoTitleProvider{reply: `标题："整理项目设置"`}
	got, err = GenerateAutoTitle(t.Context(), quoted, "整理项目设置")
	if err != nil {
		t.Fatal(err)
	}
	if got != "整理项目设置" {
		t.Fatalf("nested quote/prefix cleanup: got %q", got)
	}
}

func TestGenerateAutoTitleProviderError(t *testing.T) {
	_, err := GenerateAutoTitle(t.Context(), &autoTitleProvider{err: errors.New("boom")}, "内容")
	if err == nil {
		t.Fatal("expected provider error")
	}
}

// scriptedAutoTitleProvider returns one scripted (text, stop) per call and records
// the MaxTokens of each request, so tests can assert the truncation escalation.
type scriptedAutoTitleProvider struct {
	replies []string
	stops   []string
	tokens  []int
}

func (p *scriptedAutoTitleProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.tokens = append(p.tokens, req.MaxTokens)
	i := len(p.tokens) - 1
	if i >= len(p.replies) {
		i = len(p.replies) - 1
	}
	msg := llm.Message{Role: llm.RoleAssistant}
	if p.replies[i] != "" {
		msg.Content = []llm.ContentBlock{llm.TextBlock(p.replies[i])}
	}
	return msg, p.stops[i], llm.Usage{}, nil
}

func (p *scriptedAutoTitleProvider) Stream(context.Context, llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(func(llm.StreamEvent, error) bool) {}
}

// 思维链模型可能把预算全花在推理上，finish_reason=max_tokens 而正文为空。
// 这时应加大上限重试一次，而不是直接失败。
func TestGenerateAutoTitleRetriesTruncatedResponse(t *testing.T) {
	p := &scriptedAutoTitleProvider{
		replies: []string{"", "SQL 注入测试"},
		stops:   []string{"max_tokens", "end_turn"},
	}
	got, err := GenerateAutoTitle(t.Context(), p, "帮我测试 SQL 注入")
	if err != nil {
		t.Fatal(err)
	}
	if got != "SQL 注入测试" {
		t.Fatalf("title=%q", got)
	}
	want := []int{autoTitleMaxTokens, autoTitleRetryMaxTokens}
	if len(p.tokens) != len(want) || p.tokens[0] != want[0] || p.tokens[1] != want[1] {
		t.Fatalf("max tokens per call=%v, want %v", p.tokens, want)
	}
}

func TestGenerateAutoTitleTruncatedTwiceFails(t *testing.T) {
	p := &scriptedAutoTitleProvider{
		replies: []string{"", ""},
		stops:   []string{"max_tokens", "max_tokens"},
	}
	if _, err := GenerateAutoTitle(t.Context(), p, "内容"); err == nil {
		t.Fatal("expected error when both attempts are truncated")
	}
	if len(p.tokens) != 2 {
		t.Fatalf("calls=%d want 2 (only truncation may retry)", len(p.tokens))
	}
}

// 正常结束的空响应不重试，避免把一次空响应放大成两次调用。
func TestGenerateAutoTitleEmptyNormalStopDoesNotRetry(t *testing.T) {
	p := &scriptedAutoTitleProvider{
		replies: []string{""},
		stops:   []string{"end_turn"},
	}
	if _, err := GenerateAutoTitle(t.Context(), p, "内容"); err == nil {
		t.Fatal("expected error for empty completed response")
	}
	if len(p.tokens) != 1 {
		t.Fatalf("calls=%d want 1", len(p.tokens))
	}
}
