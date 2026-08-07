package sync

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/net"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/protocol"
	"github.com/MinusSync/internal/repo"
)

// Push sends local commits to a remote repository.
func Push(r *repo.Repository, remoteName string) error {
	ref, err := GetRemote(r.Config, remoteName)
	if err != nil {
		return fmt.Errorf("get remote: %w", err)
	}

	// Connect
	conn, err := net.Dial(ref.Host, ref.Port)
	if err != nil {
		return fmt.Errorf("connect to %s:%d: %w", ref.Host, ref.Port, err)
	}
	defer conn.Close()

	// Get local HEAD and sequence
	localHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	localCommit, err := object.ReadCommit(r.ObjectsPath(), localHash)
	if err != nil {
		return fmt.Errorf("read local commit: %w", err)
	}
	localSeq := localCommit.Sequence

	// Get remote tracking hash and sequence
	oldHash := hash.Zero
	var remoteSeq uint64
	remoteHash, err := r.Refs.GetRemoteRef(remoteName, "HEAD")
	if err == nil {
		oldHash = remoteHash
		remoteCommit, err := object.ReadCommit(r.ObjectsPath(), remoteHash)
		if err == nil {
			remoteSeq = remoteCommit.Sequence
		}
	}

	// Sequence-based conflict detection:
	// If remote has higher sequence, remote is ahead of us
	// If we have higher sequence but different hash, we are ahead (force push needed or remote diverged)
	if oldHash != hash.Zero && !oldHash.Equal(localHash) && remoteSeq > localSeq {
		return fmt.Errorf("push rejected: remote has newer commits (remote seq=%d, local seq=%d). Use 'msync pull' first", remoteSeq, localSeq)
	}

	// Send push request
	conn.Send(protocol.MsgPushRequest, nil)
	conn.Send(protocol.MsgUpdateRef, protocol.EncodeUpdateRef("HEAD", oldHash, localHash))

	// Wait for server response
	msgType, payload, err := conn.Recv()
	if err != nil {
		return fmt.Errorf("receive push response: %w", err)
	}

	switch msgType {
	case protocol.MsgWantPack:
		if err := sendPack(conn, r.ObjectsPath(), oldHash, localHash); err != nil {
			return fmt.Errorf("send pack: %w", err)
		}

		msgType, payload, err = conn.Recv()
		if err != nil {
			return fmt.Errorf("receive ack: %w", err)
		}
		if msgType == protocol.MsgOK {
			r.Refs.SetRemoteRef(remoteName, "HEAD", localHash)
			fmt.Printf("Pushed %s to %s\n", localHash.String(), remoteName)
			return nil
		}
		if msgType == protocol.MsgError {
			code, msg, _ := protocol.DecodeError(payload)
			return fmt.Errorf("push rejected: %s (code=%d)", msg, code)
		}
		return fmt.Errorf("unexpected response: %s", protocol.MessageName(msgType))

	case protocol.MsgError:
		code, msg, _ := protocol.DecodeError(payload)
		return fmt.Errorf("push rejected: %s (code=%d)", msg, code)

	case protocol.MsgOK:
		fmt.Println("Everything up-to-date")
		return nil
	}

	return nil
}

// sendPack sends all objects between oldHash and newHash.
func sendPack(conn *net.Connection, objectsDir string, oldHash, newHash hash.Hash) error {
	objSet, err := computeObjectDiff(objectsDir, oldHash, newHash)
	if err != nil {
		return err
	}

	countBytes := make([]byte, 4)
	object.WriteUint32(countBytes, uint32(len(objSet)))
	conn.Send(protocol.MsgPackHeader, countBytes)

	for _, h := range objSet {
		data, err := object.ReadRaw(objectsDir, h)
		if err != nil {
			continue
		}
		payload := protocol.EncodePackObject(h, 0, data)
		conn.Send(protocol.MsgPackObject, payload)
	}

	conn.Send(protocol.MsgStreamEnd, nil)
	return nil
}

// computeObjectDiff finds all objects reachable from newHash but not oldHash.
func computeObjectDiff(objectsDir string, oldHash, newHash hash.Hash) ([]hash.Hash, error) {
	oldReachable := make(map[string]bool)
	if !oldHash.IsZero() {
		markReachableForPack(objectsDir, oldHash, oldReachable)
	}

	var diff []hash.Hash
	newReachable := make(map[string]bool)
	collectNewObjects(objectsDir, newHash, oldReachable, newReachable, &diff)

	return diff, nil
}

func markReachableForPack(objectsDir string, h hash.Hash, reachable map[string]bool) {
	hex := h.Hex()
	if reachable[hex] {
		return
	}
	reachable[hex] = true

	header, err := object.ReadHeader(objectsDir, h)
	if err != nil {
		return
	}

	switch header.Type {
	case object.TypeCommit:
		commit, err := object.ReadCommit(objectsDir, h)
		if err != nil {
			return
		}
		markReachableForPack(objectsDir, commit.Tree, reachable)
		for _, p := range commit.Parents {
			markReachableForPack(objectsDir, p, reachable)
		}
	case object.TypeTree:
		entries, err := object.ReadTree(objectsDir, h)
		if err != nil {
			return
		}
		for _, e := range entries {
			markReachableForPack(objectsDir, e.Hash, reachable)
		}
	case object.TypeTag:
		tag, err := object.ReadTag(objectsDir, h)
		if err != nil {
			return
		}
		markReachableForPack(objectsDir, tag.Object, reachable)
	}
}

func collectNewObjects(objectsDir string, h hash.Hash, oldReachable, newReachable map[string]bool, diff *[]hash.Hash) {
	hex := h.Hex()
	if newReachable[hex] {
		return
	}
	newReachable[hex] = true

	if !oldReachable[hex] {
		*diff = append(*diff, h)
	}

	header, err := object.ReadHeader(objectsDir, h)
	if err != nil {
		return
	}

	switch header.Type {
	case object.TypeCommit:
		commit, err := object.ReadCommit(objectsDir, h)
		if err != nil {
			return
		}
		collectNewObjects(objectsDir, commit.Tree, oldReachable, newReachable, diff)
		for _, p := range commit.Parents {
			collectNewObjects(objectsDir, p, oldReachable, newReachable, diff)
		}
	case object.TypeTree:
		entries, err := object.ReadTree(objectsDir, h)
		if err != nil {
			return
		}
		for _, e := range entries {
			collectNewObjects(objectsDir, e.Hash, oldReachable, newReachable, diff)
		}
	case object.TypeTag:
		tag, err := object.ReadTag(objectsDir, h)
		if err != nil {
			return
		}
		collectNewObjects(objectsDir, tag.Object, oldReachable, newReachable, diff)
	}
}
