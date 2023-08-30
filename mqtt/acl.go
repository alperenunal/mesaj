package mqtt

type Acl interface {
	AllowConnect(packet *ConnectPacket) error
	AllowPublish(session *Session, packet *PublishPacket) error
	AllowSubscribe(session *Session, packet *SubscribePacket) error
}

func DefaultAcl() Acl {
	return defaultAcl{}
}

type defaultAcl struct {
}

func (acl defaultAcl) AllowConnect(packet *ConnectPacket) error {
	return nil
}

func (acl defaultAcl) AllowPublish(session *Session, packet *PublishPacket) error {
	return nil
}

func (acl defaultAcl) AllowSubscribe(session *Session, packet *SubscribePacket) error {
	return nil
}
