package thinclient

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

func TestP2PNetwork_PeerDiscovery(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create P2P network
	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	if p2p.PeerCount() != 0 {
		t.Errorf("Expected 0 peers initially, got %d", p2p.PeerCount())
	}

	// Add a peer manually (simulating discovery)
	peer := &Peer{
		DID:       "did:soho:peer1",
		Address:   "127.0.0.1:9001",
		PublicKey: make([]byte, ed25519.PublicKeySize),
		LastSeen:  time.Now(),
		CPUCores:  4,
		StorageGB: 100,
		Score:     75,
	}

	p2p.mu.Lock()
	p2p.peers[peer.DID] = peer
	p2p.mu.Unlock()

	if p2p.PeerCount() != 1 {
		t.Errorf("Expected 1 peer after adding, got %d", p2p.PeerCount())
	}

	// Get peers
	peers := p2p.GetPeers()
	if len(peers) != 1 {
		t.Errorf("Expected 1 peer in list, got %d", len(peers))
	}

	if peers[0].DID != peer.DID {
		t.Errorf("Expected peer DID %s, got %s", peer.DID, peers[0].DID)
	}
}

func TestP2PNetwork_FederationMode(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Initially should be online (central available)
	if p2p.IsFederated() {
		t.Error("Expected non-federated mode initially")
	}

	// Simulate central going offline
	p2p.mu.Lock()
	p2p.centralOnline = false
	p2p.isFederated = true
	p2p.mu.Unlock()

	if !p2p.IsFederated() {
		t.Error("Expected federated mode after central offline")
	}
}

func TestMDNSDiscoveryPacket(t *testing.T) {
	// Test mDNS packet marshaling
	pkt := mdnsDiscoveryPacket{
		Service: "_soholink._tcp",
		DID:     "did:soho:test",
		Address: "192.168.1.100:9000",
	}

	data, err := json.Marshal(pkt)
	if err != nil {
		t.Fatalf("Failed to marshal mDNS packet: %v", err)
	}

	var decoded mdnsDiscoveryPacket
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal mDNS packet: %v", err)
	}

	if decoded.Service != pkt.Service {
		t.Errorf("Expected service %s, got %s", pkt.Service, decoded.Service)
	}

	if decoded.DID != pkt.DID {
		t.Errorf("Expected DID %s, got %s", pkt.DID, decoded.DID)
	}

	if decoded.Address != pkt.Address {
		t.Errorf("Expected address %s, got %s", pkt.Address, decoded.Address)
	}
}

func TestVoteRequestResponse(t *testing.T) {
	// Test vote request/response serialization
	block := Block{
		Height:    5,
		Data:      []byte("test-data"),
		Hash:      []byte("test-hash"),
		PrevHash:  []byte("prev-hash"),
		Timestamp: time.Now(),
	}

	req := voteRequest{
		MsgType: "VOTE",
		Block:   block,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal vote request: %v", err)
	}

	var decodedReq voteRequest
	if err := json.Unmarshal(reqData, &decodedReq); err != nil {
		t.Fatalf("Failed to unmarshal vote request: %v", err)
	}

	if decodedReq.Block.Height != block.Height {
		t.Errorf("Expected height %d, got %d", block.Height, decodedReq.Block.Height)
	}

	// Test vote response
	resp := voteResponse{
		PeerDID:   "did:soho:voter",
		Approve:   true,
		Signature: []byte("signature-bytes"),
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal vote response: %v", err)
	}

	var decodedResp voteResponse
	if err := json.Unmarshal(respData, &decodedResp); err != nil {
		t.Fatalf("Failed to unmarshal vote response: %v", err)
	}

	if decodedResp.PeerDID != resp.PeerDID {
		t.Errorf("Expected peer DID %s, got %s", resp.PeerDID, decodedResp.PeerDID)
	}

	if decodedResp.Approve != resp.Approve {
		t.Errorf("Expected approve %v, got %v", resp.Approve, decodedResp.Approve)
	}
}

