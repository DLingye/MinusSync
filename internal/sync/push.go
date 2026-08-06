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
func Push(r *repo.Repository, remoteName, branch string) error {
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

	// Get local hash
	localHash, err := r.Refs.GetBranch(branch)
	if err != nil {
		return fmt.Errorf("local branch %q: %w", branch, err)
	}

	// Get remote tracking hash
	oldHash := hash.Zero
	remoteHash, err := r.Refs.GetRemoteRef(remoteName, "refs/heads/"+branch)
	if err == nil {
		oldHash = remoteHash
	}

	// Send push request
	conn.Send(protocol.MsgPushRequest, nil)
	refName := "refs/heads/" + branch
	conn.Send(protocol.MsgUpdateRef, protocol.EncodeUpdateRef(refName, oldHash, localHash))

	// Wait for server response
	msgType, payload, err := conn.Recv()
	if err != nil {
		return fmt.Errorf("receive push response: %w", err)
	}

	switch msgType {
	case protocol.MsgWantPack:
		// Server wants us to send pack
		if err := sendPack(conn, r.ObjectsPath(), oldHash, localHash); err != nil {
			return fmt.Errorf("send pack: %w", err)
		}

		// Wait for final acknowledgment
		msgType, payload, err = conn.Recv()
		if err != nil {
			return fmt.Errorf("receive ack: %w", err)
		}
		if msgType == protocol.MsgOK {
			// Update remote tracking ref
			r.Refs.SetRemoteRef(remoteName, refName, localHash)
			fmt.Printf("Pushed %s to %s/%s\n", localHash.String(), remoteName, branch)
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
	// Compute the set of objects to send
	objSet, err := computeObjectDiff(objectsDir, oldHash, newHash)
	if err != nil {
		return err
	}

	// Send pack header
	countBytes := make([]byte, 4)
	object.WriteUint32(countBytes, uint32(len(objSet)))
	conn.Send(protocol.MsgPackHeader, countBytes)

	// Send each object
	for _, h := range objSet {
		data, err := object.ReadRaw(objectsDir, h)
		if err != nil {
			continue
		}
		payload := protocol.EncodePackObject(h, 0, data)
		conn.Send(protocol.MsgPackObject, payload)
	}

	// End of pack
	conn.Send(protocol.MsgStreamEnd, nil)
	return nil
}

// computeObjectDiff finds all objects reachable from newHash but not oldHash.
func computeObjectDiff(objectsDir string, oldHash, newHash hash.Hash) ([]hash.Hash, error) {
	// Mark reachable from oldHash
	oldReachable := make(map[string]bool)
	if !oldHash.IsZero() {
		markReachableForPack(objectsDir, oldHash, oldReachable)
	}

	// Collect reachable from newHash that aren't in oldReachable
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
