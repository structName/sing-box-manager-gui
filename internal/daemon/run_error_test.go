package daemon

import (
	"errors"
	"net"
	"testing"
)

func TestExitCodeForRunErrorAddressInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen first socket: %v", err)
	}
	defer listener.Close()

	_, err = net.Listen("tcp", listener.Addr().String())
	if err == nil {
		t.Fatal("expected second listen to fail")
	}

	if got := ExitCodeForRunError(err); got != exitCodeAddressInUse {
		t.Fatalf("ExitCodeForRunError() = %d, want %d", got, exitCodeAddressInUse)
	}
}

func TestExitCodeForRunErrorGeneric(t *testing.T) {
	if got := ExitCodeForRunError(errors.New("boom")); got != 1 {
		t.Fatalf("ExitCodeForRunError() = %d, want 1", got)
	}
}
