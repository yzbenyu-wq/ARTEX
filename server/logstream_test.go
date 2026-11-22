package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLogStreamSince(t *testing.T) {
	const epoch = "abc-1"
	cases := []struct {
		name string
		le   string
		q    int64
		want int64
	}{
		{"no header uses query", "", 42, 42},
		{"same epoch resumes", epoch + "|100", 0, 100},
		{"same epoch beats query", epoch + "|7", 42, 7},
		{"foreign epoch ignored", "old-9|100", 5, 5},
		{"malformed seq ignored", epoch + "|x", 5, 5},
		{"negative seq ignored", epoch + "|-3", 5, 5},
		{"no separator ignored", epoch, 5, 5},
		{"whitespace trimmed", "  " + epoch + "|9  ", 0, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := logStreamSince(tc.le, tc.q, epoch); got != tc.want {
				t.Fatalf("logStreamSince(%q, %d) = %d, want %d", tc.le, tc.q, got, tc.want)
			}
		})
	}
}

func TestSSELogID(t *testing.T) {
	if got := sseLogID("e1", 9); got != "e1|9" {
		t.Fatalf("sseLogID = %q, want e1|9", got)
	}
}

// TestStreamLogsEmitsResumableIDs checks the wire format the browser needs to
// resume: every data frame is preceded by id: <epoch>|<seq> matching its Seq.
func TestStreamLogsEmitsResumableIDs(t *testing.T) {
	const marker = "stream-resumable-id-test"
	logSink.add(LogLine{Level: "info", Text: marker})
	wantSeq := logSink.seq

	s := &Server{}
	ts := httptest.NewServer(http.HandlerFunc(s.streamLogs))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/logs/stream?since=0", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	type frame struct {
		id   string
		line LogLine
	}
	frames := make(chan frame, 64)
	go func() {
		defer close(frames)
		var lastID string
		br := bufio.NewReader(resp.Body)
		for {
			raw, err := br.ReadString('\n')
			if err != nil {
				return
			}
			raw = strings.TrimRight(raw, "\r\n")
			if strings.HasPrefix(raw, "id: ") {
				lastID = strings.TrimPrefix(raw, "id: ")
				continue
			}
			if !strings.HasPrefix(raw, "data: ") {
				continue
			}
			var l LogLine
			if json.Unmarshal([]byte(strings.TrimPrefix(raw, "data: ")), &l) != nil {
				continue
			}
			frames <- frame{id: lastID, line: l}
		}
	}()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				t.Fatal("stream ended before the marker line")
			}
			if f.line.Text != marker {
				continue
			}
			if f.line.Seq != wantSeq {
				t.Fatalf("marker seq = %d, want %d", f.line.Seq, wantSeq)
			}
			wantID := sseLogID(logSink.epoch, wantSeq)
			if f.id != wantID {
				t.Fatalf("id = %q, want %q", f.id, wantID)
			}
			epoch, seqStr, ok := strings.Cut(f.id, "|")
			if !ok || epoch != logSink.epoch {
				t.Fatalf("id %q does not carry the process epoch", f.id)
			}
			if n, err := strconv.ParseInt(seqStr, 10, 64); err != nil || n != wantSeq {
				t.Fatalf("id %q does not carry seq %d", f.id, wantSeq)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for the marker line")
		}
	}
}
