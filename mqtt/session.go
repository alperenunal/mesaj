package mqtt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
)

type Session struct {
	Conn   net.Conn
	Reader *bufio.Reader
	Writer io.Writer

	Handler       handler
	Acl           Acl
	Subscriptions []*topicLevel

	SendQueue    any
	ReceiveQueue any

	CleanSession bool
	WillFlag     bool
	WillRetained bool
	ClientId     string
	Username     string
	Password     string
	WillTopic    string
	WillMsg      string
	WillQos      byte
	KeepAlive    uint16
}

func newSession(conn net.Conn, acl Acl) (*Session, error) {
	reader := bufio.NewReader(conn)
	b1, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if b1 != 0x10 {
		return nil, err
	}

	rem, err := decodeRemainingLength(reader)
	if err != nil {
		return nil, err
	}

	body := make([]byte, rem)

	_, err = io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(body[:6], []byte{0, 4, 'M', 'Q', 'T', 'T'}) {
		return nil, err
	}

	level := body[6]
	handler, err := newHandler(level)
	if err != nil {
		return nil, err
	}

	packet, err := handler.RecvConnect(body[7:])
	if err != nil {
		return nil, err
	}

	if err = acl.AllowConnect(packet); err != nil {
		return nil, err
	}

	session := &Session{
		Conn:   conn,
		Reader: reader,
		Writer: conn,

		Handler:       handler,
		Acl:           acl,
		Subscriptions: []*topicLevel{},

		CleanSession: packet.CleanSession,
		WillFlag:     packet.WillFlag,
		WillRetained: packet.WillRetain,
		ClientId:     packet.ClientId,
		Username:     packet.Username,
		Password:     packet.Password,
		WillTopic:    packet.WillTopic,
		WillMsg:      packet.WillMessage,
		WillQos:      packet.WillQoS,
		KeepAlive:    packet.KeepAlive,
	}

	handler.SendConnack(session.Writer, &ConnackPacket{
		acknowledgeFlags: 0,
		returnCode:       0,
	})

	return session, nil
}

func (s *Session) handle() {
	for {
		packetByte, err := s.Reader.ReadByte()
		if err != nil {
			fmt.Println(err)
			return
		}

		err = s.Reader.UnreadByte()
		if err != nil {
			fmt.Println(err)
			return
		}

		switch packetByte & mqttPacketTypeMask {
		case mqttPacketPublish:
			s.handlePublish()
		case mqttPacketPuback:
			s.handlePuback()
		case mqttPacketPubrec:
			s.handlePubrec()
		case mqttPacketPubrel:
			s.handlePubrel()
		case mqttPacketPubcomp:
			s.handlePubcomp()
		case mqttPacketSubscribe:
			s.handleSubscribe()
		case mqttPacketUnsubscribe:
			s.handleUnsubscribe()
		case mqttPacketPingreq:
			s.handlePingreq()
		case mqttPacketDisconnect:
			s.handleDisconnect()
		case mqttPacketAuth:
			s.handleAuth()
		default:
			// FIXME
			return
		}
	}
}

func (s *Session) handlePublish() error {
	packet, err := s.Handler.RecvPublish(s.Reader)
	if err != nil {
		return err
	}

	if err = s.Acl.AllowPublish(s, packet); err != nil {
		return err
	}

	subs := topics.matchSubscribers(packet.Topic)
	for _, sub := range subs {
		s.Handler.SendPublish(sub.Conn, packet)
	}

	return nil
}

func (s *Session) handlePuback() error {
	_, err := s.Handler.RecvPuback(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handlePubrec() error {
	_, err := s.Handler.RecvPubrec(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handlePubrel() error {
	_, err := s.Handler.RecvPubrel(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handlePubcomp() error {
	_, err := s.Handler.RecvPubcomp(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handleSubscribe() error {
	packet, err := s.Handler.RecvSubscribe(s.Reader)
	if err != nil {
		return err
	}

	if err = s.Acl.AllowSubscribe(s, packet); err != nil {
		return err
	}

	for _, topic := range packet.Topics {
		topic, err := topics.addSubscription(topic, s)
		if err != nil {
			return err
		}
		s.Subscriptions = append(s.Subscriptions, topic)
	}

	err = s.Handler.SendSuback(s.Conn, &SubackPacket{
		packetId: packet.PacketId,
		qos:      packet.Qos,
	})

	return err
}

func (s *Session) handleUnsubscribe() error {
	_, err := s.Handler.RecvUnsubscribe(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handlePingreq() error {
	_, err := s.Handler.RecvPingreq(s.Reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) handleDisconnect() error {
	_, err := s.Handler.RecvDisconnect(s.Reader)
	if err != nil {
		return err
	}

	for _, sub := range s.Subscriptions {
		sub.removeSubscription(s)
	}

	return nil
}

func (s *Session) handleAuth() error {
	_, err := s.Handler.RecvAuth(s.Reader)
	if err != nil {
		return err
	}

	return nil
}
