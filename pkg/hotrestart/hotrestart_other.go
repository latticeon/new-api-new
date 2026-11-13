//go:build !linux

package hotrestart

import (
	"errors"
	"net"
)

const Supported = false

func Supervise(Options) int                    { return 1 }
func PrepareWorker()                           {}
func Listen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }
func NotifyReady() error                       { return nil }
func NotifyAcceptStopped()                     {}
func StopAccepting()                           {}
func RequestRestart() error                    { return errors.New("Hot restart is only supported on Linux") }
func ParentGone() <-chan struct{}              { return nil }
func Drain() <-chan struct{}                   { return nil }
func Quiesce()                                 { Background.DisableAndWait() }

type ReadyListener struct{ net.Listener }
