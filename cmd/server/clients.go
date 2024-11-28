package main

import "sync"

type ClientManager struct {
	clients map[string]*Client
	mu      sync.Mutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

func (m *ClientManager) RegisterClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client.ClientId] = client
}

func (m *ClientManager) RemoveClient(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clientID)
}

func (m *ClientManager) GetClient(clientID string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[clientID]
}
