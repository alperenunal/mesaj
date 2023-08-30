package mqtt

import (
	"strings"
	"sync"
)

var (
	topics = newTopicTree()
)

type topicLevel struct {
	mu sync.RWMutex

	children    map[string]*topicLevel
	subscribers map[*Session]struct{}

	wildcardPlus  *topicLevel
	wildcardSharp *topicLevel

	retainMessage string
}

func newTopicLevel() *topicLevel {
	return &topicLevel{
		mu:            sync.RWMutex{},
		subscribers:   make(map[*Session]struct{}),
		children:      make(map[string]*topicLevel),
		wildcardPlus:  nil,
		wildcardSharp: nil,
		retainMessage: "",
	}
}

func (level *topicLevel) getSubscribers() []*Session {
	level.mu.RLock()
	defer level.mu.RUnlock()

	subs := make([]*Session, 0, len(level.subscribers))
	for s := range level.subscribers {
		subs = append(subs, s)
	}
	return subs
}

func (level *topicLevel) removeSubscription(session *Session) {
	level.mu.Lock()
	defer level.mu.Unlock()

	delete(level.subscribers, session)
}

type topicTrie struct {
	root *topicLevel
	mu   sync.RWMutex
}

func newTopicTree() *topicTrie {
	return &topicTrie{
		root: newTopicLevel(),
		mu:   sync.RWMutex{},
	}
}

func (trie *topicTrie) addSubscription(topic string, session *Session) (*topicLevel, error) {
	trie.mu.Lock()
	defer trie.mu.Unlock()

	node := trie.root
	levels := strings.Split(topic, "/")

	for _, level := range levels {
		node.mu.Lock()

		if level == "+" {
			if node.wildcardPlus == nil {
				node.wildcardPlus = newTopicLevel()
			}
			node.mu.Unlock()
			node = node.wildcardPlus
		} else if level == "#" {
			if node.wildcardSharp == nil {
				node.wildcardSharp = newTopicLevel()
			}
			node.mu.Unlock()
			node = node.wildcardSharp
		} else {
			if _, ok := node.children[level]; !ok {
				node.children[level] = newTopicLevel()
			}
			node.mu.Unlock()
			node = node.children[level]
		}
	}

	node.mu.Lock()
	node.subscribers[session] = struct{}{}
	node.mu.Unlock()

	return node, nil
}

func (trie *topicTrie) matchSubscribers(topic string) []*Session {
	trie.mu.RLock()
	defer trie.mu.RUnlock()

	node := trie.root
	levels := strings.Split(topic, "/")
	var clients []*Session

	for i, level := range levels {
		node.mu.RLock()

		if node.wildcardSharp != nil {
			clients = append(clients, node.wildcardSharp.getSubscribers()...)
		}

		if node.wildcardPlus != nil {
			if i+1 == len(levels) {
				clients = append(clients, node.wildcardPlus.getSubscribers()...)
			} else {
				sub := &topicTrie{
					root: node.wildcardPlus,
					mu:   sync.RWMutex{},
				}

				subs := sub.matchSubscribers(strings.Join(levels[i+1:], "/"))
				clients = append(clients, subs...)
			}
		}

		if child, ok := node.children[level]; ok {
			node.mu.RUnlock()
			node = child
		} else {
			break
		}

		if i+1 == len(levels) {
			node.mu.RLock()
			clients = append(clients, node.getSubscribers()...)
			if node.wildcardSharp != nil {
				clients = append(clients, node.wildcardSharp.getSubscribers()...)
			}
			node.mu.RUnlock()
		}
	}

	return clients
}
