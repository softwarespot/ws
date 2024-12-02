package room

import (
	"fmt"
	"testing"
	"time"

	testhelpers "github.com/softwarespot/ws/pkg/test-helpers"
)

func Test_NewRoom(t *testing.T) {
	cfg := NewRoomConfig[string]()
	cfg.CloseTimeout = 5 * time.Second

	r := New("root", cfg)

	u1 := NewUser[string]("u1")
	u2 := NewUser[string]("u2")

	testhelpers.AssertNoError(t, r.Register(u1))
	testhelpers.AssertNoError(t, r.Register(u2))

	go func() {
		for msg := range u1.Messages() {
			t.Log("user 1:", msg)
		}
		r.Unregister(u1)
		t.Log("user 1 unregistered")
	}()
	go func() {
		for msg := range u2.Messages() {
			t.Log("user 2:", msg)
		}
		r.Unregister(u2)
		t.Log("user 2 unregistered")
	}()

	testhelpers.AssertEqual(t, r.ID(), "root")

	go func() {
		i := 0
		for {
			if err := r.SendSync(u1, fmt.Sprintf("from user %s (%d)", u1.ID(), i)); err != nil {
				break
			}
			i += 1
		}
	}()

	testhelpers.AssertNoError(t, r.Broadcast("for ALL users"))

	time.Sleep(1 * time.Millisecond)

	testhelpers.AssertNoError(t, r.Close())
	testhelpers.AssertError(t, r.Broadcast(""))
	testhelpers.AssertError(t, r.SendSync(u1, ""))
	testhelpers.AssertEqual(t, r.Size(), 0)

	time.Sleep(1 * time.Millisecond)
}
