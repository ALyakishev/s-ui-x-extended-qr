package core

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sagernet/sing-box/log"
)

func TestMTProxyExpectedEventsStayDebug(t *testing.T) {
	cases := []struct {
		name, tag   string
		level       log.Level
		args        []any
		wantVisible bool
	}{
		{"invalid authentication", "inbound/mtproxy[test]", log.LevelError, []any{"cannot read client hello", errors.New("incorrect client random")}, false},
		{"closed relay", "inbound/mtproxy[test]", log.LevelInfo, []any{"telegram -> client has been finished (written 6048 bytes): io: read/write on closed pipe"}, false},
		{"stream start", "inbound/mtproxy[test]", log.LevelInfo, []any{"Stream has been started"}, false},
		{"stream finish", "inbound/mtproxy[test]", log.LevelInfo, []any{"Stream has been finished"}, false},
		{"client finish", "inbound/mtproxy[test]", log.LevelInfo, []any{"client -> telegram has been finished"}, false},
		{"real error", "inbound/mtproxy[test]", log.LevelError, []any{"cannot connect to telegram", errors.New("connection refused")}, true},
		{"relay failure", "inbound/mtproxy[test]", log.LevelInfo, []any{"telegram -> client has been finished (written 0 bytes): connection reset by peer"}, true},
		{"other protocol", "inbound/vless[test]", log.LevelError, []any{"cannot read client hello", errors.New("incorrect client random")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			factory := NewDefaultFactory(context.Background(), log.Formatter{DisableTimestamp: true}, &output, "")
			defer factory.Close()
			factory.SetLevel(log.LevelInfo)
			factory.NewLogger(tc.tag).(*observableLogger).Log(context.Background(), tc.level, tc.args)
			if got := output.Len() > 0; got != tc.wantVisible {
				t.Fatalf("visible at info = %v, want %v: %s", got, tc.wantVisible, output.String())
			}
			output.Reset()
			factory.SetLevel(log.LevelDebug)
			factory.NewLogger(tc.tag).(*observableLogger).Log(context.Background(), tc.level, tc.args)
			if !tc.wantVisible && !bytes.Contains(output.Bytes(), []byte("DEBUG")) {
				t.Fatalf("expected debug diagnostic: %s", output.String())
			}
		})
	}
}
