package mqtt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	errInvalidUTF8         = errors.New("invalid UTF-8 string")
	errReservedViolation   = errors.New("reserved field violation")
	errUnsupportedPacket   = errors.New("packet type not supported")
	errUnsupportedProtocol = errors.New("unsupported protocol")
	errWildcardTopic       = errors.New("invalid wildcard topic")
	errWrongPacketType     = errors.New("wrong packet byte")
)

type handler interface {
	RecvConnect(packetBody []byte) (*ConnectPacket, error)
	RecvPublish(reader *bufio.Reader) (*PublishPacket, error)
	RecvPuback(reader *bufio.Reader) (*PubackPacket, error)
	RecvPubrec(reader *bufio.Reader) (*PubrecPacket, error)
	RecvPubrel(reader *bufio.Reader) (*PubrelPacket, error)
	RecvPubcomp(reader *bufio.Reader) (*PubcompPacket, error)
	RecvSubscribe(reader *bufio.Reader) (*SubscribePacket, error)
	RecvUnsubscribe(reader *bufio.Reader) (*UnsubscribePacket, error)
	RecvPingreq(reader *bufio.Reader) (*PingreqPacket, error)
	RecvDisconnect(reader *bufio.Reader) (*DisconnectPacket, error)
	RecvAuth(reader *bufio.Reader) (*AuthPacket, error)

	SendConnack(writer io.Writer, packet *ConnackPacket) error
	SendPublish(writer io.Writer, packet *PublishPacket) error
	SendPuback(writer io.Writer, packet *PubackPacket) error
	SendPubrec(writer io.Writer, packet *PubrecPacket) error
	SendPubrel(writer io.Writer, packet *PubrelPacket) error
	SendPubcomp(writer io.Writer, packet *PubcompPacket) error
	SendSuback(writer io.Writer, packet *SubackPacket) error
	SendUnsuback(writer io.Writer, packet *UnsubackPacket) error
	SendPingresp(writer io.Writer, packet *PingrespPacket) error
	SendAuth(writer io.Writer, packet *AuthPacket) error
}

func newHandler(version byte) (handler, error) {
	switch version {
	case 4:
		return &handlerV4{}, nil
	default:
		return nil, errUnsupportedProtocol
	}
}

func panicHandler() {
	if r := recover(); r != nil {
		fmt.Fprintf(os.Stderr, "%v\n", r)
	}
}

func decodeRemainingLength(reader *bufio.Reader) (uint32, error) {
	var value uint32 = 0
	var multiplier uint32 = 1

	for {
		encoded, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		value += uint32(encoded&127) * multiplier
		multiplier *= 128

		if multiplier > uint32(math.Pow(128, 3)) {
			return 0, errors.New("overflow")
		}
		if encoded&128 == 0 {
			break
		}
	}

	return value, nil
}

func encodeRemainingLength(len uint32) []byte {
	var bytes []byte

	for len > 0 {
		encoded := len % 128
		len = len / 128

		if len > 0 {
			encoded = encoded | 128
		}
		bytes = append(bytes, byte(encoded))
	}

	return bytes
}

func validateTopic(topic string) error {
	levels := strings.Split(topic, "/")

	for i, level := range levels {
		if strings.Contains(level, "+") && len(level) != 1 {
			return errWildcardTopic
		}

		if strings.Contains(level, "#") {
			if len(level) != 1 {
				return errWildcardTopic
			}

			if i+1 != len(levels) {
				return errWildcardTopic
			}
		}
	}

	return nil
}

type handlerV4 struct {
}

func (h *handlerV4) RecvConnect(packetBody []byte) (*ConnectPacket, error) {
	defer panicHandler()

	connectFlags := packetBody[0]

	usernameFlag := (connectFlags & mqttConnectFlagUserName) != 0
	passwordFlag := (connectFlags & mqttConnectFlagPassword) != 0
	willFlag := (connectFlags & mqttConnectFlagWillFlag) != 0
	willRetain := (connectFlags & mqttConnectFlagWillRetain) != 0
	willQoS := (connectFlags & mqttConnectFlagWillQos) >> 3
	cleanSession := (connectFlags & mqttConnectFlagCleanSession) != 0

	idx := 1
	keepAlive := binary.BigEndian.Uint16(packetBody[idx : idx+2])
	idx += 2

	clientIdLen := binary.BigEndian.Uint16(packetBody[idx : idx+2])
	idx += 2
	clientId := packetBody[idx : idx+int(clientIdLen)]
	idx += int(clientIdLen)

	if !utf8.Valid(clientId) {
		return nil, errInvalidUTF8
	}

	var willTopic []byte
	var willMessage []byte
	var username []byte
	var password []byte

	if willFlag {
		willTopicLen := binary.BigEndian.Uint16(packetBody[idx : idx+2])
		idx += 2
		willTopic = packetBody[idx : idx+int(willTopicLen)]
		idx += int(willTopicLen)

		willMsgLen := binary.BigEndian.Uint16(packetBody[idx : idx+2])
		idx += 2
		willMessage = packetBody[idx : idx+int(willMsgLen)]
		idx += int(willMsgLen)

		if !utf8.Valid(willMessage) {
			return nil, errInvalidUTF8
		}
	}

	if usernameFlag {
		usernameLen := binary.BigEndian.Uint16(packetBody[idx : idx+2])
		idx += 2
		username = packetBody[idx : idx+int(usernameLen)]
		idx += int(usernameLen)

		if !utf8.Valid(username) {
			return nil, errInvalidUTF8
		}
	}

	if passwordFlag {
		passwordLen := binary.BigEndian.Uint16(packetBody[idx : idx+2])
		idx += 2
		password = packetBody[idx : idx+int(passwordLen)]
		idx += int(passwordLen)

		if !utf8.Valid(password) {
			return nil, errInvalidUTF8
		}
	}

	packet := &ConnectPacket{
		UsernameFlag: usernameFlag,
		PasswordFlag: passwordFlag,
		WillFlag:     willFlag,
		WillQoS:      willQoS,
		WillRetain:   willRetain,
		CleanSession: cleanSession,
		KeepAlive:    keepAlive,
		ClientId:     string(clientId),
		WillTopic:    string(willTopic),
		WillMessage:  string(willMessage),
		Username:     string(username),
		Password:     string(password),
	}

	return packet, nil
}

