package radix

import (
	"fmt"
	"net"
	"sync"
	. "testing"
	"time"

	"github.com/mediocregopher/radix.v2/resp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Watching the watchmen

func testStub() Conn {
	m := map[string]string{}
	return Stub("tcp", "127.0.0.1:6379", func(args []string) interface{} {
		switch args[0] {
		case "GET":
			return m[args[1]]
		case "SET":
			m[args[1]] = args[2]
			return nil
		case "ECHO":
			return args[1]
		default:
			return fmt.Errorf("testStub doesn't support command %q", args[0])
		}
	})
}

func TestStub(t *T) {
	stub := testStub()

	{ // Basic test
		var foo string
		require.Nil(t, Cmd("SET", "foo", "a").Run(stub))
		require.Nil(t, Cmd("GET", "foo").Into(&foo).Run(stub))
		assert.Equal(t, "a", foo)
	}

	{ // Basic test with an int, to ensure marshalling/unmarshalling all works
		var foo int
		require.Nil(t, Cmd("SET", "foo", 1).Run(stub))
		require.Nil(t, Cmd("GET", "foo").Into(&foo).Run(stub))
		assert.Equal(t, 1, foo)
	}
}

func TestStubLockingTimeout(t *T) {
	stub := testStub()
	wg := new(sync.WaitGroup)
	c := 1000

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < c; i++ {
			require.Nil(t, stub.Encode(CmdNoKey("ECHO", i)))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < c; i++ {
			var j int
			require.Nil(t, stub.Decode(resp.Any{I: &j}))
			assert.Equal(t, i, j)
		}
	}()

	wg.Wait()

	// test out timeout. do a write-then-read to ensure nothing bad happens in
	// when there's actually data to read
	now := time.Now()
	stub.SetDeadline(now.Add(2 * time.Second))
	require.Nil(t, stub.Encode(CmdNoKey("ECHO", 1)))
	require.Nil(t, stub.Decode(resp.Any{}))

	// now there's no data to read, should return after 2-ish seconds with a
	// timeout error
	err := stub.Decode(resp.Any{})
	nerr, ok := err.(*net.OpError)
	assert.True(t, ok)
	assert.True(t, nerr.Timeout())
}