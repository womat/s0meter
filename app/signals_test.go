package app

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"
)

// newSignalApp returns an App without meters whose signal handler listens on the returned
// channel, buffered like the one cmd subscribes.
func newSignalApp(t *testing.T, checkReload func() error) (*App, chan os.Signal) {
	t.Helper()
	cfg := NewConfig()
	cfg.DataFile = filepath.Join(t.TempDir(), "s0meter.yaml")
	signals := make(chan os.Signal, 1)
	app := New(cfg, signals, checkReload)
	app.HandleOSSignals()
	return app, signals
}

// SIGHUP restarts the App once the config file has passed the check.
func TestSIGHUPRestarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		checked := false
		app, signals := newSignalApp(t, func() error { checked = true; return nil })

		signals <- syscall.SIGHUP
		<-app.Restart()

		if !checked {
			t.Error("the config was not checked before the restart")
		}
		if app.ctx.Err() == nil {
			t.Error("the App context was not cancelled by the restart")
		}
	})
}

// A SIGHUP whose config is rejected leaves the App running, and it still reacts to the next
// signal.
func TestRejectedReloadKeepsRunning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		app, signals := newSignalApp(t, func() error { return errors.New("broken config") })

		signals <- syscall.SIGHUP
		synctest.Wait()
		if app.ctx.Err() != nil {
			t.Fatal("a rejected reload stopped the App")
		}
		select {
		case <-app.Restart():
			t.Fatal("a rejected reload restarted the App")
		default:
		}

		signals <- syscall.SIGTERM
		<-app.Shutdown()
	})
}

// SIGTERM and SIGINT stop the App; the shutdown channel is closed afterwards, so every later
// receive returns at once.
func TestStopSignals(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				app, signals := newSignalApp(t, nil)

				signals <- sig
				<-app.Shutdown()
				if _, open := <-app.Shutdown(); open {
					t.Error("shutdown channel is still open after the stop")
				}
				if app.ctx.Err() == nil {
					t.Error("the App context was not cancelled by the stop")
				}
			})
		})
	}
}

// Once its context is cancelled, the handler of an App leaves the next signal in the channel
// for the App that replaces it.
func TestCancelledHandlerLeavesSignalForNextApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		app, signals := newSignalApp(t, nil)

		app.cancelFunc()
		synctest.Wait()

		signals <- syscall.SIGTERM
		synctest.Wait()
		if len(signals) != 1 {
			t.Error("the handler of a cancelled App consumed the signal")
		}
	})
}

// A signal that waits in the channel while a start fails stays there for the next App: the
// failed App must not handle it, because its abort would run alongside the shutdown the
// signal starts (F1).
func TestFailedStartLeavesSignalForNextApp(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()

	dir := t.TempDir()
	cfg := NewConfig()
	cfg.Env = DevEnv
	cfg.DataFile = filepath.Join(dir, "s0meter.yaml")
	cfg.Webserver.ListenHost = "127.0.0.1"
	cfg.Webserver.ListenPort = busy.Addr().(*net.TCPAddr).Port
	cfg.Webserver.CertFile = filepath.Join(dir, "missing.pem")

	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	if _, err := New(cfg, signals, nil).Run(); err == nil {
		t.Fatal("Run() succeeded on a port in use")
	}

	select {
	case s := <-signals:
		if s != syscall.SIGTERM {
			t.Errorf("signal = %v, want SIGTERM", s)
		}
	default:
		t.Error("the failed App took the signal away from the next one")
	}
}
