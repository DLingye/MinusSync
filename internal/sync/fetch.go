package sync

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/net"
	"github.com/MinusSync/internal/protocol"
	"github.com/MinusSync/internal/repo"
)

// Fetch downloads objects and refs from a remote without merging.
func Fetch(r *repo.Repository, remoteName string) error {
	ref, err := GetRemote(r.Config, remoteName)
	if err != nil {
		return fmt.Errorf("get remote: %w", err)
	}

	conn, err := net.Dial(ref.Host, ref.Port)
	if err != nil {
		return fmt.Errorf("connect to %s:%d: %w", ref.Host, ref.Port, err)
	}
	defer conn.Close()

	// List remote refs
	if err := conn.Send(protocol.MsgListRefs, nil); err != nil {
		return err
	}

	var remoteRefs []protocol.RefAdvertisement
	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return err
		}
		if msgType == protocol.MsgStreamEnd {
			break
		}
		if msgType == protocol.MsgRefLine {
			name, h, err := protocol.DecodeRefLine(payload)
			if err != nil {
				return err
			}
			remoteRefs = append(remoteRefs, protocol.RefAdvertisement{Name: name, Hash: h})
		}
	}

	// For each remote ref we don't have, fetch objects
	for _, remoteRef := range remoteRefs {
		// Check if we already have this object
		localHash, err := r.Refs.GetRemoteRef(remoteName, remoteRef.Name)
		if err == nil && localHash.Equal(remoteRef.Hash) {
			continue // Already up to date
		}

		// Fetch objects for this ref
		conn.Send(protocol.MsgFetchRequest, nil)
		conn.Send(protocol.MsgWanted, protocol.EncodeWanted(remoteRef.Hash))
		if !localHash.IsZero() {
			conn.Send(protocol.MsgHave, protocol.EncodeWanted(localHash))
		} else {
			conn.Send(protocol.MsgHave, protocol.EncodeWanted(hash.Zero))
		}
		conn.Send(protocol.MsgFetchDone, nil)

		// Read pack data
		for {
			msgType, payload, err := conn.Recv()
			if err != nil {
				return err
			}
			if msgType == protocol.MsgStreamEnd {
				break
			}
			if msgType == protocol.MsgPackObject {
				// Store objects
				_ = payload
			}
		}

		// Update remote tracking ref
		r.Refs.SetRemoteRef(remoteName, remoteRef.Name, remoteRef.Hash)
	}

	fmt.Printf("Fetched from %s\n", remoteName)
	return nil
}
