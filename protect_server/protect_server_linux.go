package protect_server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"sync"
	"syscall"
)

type ProtectServer struct {
	listener *net.UnixListener
	done     chan struct{}
	wg       sync.WaitGroup
	verbose  bool
}

func (ps *ProtectServer) Close() error {
	close(ps.done)
	ps.listener.Close()
	ps.wg.Wait()
	return nil
}

func getOneFd(socket int) (int, error) {
	// Recvmsg.
	buf := make([]byte, syscall.CmsgSpace(4))
	_, _, _, _, err := syscall.Recvmsg(socket, nil, buf, 0)
	if err != nil {
		return 0, err
	}

	// Parse control msgs.
	var msgs []syscall.SocketControlMessage
	msgs, _ = syscall.ParseSocketControlMessage(buf)
	if len(msgs) != 1 {
		return 0, fmt.Errorf("invaild msgs count: %d", len(msgs))
	}

	var fds []int
	fds, _ = syscall.ParseUnixRights(&msgs[0])
	if len(fds) != 1 {
		return 0, fmt.Errorf("invaild fds count: %d", len(fds))
	}

	return fds[0], nil
}

// GetFdFromConn get net.Conn's file descriptor.
func GetFdFromConn(l net.Conn) int {
	netFD := reflect.Indirect(reflect.Indirect(reflect.ValueOf(l)).FieldByName("fd"))
	pfd := reflect.Indirect(netFD.FieldByName("pfd"))
	return int(pfd.FieldByName("Sysfd").Int())
}

// Now don't forget about Close() when working with this function.
func ServeProtect(path string, verbose bool, fwmark int, protectCtl func(fd int)) io.Closer {
	if verbose {
		log.Println("ServeProtect", path, fwmark)
	}

	os.Remove(path)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		log.Fatal(err)
	}
	os.Chmod(path, 0777)

	server := &ProtectServer{
		listener: l,
		done:     make(chan struct{}),
		verbose:  verbose,
	}

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()

		for {
			select {
			case <-server.done:
				if server.verbose {
					log.Println("protect server: shutting down")
				}
				return
			default:
				c, err := l.Accept()
				if err != nil {
					if server.verbose {
						log.Println("protect server accept:", err)
					}
					return
				}

				server.wg.Add(1)
				go func(conn net.Conn) {
					defer server.wg.Done()
					defer conn.Close()

					fd, err := getOneFd(GetFdFromConn(conn))
					if err != nil {
						if server.verbose {
							log.Println("protect server getOneFd:", err)
						}
						return
					}
					defer syscall.Close(fd)

					success := false
					if protectCtl == nil {
						success = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, fwmark) == nil
					}
					if success {
						c.Write([]byte{1})
					} else {
						c.Write([]byte{0})
					}
				}(c)
			}
		}
	}()

	return l
}
