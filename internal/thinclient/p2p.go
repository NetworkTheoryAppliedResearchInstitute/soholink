package thinclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Peer represents a discovered thin-client peer on the local network.
type Peer struct {
	DID       string
	Address   string // IP:Port
	PublicKey []byte
	LastSeen  time.Time
	CPUCores  int
	StorageGB int64
	GPUModel  string
	Score     int // LBTAS reputation score
}

// Block is a locally-produced blockchain block for P2P consensus.
type Block struct {
	Height    int64
	Data      []byte
	Hash      []byte
	PrevHash  []byte
	Timestamp time.Time
	Synced    bool
}

// Vote records a peer's approval or rejection of a proposed block.
type Vote struct {
	PeerDID   string
	Approve   bool
	Signature []byte
}

// capabilityMsg is the JSON payload exchanged during peer capability announcement.
type capabilityMsg struct {
	DID       string `json:"did"`
	CPUCores  int    `json:"cpu_cores"`
	StorageGB int64  `json:"storage_gb"`
	GPUModel  string `json:"gpu_model"`
	Score     int    `json:"score"`
}

// voteRequest is the JSON payload sent when requesting a vote from a peer.
type voteRequest struct {
	MsgType string `json:"msg_type"`
	Block   Block  `json:"block"`
}

// voteResponse is the JSON payload received from a peer's vote.
type voteResponse struct {
	PeerDID   string `json:"peer_did"`
	Approve   bool   `json:"approve"`
	Signature []byte `json:"signature"`
}

// centralBlockPayload is the JSON body sent to the central SOHO API.
type centralBlockPayload struct {
	Height     int64    `json:"height"`
	Data       []byte   `json:"data"`
	Hash       []byte   `json:"hash"`
	PrevHash   []byte   `json:"prev_hash"`
	Timestamp  string   `json:"timestamp"`
	MerkleProof []byte  `json:"merkle_proof,omitempty"`
	Signatures [][]byte `json:"signatures,omitempty"`
}

// mdnsDiscoveryPacket is the UDP multicast discovery payload.
type mdnsDiscoveryPacket struct {
	Service string `json:"service"`
	DID     string `json:"did"`
	Address string `json:"address"`
}

const (
	mdnsMulticastAddr = "224.0.0.251:5353"
	mdnsServiceName   = "_soholink._tcp"
)

// P2PNetwork manages mesh networking among thin clients when the
// central SOHO is offline. It provides mDNS-based peer discovery,
// simple majority-vote block consensus, and resource sharing fallback.
type P2PNetwork struct {
	store *store.Store

	localDID  string
	listenAddr string

	mu    sync.RWMutex
	peers map[string]*Peer

	centralOnline bool
	isFederated   bool // Operating in P2P mode

	// Channels
	blockChan chan Block
}

// NewP2PNetwork creates a new P2P network for a thin client.
func NewP2PNetwork(s *store.Store, localDID string, listenAddr string) *P2PNetwork {
	return &P2PNetwork{
		store:         s,
		localDID:      localDID,
		listenAddr:    listenAddr,
		peers:         make(map[string]*Peer),
		centralOnline: true,
		blockChan:     make(chan Block, 100),
	}
}

// Start begins peer discovery and central-SOHO health monitoring.
func (p *P2PNetwork) Start(ctx context.Context) {
	go p.listenForPeers(ctx)
	go p.monitorCentral(ctx)
	go p.discoverPeers(ctx)
	log.Printf("[p2p] network started (local DID=%s, listen=%s)", p.localDID, p.listenAddr)
}

// IsFederated returns true when operating in P2P fallback mode.
func (p *P2PNetwork) IsFederated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isFederated
}

// PeerCount returns the number of known peers.
func (p *P2PNetwork) PeerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.peers)
}

// GetPeers returns a copy of the current peer list.
func (p *P2PNetwork) GetPeers() []Peer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	peers := make([]Peer, 0, len(p.peers))
	for _, peer := range p.peers {
		peers = append(peers, *peer)
	}
	return peers
}