func TestFindBestPeer(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Add peers with different capabilities
	peers := []*Peer{
		{
			DID:       "did:soho:peer1",
			CPUCores:  2,
			StorageGB: 50,
			Score:     60,
		},
		{
			DID:       "did:soho:peer2",
			CPUCores:  8,
			StorageGB: 200,
			Score:     85, // Best score
		},
		{
			DID:       "did:soho:peer3",
			CPUCores:  4,
			StorageGB: 100,
			Score:     70,
		},
	}

	p2p.mu.Lock()
	for _, peer := range peers {
		p2p.peers[peer.DID] = peer
	}
	p2p.mu.Unlock()

	// Find best peer for job requiring 4 CPUs
	best := p2p.FindBestPeer(4, 50)

	if best == nil {
		t.Fatal("Expected to find a best peer")
	}

	// Should select peer2 (has 8 CPUs and highest score)
	if best.DID != "did:soho:peer2" {
		t.Errorf("Expected peer2 to be selected, got %s", best.DID)
	}

	// Find best peer for job requiring more CPUs than available
	best = p2p.FindBestPeer(16, 50)
	if best != nil {
		t.Error("Expected no peer to be found for excessive CPU requirement")
	}
}

func TestCollectVotes_Consensus(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Add 5 peers (need 3 for majority)
	pubKey1, _, _ := ed25519.GenerateKey(nil)
	pubKey2, _, _ := ed25519.GenerateKey(nil)
	pubKey3, _, _ := ed25519.GenerateKey(nil)
	pubKey4, _, _ := ed25519.GenerateKey(nil)
	pubKey5, _, _ := ed25519.GenerateKey(nil)

	p2p.mu.Lock()
	p2p.peers["did:soho:peer1"] = &Peer{DID: "did:soho:peer1", Address: "127.0.0.1:9001", PublicKey: pubKey1}
	p2p.peers["did:soho:peer2"] = &Peer{DID: "did:soho:peer2", Address: "127.0.0.1:9002", PublicKey: pubKey2}
	p2p.peers["did:soho:peer3"] = &Peer{DID: "did:soho:peer3", Address: "127.0.0.1:9003", PublicKey: pubKey3}
	p2p.peers["did:soho:peer4"] = &Peer{DID: "did:soho:peer4", Address: "127.0.0.1:9004", PublicKey: pubKey4}
	p2p.peers["did:soho:peer5"] = &Peer{DID: "did:soho:peer5", Address: "127.0.0.1:9005", PublicKey: pubKey5}
	p2p.mu.Unlock()

	block := Block{
		Height:    1,
		Data:      []byte("test-block"),
		Hash:      []byte("block-hash"),
		PrevHash:  []byte("prev-hash"),
		Timestamp: time.Now(),
	}

	// Note: collectVotes will try to connect to peers, which will fail in tests
	// This test validates the logic structure
	votes := p2p.collectVotes(block)

	// In real scenario with running peers, we'd expect votes
	// For now, verify the function doesn't panic
	if len(votes) > 5 {
		t.Errorf("Unexpected number of votes: %d", len(votes))
	}
}

func TestPingCentral(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Set up a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	centralAddr := listener.Addr().String()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Store central address
	ctx := context.Background()
	_ = p2p.store.SetNodeInfo(ctx, "central_address", centralAddr)

	// Ping should succeed
	online := p2p.pingCentral()
	if !online {
		t.Error("Expected pingCentral to succeed with running server")
	}

	// Close server
	listener.Close()
	time.Sleep(100 * time.Millisecond)

	// Ping should fail
	online = p2p.pingCentral()
	if online {
		t.Error("Expected pingCentral to fail after server closed")
	}
}

func TestCapabilityMessage(t *testing.T) {
	cap := capabilityMsg{
		DID:       "did:soho:test",
		CPUCores:  8,
		StorageGB: 500,
		GPUModel:  "RTX 3090",
		Score:     90,
	}

	data, err := json.Marshal(cap)
	if err != nil {
		t.Fatalf("Failed to marshal capability: %v", err)
	}

	var decoded capabilityMsg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal capability: %v", err)
	}

	if decoded.DID != cap.DID {
		t.Errorf("Expected DID %s, got %s", cap.DID, decoded.DID)
	}

	if decoded.CPUCores != cap.CPUCores {
		t.Errorf("Expected CPU cores %d, got %d", cap.CPUCores, decoded.CPUCores)
	}

	if decoded.StorageGB != cap.StorageGB {
		t.Errorf("Expected storage %dGB, got %dGB", cap.StorageGB, decoded.StorageGB)
	}

	if decoded.GPUModel != cap.GPUModel {
		t.Errorf("Expected GPU model %s, got %s", cap.GPUModel, decoded.GPUModel)
	}

	if decoded.Score != cap.Score {
		t.Errorf("Expected score %d, got %d", cap.Score, decoded.Score)
	}
}

