package mqtt

const (
	mqttPacketTypeMask = byte(0xf0)
	mqttPacketFlagMask = byte(0x0f)

	mqttPacketConnect     = byte(0x10)
	mqttPacketConnack     = byte(0x20)
	mqttPacketPublish     = byte(0x30)
	mqttPacketPuback      = byte(0x40)
	mqttPacketPubrec      = byte(0x50)
	mqttPacketPubrel      = byte(0x60)
	mqttPacketPubcomp     = byte(0x70)
	mqttPacketSubscribe   = byte(0x80)
	mqttPacketSuback      = byte(0x90)
	mqttPacketUnsubscribe = byte(0xa0)
	mqttPacketUnsuback    = byte(0xb0)
	mqttPacketPingreq     = byte(0xc0)
	mqttPacketPingresp    = byte(0xd0)
	mqttPacketDisconnect  = byte(0xe0)
	mqttPacketAuth        = byte(0xf0)

	mqttConnectFlagUserName     = byte(0x80)
	mqttConnectFlagPassword     = byte(0x40)
	mqttConnectFlagWillRetain   = byte(0x20)
	mqttConnectFlagWillQos      = byte(0x18)
	mqttConnectFlagWillFlag     = byte(0x04)
	mqttConnectFlagCleanSession = byte(0x02)

	mqttPublishFlagRetain = byte(0x01)
	mqttPublishFlagQos    = byte(0x06)
	mqttPublishFlagDup    = byte(0x08)

	mqttPubrelFlag      = byte(0x02)
	mqttSubscribeFlag   = byte(0x02)
	mqttUnsubscribeFlag = byte(0x02)
)

type ConnectPacket struct {
	UsernameFlag bool
	PasswordFlag bool
	WillFlag     bool
	WillQoS      byte
	WillRetain   bool
	CleanSession bool
	KeepAlive    uint16
	ClientId     string
	WillTopic    string
	WillMessage  string
	Username     string
	Password     string
}

type ConnackPacket struct {
	acknowledgeFlags byte
	returnCode       byte
}

type PublishPacket struct {
	Duplicate bool
	Qos       byte
	Retain    bool
	Topic     string
	PacketId  uint16
	Payload   string
}

type PubackPacket struct {
	packetId uint16
}

type PubrecPacket struct {
	packetId uint16
}

type PubrelPacket struct {
	packetId uint16
}

type PubcompPacket struct {
	packetId uint16
}

type SubscribePacket struct {
	PacketId uint16
	Topics   []string
	Qos      []byte
}

type SubackPacket struct {
	packetId uint16
	qos      []byte
}

type UnsubscribePacket struct {
	packetId uint16
	topics   []string
}

type UnsubackPacket struct {
	packetId uint16
}

type PingreqPacket struct {
}

type PingrespPacket struct {
}

type DisconnectPacket struct {
}

type AuthPacket struct {
}