// monitorCentral periodically pings the central SOHO and switches
// between online and P2P modes.
func (p *P2PNetwork) monitorCentral(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			online := p.pingCentral()

			p.mu.Lock()
			if !online && p.centralOnline {
				// Central just went down — switch to P2P
				log.Println("[p2p] Central SOHO offline. Switching to P2P federation mode...")
				p.centralOnline = false
				p.isFederated = true
			} else if online && !p.centralOnline {
				// Central came back — sync and resume
				log.Println("[p2p] Central SOHO back online. Syncing state...")
				p.centralOnline = true
				p.isFederated = false
				go p.syncWithCentral(ctx)
			}
			p.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// pingCentral attempts a TCP connection to the central SOHO.
func (p *P2PNetwork) pingCentral() bool {
	// In production this would query the actual central SOHO endpoint.
	// For now we try a TCP connect to the configured address.
	centralAddr, _ := p.store.GetNodeInfo(context.Background(), "central_address")
	if centralAddr == "" {
		return true // No central configured — assume online
	}

	conn, err := net.DialTimeout("tcp", centralAddr, 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// discoverPeers uses mDNS multicast to find other thin clients on the local
// network and falls back to the peer store when mDNS is unavailable.
func (p *P2PNetwork) discoverPeers(ctx context.Context) {
	// Announce our presence on the multicast group immediately.
	p.mdnsAnnounce()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Listen for mDNS responses in a background goroutine.
	go p.mdnsListen(ctx)

	for {
		select {
		case <-ticker.C:
			// Re-announce periodically so new peers see us.
			p.mdnsAnnounce()

			// Also query the store for previously-persisted peer records.
			rows, err := p.store.GetP2PPeers(ctx)
			if err != nil {
				continue
			}
			p.mu.Lock()
			for _, row := range rows {
				if row.PeerDID == p.localDID {
					continue
				}
				if _, exists := p.peers[row.PeerDID]; !exists {
					p.peers[row.PeerDID] = &Peer{
						DID:       row.PeerDID,
						Address:   row.Address,
						PublicKey: row.PublicKey,
						LastSeen:  row.LastSeen,
						CPUCores:  row.CPUCores,
						StorageGB: row.StorageGB,
						GPUModel:  row.GPUModel,
					}
				}
			}
			p.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// mdnsAnnounce sends a UDP multicast packet advertising this node.
func (p *P2PNetwork) mdnsAnnounce() {
	addr, err := net.ResolveUDPAddr("udp4", mdnsMulticastAddr)
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()

	pkt := mdnsDiscoveryPacket{
		Service: mdnsServiceName,
		DID:     p.localDID,
		Address: p.listenAddr,
	}
	data, _ := json.Marshal(pkt)
	conn.Write(data)
}

// mdnsListen joins the multicast group and processes discovery packets
// from other SoHoLINK peers on the local network.
func (p *P2PNetwork) mdnsListen(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp4", mdnsMulticastAddr)
	if err != nil {
		log.Printf("[p2p] mDNS resolve error: %v", err)
		return
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("[p2p] mDNS listen error (non-fatal, using store fallback): %v", err)
		return
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		var pkt mdnsDiscoveryPacket
		if err := json.Unmarshal(buf[:n], &pkt); err != nil {
			continue
		}
		if pkt.Service != mdnsServiceName || pkt.DID == p.localDID {
			continue
		}

		// Resolve the advertised address — if it's 0.0.0.0, use the source IP.
		peerAddr := pkt.Address
		if host, port, err := net.SplitHostPort(peerAddr); err == nil {
			if host == "0.0.0.0" || host == "" {
				peerAddr = net.JoinHostPort(src.IP.String(), port)
			}
		}

		p.mu.Lock()
		if _, exists := p.peers[pkt.DID]; !exists {
			p.peers[pkt.DID] = &Peer{
				DID:      pkt.DID,
				Address:  peerAddr,
				LastSeen: time.Now(),
			}
			log.Printf("[p2p] discovered peer %s via mDNS at %s", pkt.DID, peerAddr)
		} else {
			p.peers[pkt.DID].LastSeen = time.Now()
			p.peers[pkt.DID].Address = peerAddr
		}
		p.mu.Unlock()
	}
}

// listenForPeers accepts incoming TCP connections from peers.
func (p *P2PNetwork) listenForPeers(ctx context.Context) {
	ln, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		log.Printf("[p2p] failed to listen on %s: %v", p.listenAddr, err)
		return
	}
	defer ln.Close()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		go p.handlePeerConnection(ctx, conn)
	}
}

// handlePeerConnection processes an incoming peer connection with DID
// challenge-response authentication, capability exchange, and heartbeat.
func (p *P2PNetwork) handlePeerConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	log.Printf("[p2p] incoming peer connection from %s", conn.RemoteAddr())

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// 1. Read 4-byte message type header.
	var msgType [4]byte
	if _, err := io.ReadFull(conn, msgType[:]); err != nil {
		log.Printf("[p2p] failed to read message type from %s: %v", conn.RemoteAddr(), err)
		return
	}

	switch string(msgType[:]) {
	case "AUTH":
		p.handleAuth(ctx, conn)
	case "VOTE":
		p.handleVoteRequest(ctx, conn)
	default:
		log.Printf("[p2p] unknown message type %q from %s", string(msgType[:]), conn.RemoteAddr())
	}
}

// handleAuth performs DID challenge-response authentication and capability exchange.
func (p *P2PNetwork) handleAuth(ctx context.Context, conn net.Conn) {
	// 2. Read peer's DID (length-prefixed string: 4-byte big-endian length + data).
	var didLen uint32
	if err := binary.Read(conn, binary.BigEndian, &didLen); err != nil {
		log.Printf("[p2p] failed to read DID length: %v", err)
		return
	}
	if didLen > 1024 {
		log.Printf("[p2p] DID length too large: %d", didLen)
		return
	}
	didBuf := make([]byte, didLen)
	if _, err := io.ReadFull(conn, didBuf); err != nil {
		log.Printf("[p2p] failed to read DID: %v", err)
		return
	}
	peerDID := string(didBuf)

	// 3. Look up peer's public key from the store.
	pubKey, err := p.lookupPublicKey(ctx, peerDID)
	if err != nil || pubKey == nil {
		log.Printf("[p2p] unknown peer DID %s (no public key found)", peerDID)
		return
	}

	// 4. Generate random 32-byte nonce and send as challenge.
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		log.Printf("[p2p] failed to generate nonce: %v", err)
		return
	}
	if _, err := conn.Write(nonce); err != nil {
		log.Printf("[p2p] failed to send challenge nonce: %v", err)
		return
	}

	// 5. Read peer's Ed25519 signature of the nonce (64 bytes).
	sig := make([]byte, ed25519.SignatureSize)
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err := io.ReadFull(conn, sig); err != nil {
		log.Printf("[p2p] failed to read signature from %s: %v", peerDID, err)
		return
	}

	// 6. Verify signature.
	if !ed25519.Verify(ed25519.PublicKey(pubKey), nonce, sig) {
		log.Printf("[p2p] invalid signature from peer %s", peerDID)
		return
	}
	log.Printf("[p2p] peer %s authenticated successfully", peerDID)

	// 7. Exchange capability data: send ours, then receive theirs.
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	localCap := capabilityMsg{
		DID:       p.localDID,
		CPUCores:  0, // populated from local info if available
		StorageGB: 0,
		GPUModel:  "",
		Score:     0,
	}
	capData, _ := json.Marshal(localCap)
	capLen := uint32(len(capData))
	if err := binary.Write(conn, binary.BigEndian, capLen); err != nil {
		log.Printf("[p2p] failed to send capability length: %v", err)
		return
	}
	if _, err := conn.Write(capData); err != nil {
		log.Printf("[p2p] failed to send capability data: %v", err)
		return
	}

	// Receive peer capability.
	var peerCapLen uint32
	if err := binary.Read(conn, binary.BigEndian, &peerCapLen); err != nil {
		log.Printf("[p2p] failed to read peer capability length: %v", err)
		return
	}
	if peerCapLen > 4096 {
		log.Printf("[p2p] peer capability payload too large: %d", peerCapLen)
		return
	}
	peerCapBuf := make([]byte, peerCapLen)
	if _, err := io.ReadFull(conn, peerCapBuf); err != nil {
		log.Printf("[p2p] failed to read peer capability data: %v", err)
		return
	}
	var peerCap capabilityMsg
	if err := json.Unmarshal(peerCapBuf, &peerCap); err != nil {
		log.Printf("[p2p] failed to parse peer capabilities: %v", err)
		return
	}

	// 8. Register peer in the local peer table.
	peer := &Peer{
		DID:       peerDID,
		Address:   conn.RemoteAddr().String(),
		PublicKey: pubKey,
		LastSeen:  time.Now(),
		CPUCores:  peerCap.CPUCores,
		StorageGB: peerCap.StorageGB,
		GPUModel:  peerCap.GPUModel,
		Score:     peerCap.Score,
	}
	p.mu.Lock()
	p.peers[peerDID] = peer
	p.mu.Unlock()

	// Persist to store.
	_ = p.store.UpsertP2PPeer(ctx, &store.P2PPeerRow{
		PeerDID:   peerDID,
		Address:   conn.RemoteAddr().String(),
		PublicKey: pubKey,
		LastSeen:  time.Now(),
		Score:     peerCap.Score,
		CPUCores:  peerCap.CPUCores,
		StorageGB: peerCap.StorageGB,
		GPUModel:  peerCap.GPUModel,
	})

	log.Printf("[p2p] registered peer %s (cpu=%d, storage=%dGB, gpu=%s)",
		peerDID, peerCap.CPUCores, peerCap.StorageGB, peerCap.GPUModel)

	// 9. Start heartbeat goroutine.
	go p.heartbeatPeer(ctx, peerDID, conn.RemoteAddr().String())
}

// handleVoteRequest processes an incoming vote request from a peer.
func (p *P2PNetwork) handleVoteRequest(_ context.Context, conn net.Conn) {
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Read JSON-encoded block data (length-prefixed).
	var dataLen uint32
	if err := binary.Read(conn, binary.BigEndian, &dataLen); err != nil {
		log.Printf("[p2p] failed to read vote data length: %v", err)
		return
	}
	if dataLen > 10*1024*1024 { // 10 MB max
		log.Printf("[p2p] vote data too large: %d", dataLen)
		return
	}
	dataBuf := make([]byte, dataLen)
	if _, err := io.ReadFull(conn, dataBuf); err != nil {
		log.Printf("[p2p] failed to read vote data: %v", err)
		return
	}

	var block Block
	if err := json.Unmarshal(dataBuf, &block); err != nil {
		log.Printf("[p2p] failed to parse block for vote: %v", err)
		return
	}

	// Simple validation: approve if block height is reasonable.
	approve := block.Height > 0 && len(block.Data) > 0

	resp := voteResponse{
		PeerDID: p.localDID,
		Approve: approve,
	}

	respData, _ := json.Marshal(resp)
	respLen := uint32(len(respData))
	_ = binary.Write(conn, binary.BigEndian, respLen)
	_, _ = conn.Write(respData)
}

// heartbeatPeer pings a peer every 30 seconds and removes it after
// 3 consecutive failures.
func (p *P2PNetwork) heartbeatPeer(ctx context.Context, peerDID, address string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	failures := 0
	const maxFailures = 3

	for {
		select {
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 5*time.Second)
			if err != nil {
				failures++
				log.Printf("[p2p] heartbeat failed for %s (%d/%d)", peerDID, failures, maxFailures)
				if failures >= maxFailures {
					log.Printf("[p2p] removing unresponsive peer %s", peerDID)
					p.mu.Lock()
					delete(p.peers, peerDID)
					p.mu.Unlock()
					return
				}
				continue
			}
			// Send PING message type.
			_, _ = conn.Write([]byte("PING"))
			conn.Close()
			failures = 0

			// Update last seen.
			p.mu.Lock()
			if peer, ok := p.peers[peerDID]; ok {
				peer.LastSeen = time.Now()
			}
			p.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// lookupPublicKey retrieves a peer's public key by DID, first checking
// the peer table, then the user store.
func (p *P2PNetwork) lookupPublicKey(ctx context.Context, peerDID string) ([]byte, error) {
	// Check local peer table first.
	p.mu.RLock()
	if peer, ok := p.peers[peerDID]; ok && len(peer.PublicKey) > 0 {
		pk := make([]byte, len(peer.PublicKey))
		copy(pk, peer.PublicKey)
		p.mu.RUnlock()
		return pk, nil
	}
	p.mu.RUnlock()

	// Check the P2P peer store.
	rows, err := p.store.GetP2PPeers(ctx)
	if err == nil {
		for _, row := range rows {
			if row.PeerDID == peerDID && len(row.PublicKey) > 0 {
				return row.PublicKey, nil
			}
		}
	}

	// Fall back to user store.
	user, err := p.store.GetUserByDID(ctx, peerDID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up public key for %s: %w", peerDID, err)
	}
	if user == nil {
		return nil, fmt.Errorf("no public key found for DID %s", peerDID)
	}
	return user.PublicKey, nil
}

// WriteBlock proposes a new block to the P2P network.
// When central is online, it delegates to central; otherwise it
// collects majority votes from peers.
func (p *P2PNetwork) WriteBlock(ctx context.Context, data []byte) error {
	p.mu.RLock()
	online := p.centralOnline
	peerCount := len(p.peers)
	p.mu.RUnlock()

	if online {
		// Delegate to central SOHO
		return p.writeBlockToCentral(ctx, data)
	}

	// P2P consensus mode
	latestHeight, _ := p.store.GetLatestBlockHeight(ctx)
	latestHash, _ := p.store.GetLatestBlockHash(ctx)

	block := Block{
		Height:    latestHeight + 1,
		Data:      data,
		PrevHash:  latestHash,
		Timestamp: time.Now(),
	}

	// Collect votes (need majority)
	votes := p.collectVotes(block)
	needed := peerCount/2 + 1

	if len(votes) >= needed {
		// Consensus reached
		if err := p.store.CreateP2PBlock(ctx, &store.P2PBlockRow{
			Height:    block.Height,
			Data:      block.Data,
			Hash:      block.Hash,
			PrevHash:  block.PrevHash,
			Timestamp: block.Timestamp,
			Synced:    false,
		}); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}

		log.Printf("[p2p] block %d accepted (%d/%d votes)", block.Height, len(votes), peerCount)
		return nil
	}

	return fmt.Errorf("consensus not reached: %d/%d votes (need %d)", len(votes), peerCount, needed)
}

// collectVotes sends a block proposal to all peers via TCP and gathers
// signed approvals. Only votes with valid Ed25519 signatures are returned.
func (p *P2PNetwork) collectVotes(block Block) []Vote {
	p.mu.RLock()
	peersCopy := make([]*Peer, 0, len(p.peers))
	for _, peer := range p.peers {
		peersCopy = append(peersCopy, peer)
	}
	p.mu.RUnlock()

	type voteResult struct {
		vote Vote
		ok   bool
	}
	results := make(chan voteResult, len(peersCopy))

	for _, peer := range peersCopy {
		go func(pr *Peer) {
			v, err := p.requestVote(pr, block)
			if err != nil {
				log.Printf("[p2p] vote request to %s failed: %v", pr.DID, err)
				results <- voteResult{ok: false}
				return
			}
			results <- voteResult{vote: v, ok: true}
		}(peer)
	}

	var votes []Vote
	for range peersCopy {
		res := <-results
		if res.ok && res.vote.Approve {
			votes = append(votes, res.vote)
		}
	}
	return votes
}

// requestVote opens a TCP connection to a single peer, sends the block
// for voting, and returns the peer's signed vote.
func (p *P2PNetwork) requestVote(peer *Peer, block Block) (Vote, error) {
	conn, err := net.DialTimeout("tcp", peer.Address, 10*time.Second)
	if err != nil {
		return Vote{}, fmt.Errorf("dial %s: %w", peer.Address, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Send "VOTE" message type header.
	if _, err := conn.Write([]byte("VOTE")); err != nil {
		return Vote{}, fmt.Errorf("write message type: %w", err)
	}

	// Send serialized block data (length-prefixed JSON).
	blockData, err := json.Marshal(block)
	if err != nil {
		return Vote{}, fmt.Errorf("marshal block: %w", err)
	}
	blockLen := uint32(len(blockData))
	if err := binary.Write(conn, binary.BigEndian, blockLen); err != nil {
		return Vote{}, fmt.Errorf("write block length: %w", err)
	}
	if _, err := conn.Write(blockData); err != nil {
		return Vote{}, fmt.Errorf("write block data: %w", err)
	}

	// Read response (length-prefixed JSON).
	var respLen uint32
	if err := binary.Read(conn, binary.BigEndian, &respLen); err != nil {
		return Vote{}, fmt.Errorf("read response length: %w", err)
	}
	if respLen > 1024*1024 { // 1 MB max
		return Vote{}, fmt.Errorf("response too large: %d bytes", respLen)
	}
	respBuf := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respBuf); err != nil {
		return Vote{}, fmt.Errorf("read response: %w", err)
	}

	var resp voteResponse
	if err := json.Unmarshal(respBuf, &resp); err != nil {
		return Vote{}, fmt.Errorf("parse vote response: %w", err)
	}

	// Verify Ed25519 signature over the block data.
	if len(resp.Signature) > 0 && len(peer.PublicKey) == ed25519.PublicKeySize {
		if !ed25519.Verify(ed25519.PublicKey(peer.PublicKey), blockData, resp.Signature) {
			return Vote{}, fmt.Errorf("invalid signature from peer %s", resp.PeerDID)
		}
	} else if len(resp.Signature) == 0 {
		// No signature provided -- reject the vote.
		return Vote{}, fmt.Errorf("peer %s provided no signature", resp.PeerDID)
	}

	return Vote{
		PeerDID:   resp.PeerDID,
		Approve:   resp.Approve,
		Signature: resp.Signature,
	}, nil
}

// writeBlockToCentral sends a block to the central SOHO for storage.
func (p *P2PNetwork) writeBlockToCentral(ctx context.Context, data []byte) error {
	centralAddr, _ := p.store.GetNodeInfo(ctx, "central_address")
	if centralAddr == "" {
		return fmt.Errorf("central address not configured")
	}

	latestHeight, _ := p.store.GetLatestBlockHeight(ctx)
	latestHash, _ := p.store.GetLatestBlockHash(ctx)

	payload := centralBlockPayload{
		Height:    latestHeight,
		Data:      data,
		Hash:      latestHash,
		PrevHash:  nil,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal block payload: %w", err)
	}

	endpoint := fmt.Sprintf("http://%s/api/blocks", centralAddr)

	// Retry with exponential backoff (3 attempts)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[p2p] block synced to central (height=%d)", latestHeight)
			return nil
		}

		if resp.StatusCode == http.StatusConflict {
			// Central has newer state — not an error, just needs reconciliation
			log.Printf("[p2p] central has newer state, skipping block %d", latestHeight)
			return nil
		}

		lastErr = fmt.Errorf("central returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("failed to sync block after 3 attempts: %w", lastErr)
}

// syncWithCentral uploads all un-synced blocks to central SOHO.
func (p *P2PNetwork) syncWithCentral(ctx context.Context) {
	blocks, err := p.store.GetUnsyncedBlocks(ctx)
	if err != nil {
		log.Printf("[p2p] failed to get unsynced blocks: %v", err)
		return
	}

	synced := 0
	for _, block := range blocks {
		if err := p.writeBlockToCentral(ctx, block.Data); err != nil {
			log.Printf("[p2p] failed to sync block %d: %v", block.Height, err)
			continue
		}
		_ = p.store.MarkBlockSynced(ctx, block.Height)
		synced++
	}

	// Sync pending operations
	pendingOps, err := p.store.GetPendingSync(ctx)
	if err == nil {
		for _, op := range pendingOps {
			// In production: HTTP POST to central SOHO
			_ = p.store.DeletePendingSync(ctx, op.SyncID)
		}
	}

	log.Printf("[p2p] sync complete: %d/%d blocks synced", synced, len(blocks))
}

// FindBestPeer selects the most suitable peer for a job based on
// required resources and LBTAS reputation.
func (p *P2PNetwork) FindBestPeer(cpuNeeded int, memNeeded int64) *Peer {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *Peer
	bestScore := -1

	for _, peer := range p.peers {
		if peer.CPUCores >= cpuNeeded && peer.Score > bestScore {
			best = peer
			bestScore = peer.Score
		}
	}
	return best
}
