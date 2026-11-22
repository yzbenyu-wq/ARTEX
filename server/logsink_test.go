package server

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestSink(capacity int) *logSinkT {
	return &logSinkT{cap: capacity, epoch: "test-epoch", subs: map[chan LogLine]struct{}{}, out: io.Discard}
}

func TestLogSinkAddStampsSeqAndEpoch(t *testing.T) {
	s := newTestSink(8)
	s.add(LogLine{Text: "first"})
	s.add(LogLine{Text: "second"})

	lines, cursor := s.recent(0, 10)
	if cursor != 2 {
		t.Fatalf("cursor = %d, want 2", cursor)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for i, l := range lines {
		if l.Seq != int64(i+1) {
			t.Errorf("lines[%d].Seq = %d, want %d", i, l.Seq, i+1)
		}
		if l.Epoch != "test-epoch" {
			t.Errorf("lines[%d].Epoch = %q, want test-epoch", i, l.Epoch)
		}
	}
}

func TestLogSinkRecentSinceAndCap(t *testing.T) {
	s := newTestSink(3)
	for i := 1; i <= 5; i++ {
		s.add(LogLine{Text: fmt.Sprintf("line-%d", i)})
	}

	lines, cursor := s.recent(0, 100) // limit > cap is clamped to cap
	if cursor != 5 {
		t.Fatalf("cursor = %d, want 5", cursor)
	}
	if len(lines) != 3 || lines[0].Seq != 3 || lines[2].Seq != 5 {
		t.Fatalf("ring not trimmed to the last 3 lines: %+v", lines)
	}

	lines, _ = s.recent(4, 100)
	if len(lines) != 1 || lines[0].Seq != 5 {
		t.Fatalf("since=4 should return only seq 5: %+v", lines)
	}

	lines, _ = s.recent(5, 100)
	if len(lines) != 0 {
		t.Fatalf("since=cursor should return nothing: %+v", lines)
	}
}

func TestLogSinkSubscriberLifecycle(t *testing.T) {
	s := newTestSink(8)
	ch, unsub := s.subscribe()
	s.add(LogLine{Level: "warn", Tag: "pg", Text: "hello"})

	select {
	case got := <-ch:
		if got.Seq != 1 || got.Epoch != "test-epoch" || got.Tag != "pg" || got.Text != "hello" {
			t.Fatalf("unexpected line: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber received nothing")
	}

	unsub()
	unsub() // idempotent
	if _, ok := <-ch; ok {
		t.Fatal("channel still open after unsub")
	}
	s.add(LogLine{Text: "after unsub"}) // must not send on the closed channel
}

func TestLogSinkAddDoesNotBlockOnSlowSubscriber(t *testing.T) {
	s := newTestSink(8)
	_, unsub := s.subscribe()
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			s.add(LogLine{Text: "flood"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("add blocked on a slow subscriber")
	}
}

// TestLogSinkSubscribeUnsubscribeRace stresses add() against subscribe/unsub.
// Before fan-out moved under the sink mutex this interleaving panicked with
// "send on closed channel" and took the whole process down.
func TestLogSinkSubscribeUnsubscribeRace(t *testing.T) {
	s := newTestSink(256)
	stop := make(chan struct{})
	var writers sync.WaitGroup
	for i := 0; i < 8; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					s.add(LogLine{Level: "info", Text: "race"})
				}
			}
		}()
	}

	for i := 0; i < 500; i++ {
		ch, unsub := s.subscribe()
		done := make(chan struct{})
		go func() {
			defer close(done)
			deadline := time.NewTimer(200 * time.Millisecond)
			defer deadline.Stop()
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-deadline.C:
					return
				}
			}
		}()
		unsub()
		<-done
	}

	close(stop)
	writers.Wait()
}

func TestParseLog(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantTag   string
		wantLevel string
	}{
		{"stdlib prefix and tag", "2026/11/22 10:00:00 [pg] connected", "pg", "info"},
		{"error keyword", "2026/11/22 10:00:00 [engine] dial tcp: connection refused: error boom", "engine", "error"},
		{"chinese failure", "[work#1] 第 3 轮 失败", "work#1", "error"},
		{"warn keyword", "[guard] out of scope, skip target", "guard", "warn"},
		{"no tag", "backend listening on :8787", "", "info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLog(tc.in)
			if got.Tag != tc.wantTag {
				t.Errorf("Tag = %q, want %q", got.Tag, tc.wantTag)
			}
			if got.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", got.Level, tc.wantLevel)
			}
			if got.TS == "" {
				t.Error("TS is empty")
			}
			if strings.HasPrefix(got.Text, "2026/") {
				t.Errorf("stdlib timestamp prefix not stripped: %q", got.Text)
			}
		})
	}
}

func TestLevelOf(t *testing.T) {
	cases := map[string]string{
		"fatal: boom":   "error",
		"panic: x":      "error",
		"err: bad":      "error",
		"写入失败":          "error",
		"⚠ retry later": "warn",
		"skip target":   "warn",
		"全部完成":          "info",
	}
	for in, want := range cases {
		if got := levelOf(in); got != want {
			t.Errorf("levelOf(%q) = %q, want %q", in, got, want)
		}
	}
}
