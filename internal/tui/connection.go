package tui

type connectionState uint8

const (
	connectionOffline connectionState = iota
	connectionConnecting
	connectionOnline
)

func (m *Model) renderConnectionBadge() string {
	switch m.connectionState {
	case connectionConnecting:
		return m.theme.connPending.Render("[CONNECTING]")
	case connectionOnline:
		return m.theme.connOnline.Render("[ONLINE]")
	default:
		return m.theme.connOffline.Render("[OFFLINE]")
	}
}
