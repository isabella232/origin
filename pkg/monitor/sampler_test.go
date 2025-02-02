package monitor

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestStartSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewMonitorWithInterval(5 * time.Millisecond)

	doneCh := make(chan struct{})
	var count int
	m.AddSampler(StartSampling(ctx, m, 21*time.Millisecond, func(previous bool) (condition *monitorapi.Condition, next bool) {
		defer func() { count++ }()
		switch {
		case count <= 5:
			return nil, true
		case count == 6:
			return &monitorapi.Condition{Level: monitorapi.Error, Locator: "tester", Message: "dying"}, false
		case count == 7:
			return &monitorapi.Condition{Level: monitorapi.Info, Locator: "tester", Message: "recovering"}, true
		case count <= 12:
			return nil, true
		case count == 13:
			return &monitorapi.Condition{Level: monitorapi.Error, Locator: "tester", Message: "dying 2"}, false
		case count <= 16:
			return nil, false
		case count == 17:
			return &monitorapi.Condition{Level: monitorapi.Info, Locator: "tester", Message: "recovering 2"}, true
		case count <= 20:
			return nil, true
		default:
			doneCh <- struct{}{}
			return nil, true
		}
	}).ConditionWhenFailing(&monitorapi.Condition{
		Level:   monitorapi.Error,
		Locator: "tester",
		Message: "down",
	}))

	m.StartSampling(ctx)
	<-doneCh
	cancel()

	var describe []string
	var log []string
	events := m.EventIntervals(time.Time{}, time.Time{})
	for _, interval := range events {
		i := interval.To.Sub(interval.From)
		describe = append(describe, fmt.Sprintf("%v %s", interval.Condition, i))
		log = append(log, fmt.Sprintf("%v", interval.Condition))
	}

	expected := []string{
		fmt.Sprintf("{Error tester dying}"),
		fmt.Sprintf("{Error tester down}"),
		fmt.Sprintf("{Info tester recovering}"),
		fmt.Sprintf("{Error tester dying 2}"),
		fmt.Sprintf("{Error tester down}"),
		fmt.Sprintf("{Info tester recovering 2}"),
	}
	if !reflect.DeepEqual(log, expected) {
		t.Fatalf("%s", diff.ObjectReflectDiff(log, expected))
	}
	if events[4].To.Sub(events[4].From) < 2*events[1].To.Sub(events[1].From) {
		t.Fatalf("last condition should be at least 2x first condition length:\n%s", strings.Join(describe, "\n"))
	} else {
		t.Logf("%s", strings.Join(describe, "\n"))
	}
}
