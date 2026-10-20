package core

import (
	"context"
	"sync"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
)

func TestRoutedFlowAccountsBothDirections(t *testing.T) {
	tracker := NewStatsTracker()
	flow := tracker.RoutedFlow(context.Background(), adapter.InboundContext{Inbound: "in", User: "user"}, nil, fakeStatsOutbound{tag: "out"})
	flow.AttachFlow(&testFlowHandle{tracker: flow})
	flow.CountForward(123)
	flow.CountReverse(456)
	flow.CloseFlow(tun.FlowCloseFinished)
	flow.CloseFlow(tun.FlowCloseReset)
	if active := tracker.inflight.Active(); active != 0 {
		t.Fatalf("closed flow retains %d active writers", active)
	}
	stats := *tracker.GetStats()
	if len(stats) != 6 {
		t.Fatalf("expected upload/download for inbound, outbound and user: %+v", stats)
	}
	for _, stat := range stats {
		want := int64(456)
		if stat.Direction {
			want = 123
		}
		if stat.Traffic != want {
			t.Errorf("%s/%s upload=%v traffic=%d, want %d", stat.Resource, stat.Tag, stat.Direction, stat.Traffic, want)
		}
	}
}

func TestRoutedFlowClosesWithoutTrackerDeadlock(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(map[bool]string{false: "inbound", true: "reset"}[reset], func(t *testing.T) {
			tracker := NewConnTracker()
			flow := tracker.RoutedFlow(context.Background(), adapter.InboundContext{Inbound: "in", Network: "udp"}, nil, nil)
			handle := &testFlowHandle{tracker: flow}
			flow.AttachFlow(handle)
			done := make(chan struct{})
			go func() {
				if reset {
					tracker.Reset()
				} else if closed := tracker.CloseConnByInbound("in"); closed != 1 {
					t.Errorf("closed %d flows, want 1", closed)
				}
				close(done)
			}()
			waitForTestChannel(t, done, "closing a flow deadlocked its tracker callback")
			if !handle.closed || len(tracker.connections) != 0 || tracker.inflight.Active() != 0 {
				t.Fatalf("flow was not fully closed: handle=%v connections=%d writers=%d", handle.closed, len(tracker.connections), tracker.inflight.Active())
			}
		})
	}
}

func TestRoutedFlowClosedBeforeAttachStaysUntracked(t *testing.T) {
	tracker := NewConnTracker()
	flow := tracker.RoutedFlow(context.Background(), adapter.InboundContext{Inbound: "in"}, nil, nil)
	flow.CloseFlow(tun.FlowCloseReset)
	flow.AttachFlow(&testFlowHandle{tracker: flow})
	if len(tracker.connections) != 0 || tracker.inflight.Active() != 0 {
		t.Fatal("a rejected flow was registered after closure")
	}
}

type testFlowHandle struct {
	tracker tun.FlowTracker
	once    sync.Once
	closed  bool
}

func (h *testFlowHandle) CloseFlow() {
	h.once.Do(func() {
		h.closed = true
		h.tracker.CloseFlow(tun.FlowCloseReset)
	})
}