func (h *handlerV4) RecvPublish(reader *bufio.Reader) (*PublishPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPublish {
		return nil, errWrongPacketType
	}

	dup := (packetByte & mqttPublishFlagDup) != 0
	qos := (packetByte & mqttPublishFlagQos) >> 1
	retain := (packetByte & mqttPublishFlagRetain) != 0

	if dup && qos == 0 {
		return nil, errors.New("QoS 0 message can't be a duplicate")
	}

	if qos > 2 {
		return nil, errors.New("invalid qos level")
	}

	length, err := decodeRemainingLength(reader)
	if err != nil {
		return nil, err
	}

	packetBody := make([]byte, length)
	_, err = io.ReadFull(reader, packetBody)
	if err != nil {
		return nil, err
	}

	topicLen := binary.BigEndian.Uint16(packetBody[:2])
	topic := packetBody[2 : topicLen+2]
	if !utf8.Valid(topic) {
		return nil, errInvalidUTF8
	}
	if bytes.ContainsAny(topic, "#+") {
		return nil, errors.New("publish topic can't contain wildcards")
	}

	packetId := uint16(0)
	if qos != 0 {
		packetId = binary.BigEndian.Uint16(packetBody[topicLen+2 : topicLen+4])
		topicLen += 2
	}

	message := packetBody[topicLen+2:]
	if !utf8.Valid(message) {
		return nil, errInvalidUTF8
	}

	packet := &PublishPacket{
		Duplicate: dup,
		Qos:       qos,
		Retain:    retain,
		Topic:     string(topic),
		PacketId:  packetId,
		Payload:   string(message),
	}

	return packet, nil
}