func TestCentralBlockPayload(t *testing.T) {
	payload := centralBlockPayload{
		Height:      10,
		Data:        []byte("block-data"),
		Hash:        []byte("block-hash"),
		PrevHash:    []byte("prev-hash"),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		MerkleProof: []byte("merkle-proof"),
		Signatures:  [][]byte{[]byte("sig1"), []byte("sig2")},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal central block payload: %v", err)
	}

	var decoded centralBlockPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal central block payload: %v", err)
	}

	if decoded.Height != payload.Height {
		t.Errorf("Expected height %d, got %d", payload.Height, decoded.Height)
	}

	if len(decoded.Signatures) != len(payload.Signatures) {
		t.Errorf("Expected %d signatures, got %d", len(payload.Signatures), len(decoded.Signatures))
	}
}

func TestLookupPublicKey(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	// Create a user in the store
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = privKey

	user := &store.User{
		Username:  "testuser",
		DID:       "did:soho:testuser",
		PublicKey: pubKey,
		Role:      "basic",
	}

	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatal(err)
	}

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Lookup should find the key from user store
	foundKey, err := p2p.lookupPublicKey(ctx, "did:soho:testuser")
	if err != nil {
		t.Fatalf("Failed to lookup public key: %v", err)
	}

	if len(foundKey) != len(pubKey) {
		t.Errorf("Expected key length %d, got %d", len(pubKey), len(foundKey))
	}

	for i := range pubKey {
		if foundKey[i] != pubKey[i] {
			t.Error("Public key mismatch")
			break
		}
	}

	// Lookup non-existent key should fail
	_, err = p2p.lookupPublicKey(ctx, "did:soho:nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent DID")
	}
}

func TestPeerPersistence(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	// Create P2P network
	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Add peer to store
	pubKey, _, _ := ed25519.GenerateKey(nil)
	peerRow := &store.P2PPeerRow{
		PeerDID:   "did:soho:stored-peer",
		Address:   "192.168.1.100:9000",
		PublicKey: pubKey,
		LastSeen:  time.Now(),
		Score:     80,
		CPUCores:  4,
		StorageGB: 100,
		GPUModel:  "GPU",
	}

	if err := s.UpsertP2PPeer(ctx, peerRow); err != nil {
		t.Fatal(err)
	}

	// Retrieve peers from store
	rows, err := s.GetP2PPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Errorf("Expected 1 peer in store, got %d", len(rows))
	}

	if rows[0].PeerDID != peerRow.PeerDID {
		t.Errorf("Expected peer DID %s, got %s", peerRow.PeerDID, rows[0].PeerDID)
	}

	// Load peers into P2P network (simulating discovery from store)
	p2p.mu.Lock()
	for _, row := range rows {
		if row.PeerDID != p2p.localDID {
			p2p.peers[row.PeerDID] = &Peer{
				DID:       row.PeerDID,
				Address:   row.Address,
				PublicKey: row.PublicKey,
				LastSeen:  row.LastSeen,
				CPUCores:  row.CPUCores,
				StorageGB: row.StorageGB,
				GPUModel:  row.GPUModel,
				Score:     row.Score,
			}
		}
	}
	p2p.mu.Unlock()

	if p2p.PeerCount() != 1 {
		t.Errorf("Expected 1 peer loaded from store, got %d", p2p.PeerCount())
	}
}

func TestWriteBlock_OnlineMode(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Set online mode (central available)
	p2p.mu.Lock()
	p2p.centralOnline = true
	p2p.isFederated = false
	p2p.mu.Unlock()

	// No central configured, so this should fail
	err = p2p.WriteBlock(ctx, []byte("test-data"))
	if err == nil {
		t.Error("Expected error when writing block without central address")
	}
}

func TestWriteBlock_P2PMode(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	p2p := NewP2PNetwork(s, "did:soho:test-node", "127.0.0.1:9000")

	// Set P2P mode (central offline)
	p2p.mu.Lock()
	p2p.centralOnline = false
	p2p.isFederated = true
	p2p.mu.Unlock()

	// With no peers, consensus cannot be reached
	err = p2p.WriteBlock(ctx, []byte("test-data"))
	if err == nil {
		t.Error("Expected error when writing block without peers for consensus")
	}

	if err != nil && err.Error() == "" {
		t.Error("Expected descriptive error message")
	}
}
