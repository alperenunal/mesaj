package mqtt

import (
	"fmt"
	"net"
)

func Tcp(acl Acl) error {
	listener, err := net.Listen("tcp", ":1883")
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}

			session, err := newSession(conn, acl)
			if err != nil {
				fmt.Println(err)
				return
			}

			go session.handle()
		}
	}()

	return nil
}
