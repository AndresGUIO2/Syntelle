package main

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestCloudLifecycleReleasesAfterFinalSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		closed := false
		l := newCloudLifecycle(true, func() { closed = true }, 90*time.Second, 5*time.Second)
		defer l.close()
		l.observe("create", "one")
		l.observe("create", "two")
		l.observe("stop", "one")
		time.Sleep(100 * time.Second)
		synctest.Wait()
		if closed {
			t.Fatal("live second session was stopped")
		}
		l.observe("stop", "two")
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if !closed {
			t.Fatal("idle worker retained GPU")
		}
	})
}
func TestCloudLifecycleStartupAndLocalMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cloud, local := false, false
		c := newCloudLifecycle(true, func() { cloud = true }, 90*time.Second, 5*time.Second)
		defer c.close()
		l := newCloudLifecycle(false, func() { local = true }, 90*time.Second, 5*time.Second)
		defer l.close()
		time.Sleep(91 * time.Second)
		synctest.Wait()
		if !cloud || local {
			t.Fatal("startup idle/local lifecycle regression")
		}
	})
}