func (h *handlerV4) RecvPuback(reader *bufio.Reader) (*PubackPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPuback {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != 0 {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 2 {
		return nil, errors.New("remaining length should be 2")
	}

	var identifier [2]byte
	_, err = io.ReadFull(reader, identifier[:])
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(identifier[:])
	return &PubackPacket{packetId}, nil
}

func (h *handlerV4) RecvPubrec(reader *bufio.Reader) (*PubrecPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPubrec {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != 0 {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 2 {
		return nil, errors.New("remaining length should be 2")
	}

	var identifier [2]byte
	_, err = io.ReadFull(reader, identifier[:])
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(identifier[:])
	return &PubrecPacket{packetId}, nil
}

func (h *handlerV4) RecvPubrel(reader *bufio.Reader) (*PubrelPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPubrel {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != mqttPubrelFlag {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 2 {
		return nil, errors.New("remaining length should be 2")
	}

	var identifier [2]byte
	_, err = io.ReadFull(reader, identifier[:])
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(identifier[:])
	return &PubrelPacket{packetId}, nil
}

func (h *handlerV4) RecvPubcomp(reader *bufio.Reader) (*PubcompPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPubcomp {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != 0 {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 2 {
		return nil, errors.New("remaining length should be 2")
	}

	var identifier [2]byte
	_, err = io.ReadFull(reader, identifier[:])
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(identifier[:])
	return &PubcompPacket{packetId}, nil
}

func (h *handlerV4) RecvSubscribe(reader *bufio.Reader) (*SubscribePacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketSubscribe {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != mqttSubscribeFlag {
		return nil, errReservedViolation
	}

	remLength, err := decodeRemainingLength(reader)
	if err != nil {
		return nil, err
	}

	packetBody := make([]byte, remLength)
	_, err = io.ReadFull(reader, packetBody)
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(packetBody[:2])
	filters := packetBody[2:]

	var idx uint32 = 0
	var topics []string
	var qosLevels []byte

	for idx < remLength-2 {
		length := binary.BigEndian.Uint16(filters[idx : idx+2])
		idx += 2

		topic := filters[idx : idx+uint32(length)]
		if !utf8.Valid([]byte(topic)) {
			return nil, errInvalidUTF8
		}

		if err = validateTopic(string(topic)); err != nil {
			return nil, err
		}

		idx += uint32(length)
		qos := filters[idx]
		idx++

		topics = append(topics, string(topic))
		qosLevels = append(qosLevels, qos)
	}

	if len(topics) == 0 {
		return nil, errors.New("at least one topic required")
	}

	packet := &SubscribePacket{
		PacketId: packetId,
		Topics:   topics,
		Qos:      qosLevels,
	}
	return packet, nil
}

func (h *handlerV4) RecvUnsubscribe(reader *bufio.Reader) (*UnsubscribePacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketUnsubscribe {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != mqttUnsubscribeFlag {
		return nil, errReservedViolation
	}

	remLength, err := decodeRemainingLength(reader)
	if err != nil {
		return nil, err
	}

	packetBody := make([]byte, remLength)
	_, err = io.ReadFull(reader, packetBody)
	if err != nil {
		return nil, err
	}

	packetId := binary.BigEndian.Uint16(packetBody[:2])
	filters := packetBody[2:]

	var idx uint32 = 0
	var topics []string

	for idx < remLength-2 {
		length := binary.BigEndian.Uint16(filters[idx : idx+2])
		idx += 2

		topic := filters[idx : idx+uint32(length)]
		if !utf8.Valid([]byte(topic)) {
			return nil, errInvalidUTF8
		}

		idx += uint32(length)
		topics = append(topics, string(topic))
	}

	if len(topics) == 0 {
		return nil, errors.New("at least one topic required")
	}

	packet := &UnsubscribePacket{
		packetId: packetId,
		topics:   topics,
	}
	return packet, nil
}

func (h *handlerV4) RecvPingreq(reader *bufio.Reader) (*PingreqPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketPingreq {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != 0 {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 0 {
		return nil, errors.New("remaining length must be 0")
	}

	return &PingreqPacket{}, nil
}

func (h *handlerV4) RecvDisconnect(reader *bufio.Reader) (*DisconnectPacket, error) {
	defer panicHandler()

	packetByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if packetByte&mqttPacketTypeMask != mqttPacketDisconnect {
		return nil, errWrongPacketType
	}

	if packetByte&mqttPacketFlagMask != 0 {
		return nil, errReservedViolation
	}

	length, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if length != 0 {
		return nil, errors.New("remaining length must be 0")
	}

	return &DisconnectPacket{}, nil
}

func (h *handlerV4) RecvAuth(reader *bufio.Reader) (*AuthPacket, error) {
	return nil, errUnsupportedPacket
}

func (h *handlerV4) SendConnack(writer io.Writer, packet *ConnackPacket) error {
	buffer := []byte{0x20, 0x02, packet.acknowledgeFlags, packet.returnCode}
	_, err := writer.Write(buffer)
	return err
}

func (h *handlerV4) SendPublish(writer io.Writer, packet *PublishPacket) error {
	retain := byte(0)
	qos := packet.Qos << 1
	dup := byte(0)
	topicLen := uint16(len(packet.Topic))
	messageLen := len(packet.Payload)
	remLen := encodeRemainingLength(uint32(messageLen) + uint32(topicLen) + 4)

	if packet.Retain {
		retain = 1
	}

	if packet.Duplicate {
		dup = 0b1000
	}

	buffer := []byte{mqttPacketPublish | dup | qos | retain}
	buffer = append(buffer, remLen...)
	buffer = append(buffer, byte(topicLen>>8), byte(topicLen&0x0f))
	buffer = append(buffer, []byte(packet.Topic)...)
	buffer = append(buffer, byte(packet.PacketId>>8), byte(packet.PacketId&0x0f)) // FIXME
	buffer = append(buffer, []byte(packet.Payload)...)

	_, err := writer.Write(buffer)
	return err
}

func (h *handlerV4) SendPuback(writer io.Writer, packet *PubackPacket) error {
	return nil
}

func (h *handlerV4) SendPubrec(writer io.Writer, packet *PubrecPacket) error {
	return nil
}

func (h *handlerV4) SendPubrel(writer io.Writer, packet *PubrelPacket) error {
	return nil
}

func (h *handlerV4) SendPubcomp(writer io.Writer, packet *PubcompPacket) error {
	return nil
}

func (h *handlerV4) SendSuback(writer io.Writer, packet *SubackPacket) error {
	len := encodeRemainingLength(uint32(2 + len(packet.qos)))

	buffer := []byte{mqttPacketSuback}
	buffer = append(buffer, len...)
	buffer = append(buffer, byte(packet.packetId>>8), byte(packet.packetId)&0x0f)
	buffer = append(buffer, packet.qos...)

	_, err := writer.Write(buffer)
	return err
}

func (h *handlerV4) SendUnsuback(writer io.Writer, packet *UnsubackPacket) error {
	return nil
}

func (h *handlerV4) SendPingresp(writer io.Writer, packet *PingrespPacket) error {
	return nil
}

func (h *handlerV4) SendAuth(writer io.Writer, packet *AuthPacket) error {
	return nil
}
